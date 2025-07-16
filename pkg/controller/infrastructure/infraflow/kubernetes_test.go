// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/compute/v1"

	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
)

var _ = Describe("Infrastructure", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	Describe("#ListKubernetesFirewalls", func() {
		It("should list all kubernetes related firewall names", func() {
			var (
				ctx                     = context.TODO()
				network                 = "bar"
				shootSeedNamespace      = "shoot--foobar--gcp"
				otherShootSeedNamespace = "shoot--foo--other"

				k8sFirewallName   = fmt.Sprintf("%sbar-fw", KubernetesFirewallNamePrefix)
				shootFirewallName = fmt.Sprintf("%sbar-fw", shootSeedNamespace)

				otherK8SFirewallName   = fmt.Sprintf("%sother-fw", KubernetesFirewallNamePrefix)
				otherShootFirewallName = fmt.Sprintf("%sother-fw", otherShootSeedNamespace)

				firewalls = []*compute.Firewall{
					{Name: k8sFirewallName, Network: network, TargetTags: []string{shootSeedNamespace}},
					{Name: shootFirewallName, Network: network},
					{Name: otherK8SFirewallName, Network: network, TargetTags: []string{otherShootSeedNamespace}},
					{Name: otherShootFirewallName, Network: network},
				}

				expectedResult = []*compute.Firewall{
					{Name: k8sFirewallName, Network: network, TargetTags: []string{shootSeedNamespace}},
					{Name: shootFirewallName, Network: network},
				}

				client = mockgcpclient.NewMockComputeClient(ctrl)
			)

			client.EXPECT().ListFirewallRules(ctx, gomock.AssignableToTypeOf(gcpclient.FirewallListOpts{})).
				DoAndReturn(func(_ context.Context, _ gcpclient.FirewallListOpts) ([]*compute.Firewall, error) {
					var result []*compute.Firewall
					opts := CreateFirewallListOpts(network, shootSeedNamespace)
					for _, rule := range firewalls {
						if opts.ClientFilter(rule) {
							result = append(result, rule)
						}
					}
					return result, nil
				})

			actual, err := ListKubernetesFirewalls(ctx, client, network, shootSeedNamespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expectedResult))
		})
	})

	Describe("#ListKubernetesRoutes", func() {
		It("should list all kubernetes related route names with the shoot namespace as prefix", func() {
			var (
				ctx                     = context.TODO()
				projectID               = "foo"
				network                 = "bar"
				shootSeedNamespace      = "shoot--foobar--gcp"
				otherShootSeedNamespace = "shoot--foo--other"

				routeName      = fmt.Sprintf("%s-2690fa98-450f-11e9-8ebe-ce2a79d67b14", shootSeedNamespace)
				otherRouteName = fmt.Sprintf("%s-4123f123-4351-6234-91ee-asd3612op412", otherShootSeedNamespace)

				nextHopInstance = fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/zone-id/instances/%s-worker-tqba1-z1-7b74dd4b94-nsplm",
					projectID,
					shootSeedNamespace,
				)
				otherNextHopInstance = fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/zone-id/instances/%s-worker-tqba1-z1-7b74dd4b94-nsplm",
					projectID,
					otherShootSeedNamespace,
				)

				routes = []*compute.Route{
					{Name: routeName, Network: network, NextHopInstance: nextHopInstance},
					{Name: otherRouteName, Network: network, NextHopInstance: otherNextHopInstance},
					{Name: otherRouteName, Network: network},
				}

				expectedResult = []*compute.Route{
					{Name: routeName, Network: network, NextHopInstance: nextHopInstance},
				}

				client = mockgcpclient.NewMockComputeClient(ctrl)
			)

			client.EXPECT().ListRoutes(ctx, gomock.AssignableToTypeOf(gcpclient.RouteListOpts{})).
				DoAndReturn(func(_ context.Context, _ gcpclient.RouteListOpts) ([]*compute.Route, error) {
					var result []*compute.Route
					opts := CreateRoutesListOpts(network, shootSeedNamespace)
					for _, rule := range routes {
						if opts.ClientFilter(rule) {
							result = append(result, rule)
						}
					}
					return result, nil
				})

			actual, err := ListKubernetesRoutes(ctx, client, network, shootSeedNamespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(expectedResult))
		})
	})

	Describe("#DeleteFirewalls", func() {
		It("should delete all firewalls", func() {
			var (
				ctx           = context.TODO()
				firewallName  = fmt.Sprintf("%sfw", KubernetesFirewallNamePrefix)
				firewallRules = []*compute.Firewall{{Name: firewallName}}

				client = mockgcpclient.NewMockComputeClient(ctrl)
			)

			client.EXPECT().DeleteFirewallRule(ctx, firewallRules[0].Name).Return(nil)

			Expect(DeleteFirewalls(ctx, client, firewallRules)).To(Succeed())
		})
	})

	Describe("#DeleteRoutes", func() {
		It("should delete all routess", func() {
			var (
				ctx       = context.TODO()
				routeName = "shoot--foobar--gcp-2690fa98-450f-11e9-8ebe-ce2a79d67b14"
				routes    = []*compute.Route{{Name: routeName}}
				client    = mockgcpclient.NewMockComputeClient(ctrl)
			)

			client.EXPECT().DeleteRoute(ctx, routes[0].Name).Return(nil)

			Expect(DeleteRoutes(ctx, client, routes)).To(Succeed())
		})
	})
})
