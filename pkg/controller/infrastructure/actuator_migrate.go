// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
)

// Migrate implements infrastructure.Actuator.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	// todo: remove this in a future release. For now to support migration from terraform a little longer, we will
	//  reconcile (and thus migrate to flow), before the CP migration. That way we only have one scenario to support for the migration.
	if err := a.reconcile(ctx, log, infra, cluster); err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	tf, err := NewTerraformer(log, a.restConfig, "infra", infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}
	return util.DetermineError(CleanupTerraformerResources(ctx, tf), helper.KnownCodes)
}
