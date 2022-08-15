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
	"net/http"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
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

	iam, err := gcpclient.NewIAMClient(ctx, serviceAccount)
	if err != nil {
		return err
	}
	createSA, err := shouldCreateServiceAccount(iam, cluster.ObjectMeta.Name)
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
// If we do have ServiceAccount permissions and there is already a service acccount with the shoot name, continue
// to reconcile it in the terraform.
func shouldCreateServiceAccount(iam gcpclient.IAMClient, clusterName string) (bool, error) {
	if !features.ExtensionFeatureGate.Enabled(features.DisableGardenerServiceAccountCreation) {
		return true, nil
	}

	if _, err := iam.GetServiceAccount(context.Background(), clusterName); err != nil {
		if gcpclient.IsErrorCode(err, http.StatusNotFound, http.StatusUnauthorized) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
