// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	gcpinternal "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	infrainternal "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

// FlowReconciler an implementation of an infrastructure reconciler using native SDKs.
type FlowReconciler struct {
	client                     client.Client
	restConfig                 *rest.Config
	log                        logr.Logger
	disableProjectedTokenMount bool
}

// NewFlowReconciler creates a new flow reconciler.
func NewFlowReconciler(client client.Client, restConfig *rest.Config, log logr.Logger, projToken bool) (Reconciler, error) {
	return &FlowReconciler{
		client:                     client,
		restConfig:                 restConfig,
		log:                        log,
		disableProjectedTokenMount: projToken,
	}, nil
}

// Reconcile reconciles the infrastructure and returns the status (state of the world), the state (input for the next loops) and any errors that occurred.
func (f *FlowReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var (
		infraState *gcp.InfrastructureState
		err        error
	)

	// when the function is called, we may have: a. no state, b. terraform state (migration) or c. flow state. In case of a TF state
	// because no explicit migration to the new flow format is necessary, we simply return an empty state.
	fsOk, err := hasFlowState(infra.Status.State)
	if err != nil {
		return err
	}

	if fsOk {
		// if it had a flow state, then we just decode it.
		infraState, err = f.infrastructureStateFromRaw(infra)
		if err != nil {
			return err
		}
	} else {
		// otherwise migrate it from the terraform state if needed.
		infraState, err = f.migrateFromTerraform(ctx, infra)
		if err != nil {
			return err
		}
	}

	credentialsConfig, err := gcpinternal.GetCredentialsConfigFromSecretReference(ctx, f.client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	fctx, err := infraflow.NewFlowContext(ctx, infraflow.Opts{
		Log:               f.log,
		Infra:             infra,
		State:             infraState,
		Cluster:           cluster,
		CredentialsConfig: credentialsConfig,
		Factory:           gcpclient.New(),
		Client:            f.client,
	})
	if err != nil {
		return fmt.Errorf("failed to create flow context: %v", err)
	}

	return fctx.Reconcile(ctx)
}

// Delete deletes the infrastructure resource using the flow reconciler.
func (f *FlowReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	infraState, err := f.infrastructureStateFromRaw(infra)
	if err != nil {
		return err
	}

	credentialsConfig, err := gcpinternal.GetCredentialsConfigFromSecretReference(ctx, f.client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}
	gc := gcpclient.New()
	fctx, err := infraflow.NewFlowContext(ctx, infraflow.Opts{
		Log:               f.log,
		Infra:             infra,
		Cluster:           cluster,
		CredentialsConfig: credentialsConfig,
		Factory:           gc,
		Client:            f.client,
		State:             infraState,
	})
	if err != nil {
		return fmt.Errorf("failed to create flow context: %v", err)
	}

	err = fctx.Delete(ctx)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformer(f.log, f.restConfig, infrainternal.TerraformerPurpose, infra, f.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return CleanupTerraformerResources(ctx, tf)
}

// Restore implements the restoration of an infrastructure resource during the control plane migration.
func (f *FlowReconciler) Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return f.Reconcile(ctx, infra, cluster)
}

func (f *FlowReconciler) infrastructureStateFromRaw(infra *extensionsv1alpha1.Infrastructure) (*gcp.InfrastructureState, error) {
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

func (f *FlowReconciler) migrateFromTerraform(ctx context.Context, infra *extensionsv1alpha1.Infrastructure) (*gcp.InfrastructureState, error) {
	state := &gcp.InfrastructureState{
		Data: map[string]string{},
	}
	// we want to prevent allowing the deletion of infrastructure if there may be still resources in the cloudprovider. We will initialize the data
	// with a specific "marker" so that the deletion
	tf, err := internal.NewTerraformer(f.log, f.restConfig, infrainternal.TerraformerPurpose, infra, f.disableProjectedTokenMount)
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
