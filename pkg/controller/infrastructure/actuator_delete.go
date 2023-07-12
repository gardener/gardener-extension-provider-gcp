// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
