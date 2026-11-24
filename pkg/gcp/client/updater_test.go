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

	Context("Router", func() {
		It("should not patch router when BGP state is unchanged", func() {
			desired := &compute.Router{Name: "test-router", Network: "test-vpc"}
			current := &compute.Router{Name: "test-router", Network: "test-vpc"}

			// PatchRouter must not be called
			result, err := updater.Router(ctx, "europe-west1", desired, current)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(current))
		})

		It("should patch router with desired (not current) when BgpPeers drift is detected", func() {
			desired := &compute.Router{Name: "test-router", Network: "test-vpc"}
			current := &compute.Router{
				Name:    "test-router",
				Network: "test-vpc",
				BgpPeers: []*compute.RouterBgpPeer{
					{Name: "rogue-peer", PeerIpAddress: "169.254.0.1", PeerAsn: 65001},
				},
			}

			patched := &compute.Router{Name: "test-router"}
			computeClient.EXPECT().
				PatchRouter(ctx, "europe-west1", "test-router", gomock.AssignableToTypeOf(&compute.Router{})).
				DoAndReturn(func(_ context.Context, _ string, _ string, r *compute.Router) (*compute.Router, error) {
					// must send desired, not current — rogue peer must not appear
					Expect(r.BgpPeers).To(BeEmpty())
					Expect(r.ForceSendFields).To(ContainElement("BgpPeers"))
					return patched, nil
				})

			result, err := updater.Router(ctx, "europe-west1", desired, current)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(patched))
		})

		It("should patch router with desired (not current) when Bgp drift is detected", func() {
			desired := &compute.Router{Name: "test-router", Network: "test-vpc"}
			current := &compute.Router{
				Name:    "test-router",
				Network: "test-vpc",
				Bgp:     &compute.RouterBgp{Asn: 65000},
			}

			patched := &compute.Router{Name: "test-router"}
			computeClient.EXPECT().
				PatchRouter(ctx, "europe-west1", "test-router", gomock.AssignableToTypeOf(&compute.Router{})).
				DoAndReturn(func(_ context.Context, _ string, _ string, r *compute.Router) (*compute.Router, error) {
					Expect(r.Bgp).To(BeNil())
					Expect(r.NullFields).To(ContainElement("Bgp"))
					return patched, nil
				})

			result, err := updater.Router(ctx, "europe-west1", desired, current)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(patched))
		})
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
	})
})
