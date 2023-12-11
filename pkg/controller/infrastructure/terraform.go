//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package infrastructure

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/terraformer"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	k8sClient "sigs.k8s.io/controller-runtime/pkg/client"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

// TerraformReconciler can manage infrastructure resources using Terraformer.
type TerraformReconciler struct {
	client                     k8sClient.Client
	restConfig                 *rest.Config
	stateInitializer           terraformer.StateConfigMapInitializer
	disableProjectedTokenMount bool
}

// NewTerraformReconciler returns a new instance of TerraformReconciler.
func NewTerraformReconciler(client k8sClient.Client, restConfig *rest.Config, tfInitializer terraformer.StateConfigMapInitializer, disableProjectedTokenMount bool) *TerraformReconciler {
	return &TerraformReconciler{
		client:                     client,
		restConfig:                 restConfig,
		stateInitializer:           tfInitializer,
		disableProjectedTokenMount: disableProjectedTokenMount,
	}
}

// Reconcile reconciles infrastructure using Terraformer.
func (t *TerraformReconciler) Reconcile(ctx context.Context, log logr.Logger, cluster *controller.Cluster, infra *extensionsv1alpha1.Infrastructure) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	log.Info("reconcile infrastructure using terraform reconciler")
	tf, err := internal.NewTerraformer(log, t.restConfig, infrastructure.TerraformerPurpose, infra, t.disableProjectedTokenMount)
	if err != nil {
		return nil, nil, err
	}

	serviceAccount, err := infrastructure.GetServiceAccountFromInfrastructure(ctx, t.client, infra)
	if err != nil {
		return nil, nil, err
	}

	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return nil, nil, err
	}

	createSA, err := shouldCreateServiceAccount(infra)
	if err != nil {
		return nil, nil, err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, serviceAccount, config, cluster.Shoot.Spec.Networking.Pods, createSA)
	if err != nil {
		return nil, nil, err
	}

	tf, err = internal.SetTerraformerEnvVars(tf, infra.Spec.SecretRef)
	if err != nil {
		return nil, nil, err
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(t.client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, t.stateInitializer)).
		Apply(ctx); err != nil {
		return nil, nil, fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	return t.reconcileStatus(ctx, tf, config, createSA)
}

// Delete deletes the infrastructure using Terraformer.
func (t *TerraformReconciler) Delete(ctx context.Context, log logr.Logger, _ *extensions.Cluster, infra *extensionsv1alpha1.Infrastructure) error {
	tf, err := internal.NewTerraformer(log, t.restConfig, infrastructure.TerraformerPurpose, infra, t.disableProjectedTokenMount)
	if err != nil {
		return err
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

	serviceAccount, err := gcp.GetServiceAccountFromSecretReference(ctx, t.client, infra.Spec.SecretRef)
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
				return cleanupKubernetesFirewallRules(ctx, config, gcpClient, tf, serviceAccount, infra.Namespace)
			}).RetryUntilTimeout(10*time.Second, 5*time.Minute),
			SkipIf: !configExists,
		})

		destroyKubernetesRoutes = g.Add(flow.Task{
			Name: "Destroying Kubernetes route entries",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return cleanupKubernetesRoutes(ctx, config, gcpClient, tf, serviceAccount, infra.Namespace)
			}).RetryUntilTimeout(10*time.Second, 5*time.Minute),
			SkipIf: !configExists,
		})

		_ = g.Add(flow.Task{
			Name:         "Destroying Shoot infrastructure",
			Fn:           tf.Destroy,
			Dependencies: flow.NewTaskIDs(destroyKubernetesFirewallRules, destroyKubernetesRoutes),
		})

		f = g.Compile()
	)

	err = f.Run(ctx, flow.Opts{})

	return err
}

// shouldCreateServiceAccount checks whether terraform needs to create/reconcile a gardener-managed service account.
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

func cleanupKubernetesFirewallRules(
	ctx context.Context,
	config *api.InfrastructureConfig,
	gcpClient gcpclient.Interface,
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

	return infrastructure.CleanupKubernetesFirewalls(ctx, gcpClient, account.ProjectID, state.VPCName, shootSeedNamespace)
}

func cleanupKubernetesRoutes(
	ctx context.Context,
	config *api.InfrastructureConfig,
	gcpClient gcpclient.Interface,
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

	return infrastructure.CleanupKubernetesRoutes(ctx, gcpClient, account.ProjectID, state.VPCName, shootSeedNamespace)
}

func (t *TerraformReconciler) reconcileStatus(
	ctx context.Context,
	tf terraformer.Terraformer,
	config *api.InfrastructureConfig,
	createSA bool,
) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	status, err := infrastructure.ComputeStatus(ctx, tf, config, createSA)
	if err != nil {
		return nil, nil, err
	}

	state, err := tf.GetRawState(ctx)
	if err != nil {
		return nil, nil, err
	}

	stateByte, err := state.Marshal()
	if err != nil {
		return nil, nil, err
	}

	return status, &runtime.RawExtension{Raw: stateByte}, nil
}
