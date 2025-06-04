// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"strings"

	"google.golang.org/api/compute/v1"

	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

// KubernetesFirewallNamePrefix is the name prefix that Kubernetes related firewall rules have.
const (
	KubernetesFirewallNamePrefix string = "k8s"
	ShootPrefix                  string = "shoot--"
)

// CreateFirewallListOpts creates the FirewallListOpts options.
func CreateFirewallListOpts(network, shootSeedNamespace string) gcpclient.FirewallListOpts {
	return gcpclient.FirewallListOpts{
		ClientFilter: func(firewall *compute.Firewall) bool {
			if strings.HasSuffix(firewall.Network, network) {
				if strings.HasPrefix(firewall.Name, KubernetesFirewallNamePrefix) {
					for _, targetTag := range firewall.TargetTags {
						if targetTag == shootSeedNamespace {
							return true
						}
					}
				} else if strings.HasPrefix(firewall.Name, shootSeedNamespace) {
					return true
				}
			}
			return false
		},
	}
}

// ListKubernetesFirewalls lists all firewalls that are in the given network and for the given shoot and have the KubernetesFirewallNamePrefix.
func ListKubernetesFirewalls(ctx context.Context, client gcpclient.ComputeClient, network, shootSeedNamespace string) ([]*compute.Firewall, error) {
	opts := CreateFirewallListOpts(network, shootSeedNamespace)
	return client.ListFirewallRules(ctx, opts)
}

// CreateRoutesListOpts creates the RouteListOpts options.
func CreateRoutesListOpts(network, shootSeedNamespace string) gcpclient.RouteListOpts {
	return gcpclient.RouteListOpts{
		ClientFilter: func(route *compute.Route) bool {
			if strings.HasPrefix(route.Name, ShootPrefix) && strings.HasSuffix(route.Network, network) {
				urlParts := strings.Split(route.NextHopInstance, "/")
				if strings.HasPrefix(urlParts[len(urlParts)-1], shootSeedNamespace) {
					return true
				}
			}
			return false
		},
	}
}

// ListKubernetesRoutes returns a list of all routes within the shoot network which have the shoot's seed namespace as prefix.
func ListKubernetesRoutes(ctx context.Context, client gcpclient.ComputeClient, network, shootSeedNamespace string) ([]*compute.Route, error) {
	opts := CreateRoutesListOpts(network, shootSeedNamespace)
	return client.ListRoutes(ctx, opts)
}

// DeleteFirewalls deletes the firewalls with the given names in the given project.
//
// If a deletion fails, it immediately returns the error of that deletion.
func DeleteFirewalls(ctx context.Context, client gcpclient.ComputeClient, firewalls []*compute.Firewall) error {
	for _, firewall := range firewalls {
		if err := client.DeleteFirewallRule(ctx, firewall.Name); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRoutes deletes the route entries with the given names in the given project.
//
// If a deletion fails, it immediately returns the error of that deletion.
func DeleteRoutes(ctx context.Context, client gcpclient.ComputeClient, routes []*compute.Route) error {
	for _, route := range routes {
		if err := client.DeleteRoute(ctx, route.Name); err != nil {
			return err
		}
	}
	return nil
}
