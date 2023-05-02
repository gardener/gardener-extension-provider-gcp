// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bastion

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/bastion"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gcpapi "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client"
)

const (
	// SSHPort is the default SSH Port used for bastion ingress firewall rule
	SSHPort = 22
)

type actuator struct {
	common.ClientContext
}

func newActuator() bastion.Actuator {
	return &actuator{}
}

func getBastionInstance(ctx context.Context, gcpclient gcpclient.Interface, opt *Options) (*compute.Instance, error) {
	instance, err := gcpclient.Instances().Get(opt.ProjectID, opt.Zone, opt.BastionInstanceName).Context(ctx).Do()
	if err != nil {
		if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return instance, nil
}

func getFirewallRule(ctx context.Context, gcpclient gcpclient.Interface, opt *Options, firewallRuleName string) (*compute.Firewall, error) {
	firewall, err := gcpclient.Firewalls().Get(opt.ProjectID, firewallRuleName).Context(ctx).Do()
	if err != nil {
		if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return firewall, nil
}

func createFirewallRuleIfNotExist(ctx context.Context, log logr.Logger, gcpclient gcpclient.Interface, opt *Options, firewallRule *compute.Firewall) error {
	if _, err := gcpclient.Firewalls().Insert(opt.ProjectID, firewallRule).Context(ctx).Do(); err != nil {
		if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == http.StatusConflict {
			return nil
		}
		return fmt.Errorf("could not create firewall rule %s: %w", firewallRule.Name, err)
	}

	log.Info("Firewall created", "firewall", firewallRule.Name)
	return nil
}

func deleteFirewallRule(ctx context.Context, log logr.Logger, gcpclient gcpclient.Interface, opt *Options, firewallRuleName string) error {
	if _, err := gcpclient.Firewalls().Delete(opt.ProjectID, firewallRuleName).Context(ctx).Do(); err != nil {
		if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("failed to delete firewall rule %s: %w", firewallRuleName, err)
	}

	log.Info("Firewall rule removed", "rule", firewallRuleName)
	return nil
}

func patchFirewallRule(ctx context.Context, gcpclient gcpclient.Interface, opt *Options, firewallRuleName string, cidrs []string) error {
	if _, err := gcpclient.Firewalls().Patch(opt.ProjectID, firewallRuleName, patchCIDRs(cidrs)).Context(ctx).Do(); err != nil {
		return err
	}
	return nil
}

func getDisk(ctx context.Context, gcpclient gcpclient.Interface, opt *Options) (*compute.Disk, error) {
	disk, err := gcpclient.Disks().Get(opt.ProjectID, opt.Zone, opt.DiskName).Context(ctx).Do()
	if err != nil {
		if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return disk, nil
}

func getServiceAccount(ctx context.Context, c client.Client, bastion *v1alpha1.Bastion) (*gcp.ServiceAccount, error) {
	return gcp.GetServiceAccountFromSecretReference(ctx, c, corev1.SecretReference{Namespace: bastion.Namespace, Name: constants.SecretNameCloudProvider})
}

func createGCPClient(ctx context.Context, serviceAccount *gcp.ServiceAccount) (gcpclient.Interface, error) {
	return gcpclient.NewFromServiceAccount(ctx, serviceAccount.Raw)
}

func getWorkersCIDR(cluster *controller.Cluster) (string, error) {
	infrastructureConfig := &gcpapi.InfrastructureConfig{}
	err := json.Unmarshal(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, infrastructureConfig)
	if err != nil {
		return "", err
	}
	return infrastructureConfig.Networks.Workers, nil
}

func getDefaultGCPZone(ctx context.Context, gcpclient gcpclient.Interface, opt *Options, region string) (string, error) {
	resp, err := gcpclient.Regions().Get(opt.ProjectID, region).Context(ctx).Do()
	if err != nil {
		return "", err
	}
	if len(resp.Zones) > 0 {
		zone := strings.Split(resp.Zones[0], "/")
		return zone[(len(zone) - 1)], nil
	}
	return "", fmt.Errorf("no available zones in GCP region: %s", region)
}
