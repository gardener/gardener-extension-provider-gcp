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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/compute/v1"

	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/internal/mock/client"
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
				projectID               = "foo"
				network                 = "bar"
				shootSeedNamespace      = "shoot--foobar--gcp"
				otherShootSeedNamespace = "shoot--foo--other"

				k8sFirewallName   = fmt.Sprintf("%sbar-fw", KubernetesFirewallNamePrefix)
				shootFirewallName = fmt.Sprintf("%sbar-fw", shootSeedNamespace)

				оtherK8SFirewallName   = fmt.Sprintf("%sother-fw", KubernetesFirewallNamePrefix)
				otherShootFirewallName = fmt.Sprintf("%sother-fw", otherShootSeedNamespace)

				firewallNames = []string{k8sFirewallName, shootFirewallName}

				client            = mockgcpclient.NewMockInterface(ctrl)
				firewalls         = mockgcpclient.NewMockFirewallsService(ctrl)
				firewallsListCall = mockgcpclient.NewMockFirewallsListCall(ctrl)
			)

			gomock.InOrder(
				client.EXPECT().Firewalls().Return(firewalls),
				firewalls.EXPECT().List(projectID).Return(firewallsListCall),
				firewallsListCall.EXPECT().Pages(ctx, gomock.AssignableToTypeOf(func(*compute.FirewallList) error { return nil })).
					DoAndReturn(func(_ context.Context, f func(*compute.FirewallList) error) error {
						return f(&compute.FirewallList{
							Items: []*compute.Firewall{
								{Name: k8sFirewallName, Network: network, TargetTags: []string{shootSeedNamespace}},
								{Name: shootFirewallName, Network: network},
								{Name: оtherK8SFirewallName, Network: network, TargetTags: []string{otherShootSeedNamespace}},
								{Name: otherShootFirewallName, Network: network},
							},
						})
					}),
			)

			actual, err := ListKubernetesFirewalls(ctx, client, projectID, network, shootSeedNamespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(firewallNames))
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

				routeNames = []string{routeName}

				client         = mockgcpclient.NewMockInterface(ctrl)
				routes         = mockgcpclient.NewMockRoutesService(ctrl)
				routesListCall = mockgcpclient.NewMockRoutesListCall(ctrl)
			)

			gomock.InOrder(
				client.EXPECT().Routes().Return(routes),
				routes.EXPECT().List(projectID).Return(routesListCall),
				routesListCall.EXPECT().Pages(ctx, gomock.AssignableToTypeOf(func(*compute.RouteList) error { return nil })).
					DoAndReturn(func(_ context.Context, f func(*compute.RouteList) error) error {
						return f(&compute.RouteList{
							Items: []*compute.Route{
								{Name: routeName, Network: network, NextHopInstance: nextHopInstance},
								{Name: otherRouteName, Network: network, NextHopInstance: otherNextHopInstance},
								{Name: otherRouteName, Network: network},
							},
						})
					}),
			)

			actual, err := ListKubernetesRoutes(ctx, client, projectID, network, shootSeedNamespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(routeNames))
		})
	})

	Describe("#DeleteFirewalls", func() {
		It("should delete all firewalls", func() {
			var (
				ctx       = context.TODO()
				projectID = "foo"

				firewallName  = fmt.Sprintf("%sfw", KubernetesFirewallNamePrefix)
				firewallNames = []string{firewallName}

				client              = mockgcpclient.NewMockInterface(ctrl)
				firewalls           = mockgcpclient.NewMockFirewallsService(ctrl)
				firewallsDeleteCall = mockgcpclient.NewMockFirewallsDeleteCall(ctrl)
			)

			gomock.InOrder(
				client.EXPECT().Firewalls().Return(firewalls),
				firewalls.EXPECT().Delete(projectID, firewallName).Return(firewallsDeleteCall),
				firewallsDeleteCall.EXPECT().Context(ctx).Return(firewallsDeleteCall),
				firewallsDeleteCall.EXPECT().Do(),
			)

			Expect(DeleteFirewalls(ctx, client, projectID, firewallNames)).To(Succeed())
		})
	})

	Describe("#DeleteRoutes", func() {
		It("should delete all routess", func() {
			var (
				ctx       = context.TODO()
				projectID = "foo"

				routeName  = "shoot--foobar--gcp-2690fa98-450f-11e9-8ebe-ce2a79d67b14"
				routeNames = []string{routeName}

				client           = mockgcpclient.NewMockInterface(ctrl)
				routes           = mockgcpclient.NewMockRoutesService(ctrl)
				routesDeleteCall = mockgcpclient.NewMockRoutesDeleteCall(ctrl)
			)

			gomock.InOrder(
				client.EXPECT().Routes().Return(routes),
				routes.EXPECT().Delete(projectID, routeName).Return(routesDeleteCall),
				routesDeleteCall.EXPECT().Context(ctx).Return(routesDeleteCall),
				routesDeleteCall.EXPECT().Do(),
			)

			Expect(DeleteRoutes(ctx, client, projectID, routeNames)).To(Succeed())
		})
	})
})
