// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

// Restore implements infrastructure.Actuator.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	useFlow, err := shouldUseFlow(infra, cluster)
	if err != nil {
		return err
	}

	var stateConfigMapInitializer terraformer.StateConfigMapInitializer
	if !useFlow {
		terraformState, err := terraformer.UnmarshalRawState(infra.Status.State)
		if err != nil {
			return err
		}
		stateConfigMapInitializer = terraformer.CreateOrUpdateState{State: &terraformState.Data}
	}

	err = a.reconcile(ctx, log, infra, cluster, stateConfigMapInitializer)
	return util.DetermineError(err, helper.KnownCodes)
}
