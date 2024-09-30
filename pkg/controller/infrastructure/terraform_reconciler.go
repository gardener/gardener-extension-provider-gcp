// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//

package infrastructure

import (
	"context"
	"fmt"
	"net/http"
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
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

// TerraformReconciler can manage infrastructure resources using Terraformer.
type TerraformReconciler struct {
	client                     k8sClient.Client
	restConfig                 *rest.Config
	log                        logr.Logger
	disableProjectedTokenMount bool

	tokenMetadataClient  *http.Client
	tokenMetadataBaseURL string
}

// NewTerraformReconciler returns a new instance of TerraformReconciler.
func NewTerraformReconciler(client k8sClient.Client, restConfig *rest.Config, log logr.Logger, disableProjectedTokenMount bool, tokenMetadataBaseURL string, tokenMetadataClient *http.Client) *TerraformReconciler {
	return &TerraformReconciler{
		client:                     client,
		restConfig:                 restConfig,
		log:                        log,
		disableProjectedTokenMount: disableProjectedTokenMount,
		tokenMetadataBaseURL:       tokenMetadataBaseURL,
		tokenMetadataClient:        tokenMetadataClient,
	}
}

// Restore restores the infrastructure after a control plane migration. Effectively it performs a recovery of data from the infrastructure.status.state and
// proceeds to reconcile.
func (t *TerraformReconciler) Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	var initializer terraformer.StateConfigMapInitializer

	terraformState, err := terraformer.UnmarshalRawState(infra.Status.State)
	if err != nil {
		return err
	}

	initializer = terraformer.CreateOrUpdateState{State: &terraformState.Data}
	return t.reconcile(ctx, infra, cluster, initializer)
}

// Reconcile reconciles infrastructure using Terraformer.
func (t *TerraformReconciler) Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error {
	err := t.reconcile(ctx, infra, cluster, terraformer.StateConfigMapInitializerFunc(terraformer.CreateState))
	return util.DetermineError(err, helper.KnownCodes)
}

func (t *TerraformReconciler) reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster, initializer terraformer.StateConfigMapInitializer) error {
	log := t.log

	log.Info("reconcile infrastructure using terraform reconciler")
	credentialsConfig, err := infrastructure.GetCredentialsConfigFromInfrastructure(ctx, t.client, infra)
	if err != nil {
		return err
	}

	useWorkloadIdentityToken := credentialsConfig.Type == gcp.ExternalAccountCredentialType && len(credentialsConfig.TokenFilePath) > 0
	tf, err := internal.NewTerraformerWithAuth(log, t.restConfig, infrastructure.TerraformerPurpose, infra, t.disableProjectedTokenMount, useWorkloadIdentityToken)
	if err != nil {
		return err
	}

	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return err
	}

	createSA, err := shouldCreateServiceAccount(infra)
	if err != nil {
		return err
	}

	terraformFiles, err := infrastructure.RenderTerraformerTemplate(infra, credentialsConfig, config, cluster.Shoot.Spec.Networking.Pods, createSA)
	if err != nil {
		return err
	}

	if err := tf.
		InitializeWith(ctx, terraformer.DefaultInitializer(t.client, terraformFiles.Main, terraformFiles.Variables, terraformFiles.TFVars, initializer)).
		Apply(ctx); err != nil {
		return fmt.Errorf("failed to apply the terraform config: %w", err)
	}

	status, state, err := t.computeTerraformStatus(ctx, tf, config, createSA)
	if err != nil {
		return err
	}

	return patchProviderStatusAndState(ctx, t.client, infra, status, state)
}

// Delete deletes the infrastructure using Terraformer.
func (t *TerraformReconciler) Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, c *extensions.Cluster) error {
	return util.DetermineError(t.delete(ctx, infra, c), helper.KnownCodes)
}

func (t *TerraformReconciler) delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, _ *extensions.Cluster) error {
	log := t.log

	credentialsConfig, err := infrastructure.GetCredentialsConfigFromInfrastructure(ctx, t.client, infra)
	if err != nil {
		return err
	}

	useWorkloadIdentityToken := credentialsConfig.Type == gcp.ExternalAccountCredentialType && len(credentialsConfig.TokenFilePath) > 0
	tf, err := internal.NewTerraformerWithAuth(log, t.restConfig, infrastructure.TerraformerPurpose, infra, t.disableProjectedTokenMount, useWorkloadIdentityToken)
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

	gcpClient, err := gcpclient.New(t.tokenMetadataBaseURL, t.tokenMetadataClient).Compute(ctx, t.client, infra.Spec.SecretRef)
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
				return cleanupKubernetesFirewallRules(ctx, config, gcpClient, tf, infra.Namespace)
			}).RetryUntilTimeout(10*time.Second, 5*time.Minute),
			SkipIf: !configExists,
		})

		destroyKubernetesRoutes = g.Add(flow.Task{
			Name: "Destroying Kubernetes route entries",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				return cleanupKubernetesRoutes(ctx, config, gcpClient, tf, infra.Namespace)
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

	err = f.Run(ctx, flow.Opts{
		Log: log,
	})

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
	client gcpclient.ComputeClient,
	tf terraformer.Terraformer,
	shootSeedNamespace string,
) error {
	state, err := infrastructure.ExtractTerraformState(ctx, tf, config, false)
	if err != nil {
		if terraformer.IsVariablesNotFoundError(err) {
			return nil
		}
		return err
	}

	return infrastructure.CleanupKubernetesFirewalls(ctx, client, state.VPCName, shootSeedNamespace)
}

func cleanupKubernetesRoutes(
	ctx context.Context,
	config *api.InfrastructureConfig,
	client gcpclient.ComputeClient,
	tf terraformer.Terraformer,
	shootSeedNamespace string,
) error {
	state, err := infrastructure.ExtractTerraformState(ctx, tf, config, false)
	if err != nil {
		if terraformer.IsVariablesNotFoundError(err) {
			return nil
		}
		return err
	}

	return infrastructure.CleanupKubernetesRoutes(ctx, client, state.VPCName, shootSeedNamespace)
}

func (t *TerraformReconciler) computeTerraformStatus(
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
