// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	computev1 "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

const (
	// SSHPort is the default SSH Port used for bastion ingress firewall rule
	SSHPort = 22
)

type actuator struct {
	client client.Client
}

func newActuator(mgr manager.Manager) bastion.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

func getBastionInstance(ctx context.Context, client gcpclient.ComputeClient, opt BaseOptions) (*computev1.Instance, error) {
	instance, err := client.GetInstance(ctx, opt.Zone, opt.BastionInstanceName)
	return instance, gcpclient.IgnoreNotFoundError(err)
}

func createFirewallRuleIfNotExist(ctx context.Context, log logr.Logger, client gcpclient.ComputeClient, firewallRule *computev1.Firewall) error {
	if _, err := client.InsertFirewallRule(ctx, firewallRule); err != nil {
		if gcpclient.IsErrorCode(err, http.StatusConflict) {
			return nil
		}
		return fmt.Errorf("could not create firewall rule %s: %w", firewallRule.Name, err)
	}

	log.Info("Firewall created", "firewall", firewallRule.Name)
	return nil
}

func patchFirewallRule(ctx context.Context, client gcpclient.ComputeClient, firewallRuleName string, cidrs []string) error {
	if _, err := client.PatchFirewallRule(ctx, firewallRuleName, patchCIDRs(cidrs)); err != nil {
		return err
	}
	return nil
}

func getDisk(ctx context.Context, client gcpclient.ComputeClient, opt BaseOptions) (*computev1.Disk, error) {
	disk, err := client.GetDisk(ctx, opt.Zone, opt.DiskName)
	return disk, gcpclient.IgnoreNotFoundError(err)
}

func getCredentialsConfig(ctx context.Context, reader client.Reader, bastion *v1alpha1.Bastion) (*gcp.CredentialsConfig, error) {
	return gcp.GetCredentialsConfigFromSecretReference(ctx, reader, corev1.SecretReference{Namespace: bastion.Namespace, Name: constants.SecretNameCloudProvider})
}

func getWorkersCIDR(ctx context.Context, cluster *controller.Cluster, gcpClient gcpclient.ComputeClient) (string, error) {
	infrastructureConfig := &apisgcp.InfrastructureConfig{}
	if err := json.Unmarshal(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, infrastructureConfig); err != nil {
		return "", err
	}

	if len(infrastructureConfig.Networks.Workers) > 0 {
		return infrastructureConfig.Networks.Workers, nil
	}

	// BYO mode: Workers is empty — look up the subnet's primary CIDR from GCP.
	if infrastructureConfig.Networks.SubnetWorkers != nil {
		subnet, err := gcpClient.GetSubnet(ctx, cluster.Shoot.Spec.Region, infrastructureConfig.Networks.SubnetWorkers.Name)
		if err != nil {
			return "", fmt.Errorf("failed to get worker subnet %q: %w", infrastructureConfig.Networks.SubnetWorkers.Name, err)
		}
		if subnet == nil {
			return "", fmt.Errorf("worker subnet %q not found", infrastructureConfig.Networks.SubnetWorkers.Name)
		}
		return subnet.IpCidrRange, nil
	}

	return "", fmt.Errorf("no workers CIDR found: neither Networks.Workers nor Networks.SubnetWorkers is set")
}

func getDefaultGCPZone(ctx context.Context, client gcpclient.ComputeClient, region string) (string, error) {
	resp, err := client.GetRegion(ctx, region)
	if err != nil {
		return "", err
	}
	if len(resp.Zones) > 0 {
		zone := strings.Split(resp.Zones[0], "/")
		return zone[(len(zone) - 1)], nil
	}
	return "", fmt.Errorf("no available zones in GCP region: %s", region)
}
