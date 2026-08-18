// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
)

var _ = Describe("BYO subnet validation", func() {
	const (
		region      = "europe-west1"
		vpcName     = "my-vpc"
		vpcSelfLink = "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/my-vpc"

		workerSubnetName   = "my-workers"
		servicesSubnetName = "my-services"

		workerSubnetCIDR = "10.0.0.0/19"
		nodesCIDR        = "10.0.0.0/24"
		podsCIDR         = "100.128.0.0/11"
		servicesCIDR     = "192.168.0.0/16"
		overlappingCIDR  = "10.0.0.0/16"
	)

	var (
		ctrl       *gomock.Controller
		mockClient *mockgcpclient.MockComputeClient
		fctx       *FlowContext
	)

	newFctx := func(workerSubnet, servicesSubnet *gcp.SubnetReference, networking *gardencorev1beta1.Networking) *FlowContext {
		wb := shared.NewWhiteboard()
		wb.SetObject(ObjectKeyVPC, &compute.Network{
			Name:     vpcName,
			SelfLink: vpcSelfLink,
		})
		f := &FlowContext{
			computeClient: mockClient,
			whiteboard:    wb,
			networking:    networking,
			infra: &extensionsv1alpha1.Infrastructure{
				Spec: extensionsv1alpha1.InfrastructureSpec{
					Region: region,
					SecretRef: corev1.SecretReference{
						Name:      "cloudprovider",
						Namespace: "test",
					},
				},
				ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
			},
			config: &gcp.InfrastructureConfig{
				Networks: gcp.NetworkConfig{
					VPC:            &gcp.VPC{Name: vpcName},
					SubnetWorkers:  workerSubnet,
					SubnetServices: servicesSubnet,
				},
			},
		}
		f.BasicFlowContext = shared.NewBasicFlowContext()
		return f
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mockgcpclient.NewMockComputeClient(ctrl)
	})

	Describe("#ensureUserManagedWorkersSubnet", func() {
		It("should store the subnet on the whiteboard when it belongs to the configured VPC", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil, &gardencorev1beta1.Networking{
				Nodes:    ptr.To(nodesCIDR),
				Pods:     ptr.To(podsCIDR),
				Services: ptr.To(servicesCIDR),
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(&compute.Subnetwork{Name: workerSubnetName, Network: vpcSelfLink, IpCidrRange: workerSubnetCIDR}, nil)

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(Succeed())
			Expect(fctx.whiteboard.GetObject(ObjectKeyNodeSubnet)).NotTo(BeNil())
		})

		It("should fail when the subnet is not found", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil, nil)
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(nil, nil)

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail when the subnet belongs to a different VPC", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil, nil)
			otherVPCSelfLink := "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/other-vpc"
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(&compute.Subnetwork{Name: workerSubnetName, Network: otherVPCSelfLink, IpCidrRange: workerSubnetCIDR}, nil)

			err := fctx.ensureUserManagedWorkersSubnet(context.TODO())
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("not to the configured VPC %q", vpcName))))
		})

		It("should fail when the GCP API returns an error", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil, nil)
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(nil, fmt.Errorf("gcp api error"))

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(MatchError("gcp api error"))
		})

		It("should fail when the nodes CIDR is not contained in the worker subnet", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil, &gardencorev1beta1.Networking{
				Nodes: ptr.To("172.16.0.0/24"),
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(&compute.Subnetwork{Name: workerSubnetName, Network: vpcSelfLink, IpCidrRange: workerSubnetCIDR}, nil)

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(MatchError(ContainSubstring("must be a subset of")))
		})

		It("should fail when the worker subnet overlaps the pods CIDR", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil, &gardencorev1beta1.Networking{
				Pods: ptr.To(overlappingCIDR),
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(&compute.Subnetwork{Name: workerSubnetName, Network: vpcSelfLink, IpCidrRange: workerSubnetCIDR}, nil)

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(MatchError(ContainSubstring("must not overlap with")))
		})

		It("should fail when the worker subnet overlaps the services CIDR", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil, &gardencorev1beta1.Networking{
				Services: ptr.To(overlappingCIDR),
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(&compute.Subnetwork{Name: workerSubnetName, Network: vpcSelfLink, IpCidrRange: workerSubnetCIDR}, nil)

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(MatchError(ContainSubstring("must not overlap with")))
		})
	})

	Describe("#ensureUserManagedServicesSubnet", func() {
		It("should store the subnet on the whiteboard when it belongs to the configured VPC", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName}, &gardencorev1beta1.Networking{
				Nodes:    ptr.To(nodesCIDR),
				Pods:     ptr.To(podsCIDR),
				Services: ptr.To(servicesCIDR),
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(&compute.Subnetwork{Name: servicesSubnetName, Network: vpcSelfLink, IpCidrRange: servicesCIDR}, nil)

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(Succeed())
			Expect(fctx.whiteboard.GetObject(ObjectKeyServicesSubnet)).NotTo(BeNil())
		})

		It("should fail when the subnet is not found", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName}, nil)
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(nil, nil)

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail when the subnet belongs to a different VPC", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName}, nil)
			otherVPCSelfLink := "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/other-vpc"
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(&compute.Subnetwork{Name: servicesSubnetName, Network: otherVPCSelfLink, IpCidrRange: servicesCIDR}, nil)

			err := fctx.ensureUserManagedServicesSubnet(context.TODO())
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("not to the configured VPC %q", vpcName))))
		})

		It("should fail when the GCP API returns an error", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName}, nil)
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(nil, fmt.Errorf("gcp api error"))

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(MatchError("gcp api error"))
		})

		It("should fail when the services subnet CIDR does not match shoot networking.services", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName}, &gardencorev1beta1.Networking{
				Services: ptr.To(servicesCIDR),
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(&compute.Subnetwork{Name: servicesSubnetName, Network: vpcSelfLink, IpCidrRange: "10.1.0.0/16"}, nil)

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(MatchError(ContainSubstring("must match shoot networking.services")))
		})

		It("should fail when the services subnet overlaps the nodes CIDR", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName}, &gardencorev1beta1.Networking{
				Nodes: ptr.To("192.168.0.0/24"), // subnet of servicesCIDR → overlaps
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(&compute.Subnetwork{Name: servicesSubnetName, Network: vpcSelfLink, IpCidrRange: servicesCIDR}, nil)

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(MatchError(ContainSubstring("must not overlap with")))
		})

		It("should fail when the services subnet overlaps the pods CIDR", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName}, &gardencorev1beta1.Networking{
				Pods: ptr.To("192.168.1.0/24"), // subnet of servicesCIDR → overlaps
			})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(&compute.Subnetwork{Name: servicesSubnetName, Network: vpcSelfLink, IpCidrRange: servicesCIDR}, nil)

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(MatchError(ContainSubstring("must not overlap with")))
		})
	})
})
