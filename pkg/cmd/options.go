// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	extensionsbackupbucketcontroller "github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsbackupentrycontroller "github.com/gardener/gardener/extensions/pkg/controller/backupentry"
	extensionsbastioncontroller "github.com/gardener/gardener/extensions/pkg/controller/bastion"
	controllercmd "github.com/gardener/gardener/extensions/pkg/controller/cmd"
	extensionscontrolplanecontroller "github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	extensionsdnsrecordcontroller "github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionshealthcheckcontroller "github.com/gardener/gardener/extensions/pkg/controller/healthcheck"
	extensionsheartbeatcontroller "github.com/gardener/gardener/extensions/pkg/controller/heartbeat"
	extensionsinfrastructurecontroller "github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsworkercontroller "github.com/gardener/gardener/extensions/pkg/controller/worker"
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	extensioncontrolplanewebhook "github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	extensionshootwebhook "github.com/gardener/gardener/extensions/pkg/webhook/shoot"

	backupbucketcontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupbucket"
	backupentrycontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupentry"
	bastioncontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/bastion"
	controlplanecontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/controlplane"
	dnsrecordcontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/dnsrecord"
	healthcheckcontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/healthcheck"
	infrastructurecontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	workercontroller "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
	cloudproviderwebhook "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/cloudprovider"
	controlplanewebhook "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/controlplane"
	infrastructurewebhook "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/infrastructure"
	seedproviderwebhook "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/seedprovider"
	shootwebhook "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/shoot"
	servicewebhook "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/shootservice"
	terraformerwebhook "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/terraformer"
)

// ControllerSwitchOptions are the controllercmd.SwitchOptions for the provider controllers.
func ControllerSwitchOptions() *controllercmd.SwitchOptions {
	return controllercmd.NewSwitchOptions(
		controllercmd.Switch(extensionsbackupbucketcontroller.ControllerName, backupbucketcontroller.AddToManager),
		controllercmd.Switch(extensionsbackupentrycontroller.ControllerName, backupentrycontroller.AddToManager),
		controllercmd.Switch(extensionsbastioncontroller.ControllerName, bastioncontroller.AddToManager),
		controllercmd.Switch(extensionscontrolplanecontroller.ControllerName, controlplanecontroller.AddToManager),
		controllercmd.Switch(extensionsdnsrecordcontroller.ControllerName, dnsrecordcontroller.AddToManager),
		controllercmd.Switch(extensionsinfrastructurecontroller.ControllerName, infrastructurecontroller.AddToManager),
		controllercmd.Switch(extensionsworkercontroller.ControllerName, workercontroller.AddToManager),
		controllercmd.Switch(extensionshealthcheckcontroller.ControllerName, healthcheckcontroller.AddToManager),
		controllercmd.Switch(extensionsheartbeatcontroller.ControllerName, extensionsheartbeatcontroller.AddToManager),
	)
}

// WebhookSwitchOptions are the webhookcmd.SwitchOptions for the provider webhooks.
func WebhookSwitchOptions() *webhookcmd.SwitchOptions {
	return webhookcmd.NewSwitchOptions(
		webhookcmd.Switch(extensioncontrolplanewebhook.WebhookName, controlplanewebhook.AddToManager),
		webhookcmd.Switch(extensioncontrolplanewebhook.SeedProviderWebhookName, seedproviderwebhook.New),
		webhookcmd.Switch(cloudproviderwebhook.WebhookName, cloudproviderwebhook.AddToManager),
		webhookcmd.Switch(terraformerwebhook.WebhookName, terraformerwebhook.AddToManager),
		webhookcmd.Switch(infrastructurewebhook.WebhookName, infrastructurewebhook.AddToManager),
		webhookcmd.Switch(extensionshootwebhook.WebhookName, shootwebhook.AddToManager),
		webhookcmd.Switch(servicewebhook.WebhookName, servicewebhook.AddToManager),
		webhookcmd.Switch(servicewebhook.NginxIngressWebhookName, servicewebhook.AddNginxIngressWebhookToManager),
	)
}
