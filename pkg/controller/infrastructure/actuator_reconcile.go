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
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

// Reconcile implements infrastructure.Actuator.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	return a.reconcile(ctx, log, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
}

func (a *actuator) reconcile(ctx context.Context, logger logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster, stateInitializer terraformer.StateConfigMapInitializer) error {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	serviceAccount, err := infrastructure.GetServiceAccountFromInfrastructure(ctx, a.Client(), infra)
	if err != nil {
		return err
	}

	createSA, err := shouldCreateServiceAccount(infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, serviceAccount, config, cluster.Shoot.Spec.Networking.Pods, createSA)
	if err != nil {
		return err
	}

	tf, err := internal.NewTerraformerWithAuth(logger, a.RESTConfig(), infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return err
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(a.Client(), terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, stateInitializer)).
		Apply(ctx); err != nil {
		return fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	return a.updateProviderStatus(ctx, tf, infra, config, createSA)
}

// shouldCreateServiceAccount checkes whether terraform needs to create/reconcile a gardener-managed service account.
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
