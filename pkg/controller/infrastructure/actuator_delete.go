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
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

func (a *actuator) cleanupKubernetesFirewallRules(
	ctx context.Context,
	config *api.InfrastructureConfig,
	client gcpclient.Interface,
	tf terraformer.Terraformer,
	account *gcp.ServiceAccount,
	shootSeedNamespace string,
) error {
	state, err := infrastructure.ExtractTerraformState(ctx, tf, config, false)
	if err != nil {
		if terraformer.IsVariablesNotFoundError(err) {
			return nil
		}
		return err
	}

	return infrastructure.CleanupKubernetesFirewalls(ctx, client, account.ProjectID, state.VPCName, shootSeedNamespace)
}

func (a *actuator) cleanupKubernetesRoutes(
	ctx context.Context,
	config *api.InfrastructureConfig,
	client gcpclient.Interface,
	tf terraformer.Terraformer,
	account *gcp.ServiceAccount,
	shootSeedNamespace string,
) error {
	state, err := infrastructure.ExtractTerraformState(ctx, tf, config, false)
	if err != nil {
		if terraformer.IsVariablesNotFoundError(err) {
			return nil
		}
		return err
	}

	return infrastructure.CleanupKubernetesRoutes(ctx, client, account.ProjectID, state.VPCName, shootSeedNamespace)
}

// Delete implements infrastructure.Actuator.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	tf, err := internal.NewTerraformer(log, a.RESTConfig(), infrastructure.TerraformerPurpose, infra, a.disableProjectedTokenMount)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// terraform pod from previous reconciliation might still be running, ensure they are gone before doing any operations
	if err := tf.EnsureCleanedUp(ctx); err != nil {
		return err
	}

	// If the Terraform state is empty then we can exit early as we didn't create anything. Though, we clean up potentially
	// created configmaps/secrets related to the Terraformer.
	stateIsEmpty := tf.IsStateEmpty(ctx)
	if stateIsEmpty {
		log.Info("exiting early as infrastructure state is empty - nothing to do")
		return tf.CleanupConfiguration(ctx)
	}

	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	serviceAccount, err := gcp.GetServiceAccountFromSecretReference(ctx, a.Client(), infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	tf, err = internal.SetTerraformerEnvVars(tf, infra.Spec.SecretRef)
	if err != nil {
		return err
	}

	gcpClient, err := gcpclient.NewFromServiceAccount(ctx, serviceAccount.Raw)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	configExists, err := tf.ConfigExists(ctx)
	if err != nil {
		return err
	}

	var (
		g                              = flow.NewGraph("GCP infrastructure destruction")
		destroyKubernetesFirewallRules = g.Add(flow.Task{
			Name: "Destroying Kubernetes firewall rules",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return a.cleanupKubernetesFirewallRules(ctx, config, gcpClient, tf, serviceAccount, infra.Namespace)
			}).
				RetryUntilTimeout(10*time.Second, 5*time.Minute).
				DoIf(configExists),
		})

		destroyKubernetesRoutes = g.Add(flow.Task{
			Name: "Destroying Kubernetes route entries",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return a.cleanupKubernetesRoutes(ctx, config, gcpClient, tf, serviceAccount, infra.Namespace)
			}).
				RetryUntilTimeout(10*time.Second, 5*time.Minute).
				DoIf(configExists),
		})

		_ = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           tf.Destroy,
			Dependencies: flow.NewTaskIDs(destroyKubernetesFirewallRules, destroyKubernetesRoutes),
		})

		f = g.Compile()
	)

	if err := f.Run(ctx, flow.Opts{}); err != nil {
		return util.DetermineError(flow.Errors(err), helper.KnownCodes)
	}
	return nil
}
