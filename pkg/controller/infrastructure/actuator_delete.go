// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
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
	withFlow, err := shouldDeleteWithFlow(infra)
	if err != nil {
		return err
	}

	if !withFlow {
		return NewTerraformReconciler(a.client, a.restConfig, nil, a.disableProjectedTokenMount).Delete(ctx, log, cluster, infra)
	}

	flow, err := infraflow.NewFlowReconciler(ctx, log, infra, cluster, a.client)
	if err != nil {
		return err
	}
	return flow.Delete(ctx)
}

func shouldDeleteWithFlow(infra *extensionsv1alpha1.Infrastructure) (bool, error) {
	state, err := getFlowStateFromInfrastructureStatus(infra)
	if err != nil {
		return false, err
	}

	return state != nil, nil
}
