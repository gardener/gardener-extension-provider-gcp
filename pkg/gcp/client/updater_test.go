package client_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/compute/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	gcpmock "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
)

var _ = Describe("VPC Updater", func() {
	var (
		ctrl          *gomock.Controller
		computeClient *gcpmock.MockComputeClient
		updater       gcpclient.Updater
		ctx           context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		computeClient = gcpmock.NewMockComputeClient(ctrl)
		updater = gcpclient.NewUpdater(log.Log, computeClient)
		ctx = context.Background()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("RoutingConfig", func() {
		It("should not patch VPC when RoutingConfig is unchanged", func() {
			desired := &compute.Network{
				Name:                  "test-vpc",
				AutoCreateSubnetworks: false,
				RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "REGIONAL"},
				Mtu:                   8100,
			}
			current := &compute.Network{
				Name:                  "test-vpc",
				AutoCreateSubnetworks: false,
				RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "REGIONAL"},
				Mtu:                   8100,
			}

			// PatchNetwork must not be called
			result, err := updater.VPC(ctx, desired, current)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(current))
		})

		It("should patch VPC when RoutingConfig changes", func() {
			desired := &compute.Network{
				Name:                  "test-vpc",
				AutoCreateSubnetworks: false,
				RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "GLOBAL"},
				Mtu:                   8100,
			}
			current := &compute.Network{
				Name:                  "test-vpc",
				AutoCreateSubnetworks: false,
				RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "REGIONAL"},
				Mtu:                   8100,
			}

			patched := &compute.Network{Name: "test-vpc"}
			computeClient.EXPECT().
				PatchNetwork(ctx, "test-vpc", gomock.AssignableToTypeOf(&compute.Network{})).
				DoAndReturn(func(_ context.Context, _ string, nw *compute.Network) (*compute.Network, error) {
					// MTU must not be sent in the PATCH
					Expect(nw.Mtu).To(BeZero())
					return patched, nil
				})

			result, err := updater.VPC(ctx, desired, current)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(patched))
		})

		It("should not patch VPC when RoutingConfig pointer differs but value is equal", func() {
			// This is the core regression test: pointer comparison would incorrectly
			// trigger a PATCH even though nothing changed.
			desired := &compute.Network{
				Name:                  "test-vpc",
				AutoCreateSubnetworks: false,
				RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "REGIONAL"},
				Mtu:                   8100,
			}
			current := &compute.Network{
				Name:                  "test-vpc",
				AutoCreateSubnetworks: false,
				RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "REGIONAL"},
				Mtu:                   8100,
			}

			// PatchNetwork must not be called even though desired and current have different pointers
			result, err := updater.VPC(ctx, desired, current)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(current))
		})
	})
})
