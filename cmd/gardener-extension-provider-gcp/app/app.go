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

package app

import (
	"context"
	"fmt"
	"os"

	gcpinstall "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
	gcpcmd "github.com/gardener/gardener-extension-provider-gcp/pkg/cmd"
	gcpbackupbucket "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupbucket"
	gcpbackupentry "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupentry"
	gcpcontrolplane "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/controlplane"
	gcpcsimigration "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/csimigration"
	gcpdnsrecord "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/healthcheck"
	gcpinfrastructure "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	gcpworker "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpcontrolplaneexposure "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/controlplaneexposure"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	gcpbastion "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/bastion"
	"github.com/gardener/gardener/extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/util"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	gardenerhealthz "github.com/gardener/gardener/pkg/healthz"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/version/verflag"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewControllerManagerCommand creates a new command for running a GCP provider controller.
func NewControllerManagerCommand(ctx context.Context) *cobra.Command {
	var (
		generalOpts = &controllercmd.GeneralOptions{}
		restOpts    = &controllercmd.RESTOptions{}
		mgrOpts     = &controllercmd.ManagerOptions{
			LeaderElection:             true,
			LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
			LeaderElectionID:           controllercmd.LeaderElectionNameID(gcp.Name),
			LeaderElectionNamespace:    os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:          443,
			WebhookCertDir:             "/tmp/gardener-extensions-cert",
			HealthBindAddress:          ":8081",
		}
		configFileOpts = &gcpcmd.ConfigOptions{}

		// options for the backupbucket controller
		backupBucketCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the backupentry controller
		backupEntryCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the bastion controller
		bastionCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the health care controller
		healthCheckCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the controlplane controller
		controlPlaneCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the csimigration controller
		csiMigrationCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the dnsrecord controller
		dnsRecordCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		// options for the infrastructure controller
		infraCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		reconcileOpts = &controllercmd.ReconcilerOptions{}

		// options for the worker controller
		workerCtrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}
		workerReconcileOpts = &worker.Options{
			DeployCRDs: true,
		}
		workerCtrlOptsUnprefixed = controllercmd.NewOptionAggregator(workerCtrlOpts, workerReconcileOpts)

		// options for the webhook server
		webhookServerOptions = &webhookcmd.ServerOptions{
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}

		controllerSwitches = gcpcmd.ControllerSwitchOptions()
		webhookSwitches    = gcpcmd.WebhookSwitchOptions()
		webhookOptions     = webhookcmd.NewAddToManagerOptions(gcp.Name, webhookServerOptions, webhookSwitches)

		aggOption = controllercmd.NewOptionAggregator(
			generalOpts,
			restOpts,
			mgrOpts,
			controllercmd.PrefixOption("backupbucket-", backupBucketCtrlOpts),
			controllercmd.PrefixOption("backupentry-", backupEntryCtrlOpts),
			controllercmd.PrefixOption("bastion-", bastionCtrlOpts),
			controllercmd.PrefixOption("controlplane-", controlPlaneCtrlOpts),
			controllercmd.PrefixOption("csimigration-", csiMigrationCtrlOpts),
			controllercmd.PrefixOption("dnsrecord-", dnsRecordCtrlOpts),
			controllercmd.PrefixOption("infrastructure-", infraCtrlOpts),
			controllercmd.PrefixOption("worker-", &workerCtrlOptsUnprefixed),
			controllercmd.PrefixOption("healthcheck-", healthCheckCtrlOpts),
			configFileOpts,
			controllerSwitches,
			reconcileOpts,
			webhookOptions,
		)
	)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("%s-controller-manager", gcp.Name),

		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()

			if err := aggOption.Complete(); err != nil {
				return fmt.Errorf("error completing options: %w", err)
			}

			util.ApplyClientConnectionConfigurationToRESTConfig(configFileOpts.Completed().Config.ClientConnection, restOpts.Completed().Config)

			if workerReconcileOpts.Completed().DeployCRDs {
				if err := worker.ApplyMachineResourcesForConfig(ctx, restOpts.Completed().Config); err != nil {
					return fmt.Errorf("error ensuring the machine CRDs: %w", err)
				}
			}

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				return fmt.Errorf("could not instantiate manager: %w", err)
			}

			scheme := mgr.GetScheme()
			if err := controller.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := gcpinstall.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := druidv1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := autoscalingv1beta2.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}
			if err := machinev1alpha1.AddToScheme(scheme); err != nil {
				return fmt.Errorf("could not update manager scheme: %w", err)
			}

			// add common meta types to schema for controller-runtime to use v1.ListOptions
			metav1.AddToGroupVersion(scheme, machinev1alpha1.SchemeGroupVersion)

			useTokenRequestor, err := controller.UseTokenRequestor(generalOpts.Completed().GardenerVersion)
			if err != nil {
				return fmt.Errorf("could not determine whether token requestor should be used: %w", err)
			}
			gcpcontrolplane.DefaultAddOptions.UseTokenRequestor = useTokenRequestor
			gcpworker.DefaultAddOptions.UseTokenRequestor = useTokenRequestor

			useProjectedTokenMount, err := controller.UseServiceAccountTokenVolumeProjection(generalOpts.Completed().GardenerVersion)
			if err != nil {
				return fmt.Errorf("could not determine whether service account token volume projection should be used: %w", err)
			}
			gcpcontrolplane.DefaultAddOptions.UseProjectedTokenMount = useProjectedTokenMount
			gcpinfrastructure.DefaultAddOptions.UseProjectedTokenMount = useProjectedTokenMount
			gcpworker.DefaultAddOptions.UseProjectedTokenMount = useProjectedTokenMount

			configFileOpts.Completed().ApplyETCDStorage(&gcpcontrolplaneexposure.DefaultAddOptions.ETCDStorage)
			configFileOpts.Completed().ApplyHealthCheckConfig(&healthcheck.DefaultAddOptions.HealthCheckConfig)
			healthCheckCtrlOpts.Completed().Apply(&healthcheck.DefaultAddOptions.Controller)
			backupBucketCtrlOpts.Completed().Apply(&gcpbackupbucket.DefaultAddOptions.Controller)
			backupEntryCtrlOpts.Completed().Apply(&gcpbackupentry.DefaultAddOptions.Controller)
			bastionCtrlOpts.Completed().Apply(&gcpbastion.DefaultAddOptions.Controller)
			controlPlaneCtrlOpts.Completed().Apply(&gcpcontrolplane.DefaultAddOptions.Controller)
			csiMigrationCtrlOpts.Completed().Apply(&gcpcsimigration.DefaultAddOptions.Controller)
			dnsRecordCtrlOpts.Completed().Apply(&gcpdnsrecord.DefaultAddOptions.Controller)
			infraCtrlOpts.Completed().Apply(&gcpinfrastructure.DefaultAddOptions.Controller)
			reconcileOpts.Completed().Apply(&gcpinfrastructure.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&gcpcontrolplane.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&gcpworker.DefaultAddOptions.IgnoreOperationAnnotation)
			reconcileOpts.Completed().Apply(&gcpbastion.DefaultAddOptions.IgnoreOperationAnnotation)
			workerCtrlOpts.Completed().Apply(&gcpworker.DefaultAddOptions.Controller)

			if _, _, err := webhookOptions.Completed().AddToManager(ctx, mgr); err != nil {
				return fmt.Errorf("could not add webhooks to manager: %w", err)
			}

			if err := controllerSwitches.Completed().AddToManager(mgr); err != nil {
				return fmt.Errorf("could not add controllers to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("informer-sync", gardenerhealthz.NewCacheSyncHealthz(mgr.GetCache())); err != nil {
				return fmt.Errorf("could not add readycheck for informers: %w", err)
			}

			if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
				return fmt.Errorf("could not add health check to manager: %w", err)
			}

			if err := mgr.AddReadyzCheck("webhook-server", mgr.GetWebhookServer().StartedChecker()); err != nil {
				return fmt.Errorf("could not add ready check for webhook server to manager: %w", err)
			}

			if err := mgr.Start(ctx); err != nil {
				return fmt.Errorf("error running manager: %w", err)
			}

			return nil
		},
	}

	verflag.AddFlags(cmd.Flags())
	aggOption.AddFlags(cmd.Flags())

	return cmd
}
