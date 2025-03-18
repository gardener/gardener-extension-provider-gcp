// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	gcpinternal "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

// Delete implements infrastructure.Actuator.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return util.DetermineError(a.delete(ctx, log, infra, cluster), helper.KnownCodes)
}

// ForceDelete forcefully deletes the Infrastructure.
func (a *actuator) ForceDelete(_ context.Context, _ logr.Logger, _ *extensionsv1alpha1.Infrastructure, _ *controller.Cluster) error {
	return nil
}

func (a *actuator) delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	infraState, err := a.infrastructureStateFromRaw(infra)
	if err != nil {
		return err
	}

	credentialsConfig, err := gcpinternal.GetCredentialsConfigFromSecretReference(ctx, a.client, infra.Spec.SecretRef)
	if err != nil {
		return err
	}
	gc := gcpclient.New()
	fctx, err := infraflow.NewFlowContext(ctx, infraflow.Opts{
		Log:               log,
		Infra:             infra,
		Cluster:           cluster,
		CredentialsConfig: credentialsConfig,
		Factory:           gc,
		Client:            a.client,
		State:             infraState,
	})
	if err != nil {
		return fmt.Errorf("failed to create flow context: %v", err)
	}

	err = fctx.Delete(ctx)
	if err != nil {
		return err
	}

	tf, err := NewTerraformer(log, a.restConfig, "infra", infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return CleanupTerraformerResources(ctx, tf)
}
