// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	gcpinternal "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

// Reconcile implements infrastructure.Actuator.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return util.DetermineError(a.reconcile(ctx, log, infra, cluster), helper.KnownCodes)
}

func (a *actuator) reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	infraState, err := a.infrastructureStateFromRaw(infra)
	if err != nil {
		return err
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

	if raw != nil && (len(raw.Raw) > 0 || raw.Object != nil) {
		jsonBytes, err := raw.MarshalJSON()
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(jsonBytes, state); err != nil {
			return nil, err
		}
		// Older native states used the kind FlowState. Accept both native kinds, but reject
		// legacy Terraform state and other objects instead of silently treating them as empty state.
		if state.APIVersion != v1alpha1.SchemeGroupVersion.String() || (state.Kind != "InfrastructureState" && state.Kind != "FlowState") {
			return nil, fmt.Errorf("unsupported infrastructure state kind %q with apiVersion %q", state.Kind, state.APIVersion)
		}
	}

	return state, nil
}
