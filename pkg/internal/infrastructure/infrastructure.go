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
	"strings"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"google.golang.org/api/compute/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/client"
)

// KubernetesFirewallNamePrefix is the name prefix that Kubernetes related firewall rules have.
const (
	KubernetesFirewallNamePrefix string = "k8s"
	shootPrefix                  string = "shoot--"
)

// ListKubernetesFirewalls lists all firewalls that are in the given network and for the given shoot and have the KubernetesFirewallNamePrefix.
func ListKubernetesFirewalls(ctx context.Context, client gcpclient.Interface, projectID, network, shootSeedNamespace string) ([]string, error) {
	var names []string
	err := client.Firewalls().List(projectID).Pages(ctx, func(list *compute.FirewallList) error {
		for _, firewall := range list.Items {
			if strings.HasSuffix(firewall.Network, network) {
				if strings.HasPrefix(firewall.Name, KubernetesFirewallNamePrefix) {
					for _, targetTag := range firewall.TargetTags {
						if targetTag == shootSeedNamespace {
							names = append(names, firewall.Name)
							break
						}
					}
				} else if strings.HasPrefix(firewall.Name, shootSeedNamespace) {
					names = append(names, firewall.Name)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}

// ListKubernetesRoutes returns a list of all routes within the shoot network which have the shoot's seed namespace as prefix.
func ListKubernetesRoutes(ctx context.Context, client gcpclient.Interface, projectID, network, shootSeedNamespace string) ([]string, error) {
	var routes []string
	if err := client.Routes().List(projectID).Pages(ctx, func(page *compute.RouteList) error {
		for _, route := range page.Items {
			if strings.HasPrefix(route.Name, shootPrefix) && strings.HasSuffix(route.Network, network) {
				urlParts := strings.Split(route.NextHopInstance, "/")
				if strings.HasPrefix(urlParts[len(urlParts)-1], shootSeedNamespace) {
					routes = append(routes, route.Name)
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return routes, nil
}

// DeleteFirewalls deletes the firewalls with the given names in the given project.
//
// If a deletion fails, it immediately returns the error of that deletion.
func DeleteFirewalls(ctx context.Context, client gcpclient.Interface, projectID string, firewalls []string) error {
	for _, firewall := range firewalls {
		if _, err := client.Firewalls().Delete(projectID, firewall).Context(ctx).Do(); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRoutes deletes the route entries with the given names in the given project.
//
// If a deletion fails, it immediately returns the error of that deletion.
func DeleteRoutes(ctx context.Context, client gcpclient.Interface, projectID string, routes []string) error {
	for _, route := range routes {
		if _, err := client.Routes().Delete(projectID, route).Context(ctx).Do(); err != nil {
			return err
		}
	}
	return nil
}

// CleanupKubernetesFirewalls lists all Kubernetes firewall rules and then deletes them one after another.
//
// If a deletion fails, this method returns immediately with the encountered error.
func CleanupKubernetesFirewalls(ctx context.Context, client gcpclient.Interface, projectID, network, shootSeedNamespace string) error {
	firewallNames, err := ListKubernetesFirewalls(ctx, client, projectID, network, shootSeedNamespace)
	if err != nil {
		return err
	}

	return DeleteFirewalls(ctx, client, projectID, firewallNames)
}

// CleanupKubernetesRoutes lists all Kubernetes route rules and then deletes them one after another.
//
// If a deletion fails, this method returns immediately with the encountered error.
func CleanupKubernetesRoutes(ctx context.Context, client gcpclient.Interface, projectID, network, shootSeedNamespace string) error {
	routeNames, err := ListKubernetesRoutes(ctx, client, projectID, network, shootSeedNamespace)
	if err != nil {
		return err
	}

	return DeleteRoutes(ctx, client, projectID, routeNames)
}

// GetServiceAccountFromInfrastructure retrieves the ServiceAccount from the Secret referenced in the given Infrastructure.
func GetServiceAccountFromInfrastructure(ctx context.Context, c client.Client, config *extensionsv1alpha1.Infrastructure) (*gcp.ServiceAccount, error) {
	return gcp.GetServiceAccountFromSecretReference(ctx, c, config.Spec.SecretRef)
}
