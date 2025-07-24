// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
	gcpinternal "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

// Reconcile implements infrastructure.Actuator.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return util.DetermineError(a.reconcile(ctx, log, infra, cluster), helper.KnownCodes)
}

func (a *actuator) reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var (
		infraState *gcp.InfrastructureState
		err        error
	)

	// when the function is called, we may have: a. no state, b. terraform state (migration) or c. flow state.
	fsOk, err := hasFlowState(infra.Status.State)
	if err != nil {
		return err
	}

	if fsOk {
		// if it had a flow state, then we just decode it.
		infraState, err = a.infrastructureStateFromRaw(infra)
		if err != nil {
			return err
		}
	} else {
		// otherwise migrate it from the terraform state if needed.
		// todo(kon-angelo): remove in future release when the terraform library is deprecated.
		infraState, err = a.migrateFromTerraform(ctx, log, infra)
		if err != nil {
			return err
		}
	}

	credentialsConfig, err := gcpinternal.GetCredentialsConfigFromSecretReference(ctx, a.client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(ctx, infraflow.Opts{
		Log:               log,
		Infra:             infra,
		State:             infraState,
		Cluster:           cluster,
		CredentialsConfig: credentialsConfig,
		Factory:           gcpclient.New(),
		Client:            a.client,
	})
	if err != nil {
		return fmt.Errorf("failed to create flow context: %v", err)
	}

	return fctx.Reconcile(ctx)
}

func (a *actuator) infrastructureStateFromRaw(infra *extensionsv1alpha1.Infrastructure) (*gcp.InfrastructureState, error) {
	state := &gcp.InfrastructureState{}
	raw := infra.Status.State

	if raw != nil {
		jsonBytes, err := raw.MarshalJSON()
		if err != nil {
			return nil, err
		}

		// todo(ka): for now we won't use the actuator decoder because the flow state kind was registered as "FlowState" and not "InfrastructureState". So we
		// shall use the simple json unmarshal for this release.
		if err := json.Unmarshal(jsonBytes, state); err != nil {
			return nil, err
		}
	}

	return state, nil
}

func (a *actuator) migrateFromTerraform(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure) (*gcp.InfrastructureState, error) {
	state := &gcp.InfrastructureState{
		Data: map[string]string{},
	}

	// This is a special case when migrating from Terraform. If TF had created any resources (meaning there is an actual tf.state written)
	// we will mark that there are infra resources created. This is to prevent the edge case where we migrate to flow with
	// an expired set of credentials, in which case we cannot verify if there are actual provider resources.
	tf, err := NewTerraformer(log, a.restConfig, "infra", infra, a.disableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	// nothing to do if state is empty
	if tf.IsStateEmpty(ctx) {
		return state, nil
	}

	// this is a special case when migrating from Terraform. If TF had created any resources (meaning there is an actual tf.state written)
	// we mark that there are infra resources created.
	state.Data[infraflow.CreatedResourcesExistKey] = "true"

	// In addition, we will make sure that if we have created a service account we will keep track of it by adding a special marker.
	ok, err := shouldCreateServiceAccount(infra)
	if err != nil {
		return nil, err
	}
	if ok {
		state.Data[infraflow.CreatedServiceAccountKey] = "true"
	}
	return state, nil
}

func hasFlowState(state *runtime.RawExtension) (bool, error) {
	if state == nil {
		return false, nil
	}

	flowState := runtime.TypeMeta{}
	stateJson, err := state.MarshalJSON()
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(stateJson, &flowState); err != nil {
		return false, err
	}

	if flowState.GroupVersionKind().GroupVersion() == v1alpha1.SchemeGroupVersion {
		return true, nil
	}

	return false, nil
}

// shouldCreateServiceAccount checks whether terraform needs to create/reconcile a gardener-managed service account.
// For existing infrastructure the existence of a created service account is retrieved from the terraformer state.
func shouldCreateServiceAccount(infra *extensionsv1alpha1.Infrastructure) (bool, error) {
	var newCluster, hasServiceAccount bool
	rawState, err := getTerraformerRawState(infra.Status.State)
	if err != nil {
		return false, err
	}

	if rawState == nil {
		newCluster = true
	} else {
		state, err := shared.UnmarshalTerraformStateFromTerraformer(rawState)
		if err != nil {
			return false, err
		}
		hasServiceAccount = len(state.FindManagedResourcesByType("google_service_account")) > 0
	}

	if newCluster && !features.ExtensionFeatureGate.Enabled(features.DisableGardenerServiceAccountCreation) {
		return true, nil
	}
	return hasServiceAccount, nil
}

func getTerraformerRawState(state *runtime.RawExtension) (*terraformer.RawState, error) {
	if state == nil {
		return nil, nil
	}
	tfRawState, err := terraformer.UnmarshalRawState(state)
	if err != nil {
		return nil, fmt.Errorf("could not decode terraform raw state: %+v", err)
	}
	return tfRawState, nil
}
