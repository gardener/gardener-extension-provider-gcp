// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infraflow

import (
	"context"
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	)

	var (
		ctrl       *gomock.Controller
		mockClient *mockgcpclient.MockComputeClient
		fctx       *FlowContext
	)

	newFctx := func(workerSubnet, servicesSubnet *gcp.SubnetReference) *FlowContext {
		wb := shared.NewWhiteboard()
		wb.SetObject(ObjectKeyVPC, &compute.Network{
			Name:     vpcName,
			SelfLink: vpcSelfLink,
		})
		f := &FlowContext{
			computeClient: mockClient,
			whiteboard:    wb,
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
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil)
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(&compute.Subnetwork{Name: workerSubnetName, Network: vpcSelfLink}, nil)

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(Succeed())
			Expect(fctx.whiteboard.GetObject(ObjectKeyNodeSubnet)).NotTo(BeNil())
		})

		It("should fail when the subnet is not found", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil)
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(nil, nil)

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail when the subnet belongs to a different VPC", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil)
			otherVPCSelfLink := "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/other-vpc"
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(&compute.Subnetwork{Name: workerSubnetName, Network: otherVPCSelfLink}, nil)

			err := fctx.ensureUserManagedWorkersSubnet(context.TODO())
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("not to the configured VPC %q", vpcName))))
		})

		It("should fail when the GCP API returns an error", func() {
			fctx = newFctx(&gcp.SubnetReference{Name: workerSubnetName}, nil)
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, workerSubnetName).
				Return(nil, fmt.Errorf("gcp api error"))

			Expect(fctx.ensureUserManagedWorkersSubnet(context.TODO())).To(MatchError("gcp api error"))
		})
	})

	Describe("#ensureUserManagedServicesSubnet", func() {
		It("should store the subnet on the whiteboard when it belongs to the configured VPC", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(&compute.Subnetwork{Name: servicesSubnetName, Network: vpcSelfLink}, nil)

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(Succeed())
			Expect(fctx.whiteboard.GetObject(ObjectKeyServicesSubnet)).NotTo(BeNil())
		})

		It("should fail when the subnet is not found", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(nil, nil)

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail when the subnet belongs to a different VPC", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName})
			otherVPCSelfLink := "https://www.googleapis.com/compute/v1/projects/my-project/global/networks/other-vpc"
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(&compute.Subnetwork{Name: servicesSubnetName, Network: otherVPCSelfLink}, nil)

			err := fctx.ensureUserManagedServicesSubnet(context.TODO())
			Expect(err).To(MatchError(ContainSubstring(fmt.Sprintf("not to the configured VPC %q", vpcName))))
		})

		It("should fail when the GCP API returns an error", func() {
			fctx = newFctx(nil, &gcp.SubnetReference{Name: servicesSubnetName})
			mockClient.EXPECT().
				GetSubnet(gomock.Any(), region, servicesSubnetName).
				Return(nil, fmt.Errorf("gcp api error"))

			Expect(fctx.ensureUserManagedServicesSubnet(context.TODO())).To(MatchError("gcp api error"))
		})
	})
})
