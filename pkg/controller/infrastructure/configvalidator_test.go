// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	infractrl "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
)

const (
	name      = "infrastructure"
	namespace = "shoot--foobar--gcp"
	region    = "europe-west1"
)

var _ = Describe("ConfigValidator", func() {
	var (
		ctrl             *gomock.Controller
		gcpClientFactory *mockgcpclient.MockFactory
		gcpComputeClient *mockgcpclient.MockComputeClient
		ctx              context.Context
		logger           logr.Logger
		cv               infrastructure.ConfigValidator
		infra            *extensionsv1alpha1.Infrastructure
		mgr              *test.FakeManager
		c                client.Client
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gcpClientFactory = mockgcpclient.NewMockFactory(ctrl)
		gcpComputeClient = mockgcpclient.NewMockComputeClient(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		scheme := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
		c = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		mgr = &test.FakeManager{Client: c}

		cv = infractrl.NewConfigValidator(mgr, logger, gcpClientFactory)

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: gcp.Type,
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisgcp.InfrastructureConfig{
							Networks: apisgcp.NetworkConfig{
								CloudNAT: &apisgcp.CloudNAT{
									NatIPNames: []apisgcp.NatIPName{
										{Name: "test1"},
										{Name: "test2"},
									},
								},
							},
						}),
					},
				},
				Region: region,
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			},
		}
	})

	Describe("#Validate", func() {
		BeforeEach(func() {
			gcpClientFactory.EXPECT().Compute(ctx, gomock.Any(), infra.Spec.SecretRef).Return(gcpComputeClient, nil)
		})

		It("should succeed if there are no NAT IP names", func() {
			infra.Spec.ProviderConfig.Raw = encode(&apisgcp.InfrastructureConfig{
				Networks: apisgcp.NetworkConfig{},
			})

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid NAT IP names that don't exist or are not available", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string][]string{
				"test2": {"foo"},
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotFound),
				"Field": Equal("networks.cloudNAT.natIPNames[0].name"),
			}, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("networks.cloudNAT.natIPNames[1].name"),
				"Detail": Equal("external IP address is already in use by foo"),
			}))
		})

		It("should allow NAT IP names that exist and are available, or in use by the default cloud router", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string][]string{
				"test1": nil,
				"test2": {namespace + "-cloud-router"},
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow NAT IP names that exist and are in use by the configured cloud router", func() {
			infra.Spec.ProviderConfig.Raw = encode(&apisgcp.InfrastructureConfig{
				Networks: apisgcp.NetworkConfig{
					VPC: &apisgcp.VPC{
						Name: "test-vpc",
						CloudRouter: &apisgcp.CloudRouter{
							Name: "test-cloud-router",
						},
					},
					CloudNAT: &apisgcp.CloudNAT{
						NatIPNames: []apisgcp.NatIPName{
							{Name: "test1"},
						},
					},
				},
			})
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(map[string][]string{
				"test1": {"test-cloud-router"},
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should fail with InternalError if getting external addresses failed", func() {
			gcpComputeClient.EXPECT().GetExternalAddresses(ctx, region).Return(nil, errors.New("test"))

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":   Equal(field.ErrorTypeInternal),
				"Field":  Equal("networks"),
				"Detail": Equal("could not get external IP addresses: test"),
			}))
		})
	})

	Describe("#Validate BYO worker subnet", func() {
		const (
			workerSubnetName = "byo-workers"
			vpcName          = "test-vpc"
			vpcSelfLink      = "https://www.googleapis.com/compute/v1/projects/test/global/networks/test-vpc"
		)

		byoConfig := func() *apisgcp.InfrastructureConfig {
			return &apisgcp.InfrastructureConfig{
				Networks: apisgcp.NetworkConfig{
					VPC: &apisgcp.VPC{Name: vpcName},
					SubnetWorkers: &apisgcp.SubnetReference{
						Name: workerSubnetName,
					},
				},
			}
		}

		BeforeEach(func() {
			infra.Spec.ProviderConfig.Raw = encode(byoConfig())
			gcpClientFactory.EXPECT().Compute(ctx, gomock.Any(), infra.Spec.SecretRef).Return(gcpComputeClient, nil)
			gcpComputeClient.EXPECT().GetNetwork(ctx, vpcName).Return(&compute.Network{Name: vpcName, SelfLink: vpcSelfLink}, nil).AnyTimes()
			// nodes fits inside the worker subnet 10.250.0.0/19; pods and services live in separate ranges.
			Expect(c.Create(ctx, newCluster(namespace, "10.250.0.0/24", "100.96.0.0/11", "100.64.0.0/13"))).To(Succeed())
		})

		It("should succeed if the worker subnet CIDR fits the cluster networking", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.250.0.0/19",
				Network:     vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid a worker subnet that does not contain the node network", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.180.0.0/19",
				Network:     vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networking.nodes"),
			}))))
		})

		It("should forbid a worker subnet CIDR that overlaps with the pod network", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "100.96.0.0/19",
				Network:     vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networking.pods"),
			}))))
		})

		It("should forbid a worker subnet that belongs to a different VPC", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.250.0.0/19",
				Network:     "https://www.googleapis.com/compute/v1/projects/test/global/networks/other-vpc",
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.subnetWorkers.name"),
			}))))
		})

		It("should return NotFound if the worker subnet does not exist", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(nil, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeNotFound),
				"Field": Equal("networks.subnetWorkers.name"),
			}))
		})

		It("should fail with InternalError if getting the worker subnet failed", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(nil, errors.New("boom"))

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInternal),
				"Field": Equal("networks.subnetWorkers"),
			}))))
		})

		It("should allow an IPV4_IPV6 worker subnet for an IPv4 cluster", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.250.0.0/19",
				StackType:   "IPV4_IPV6",
				Network:     vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid an IPV6_ONLY worker subnet for an IPv4 cluster", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.250.0.0/19",
				StackType:   "IPV6_ONLY",
				Network:     vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.subnetWorkers.stackType"),
			}))))
		})
	})

	Describe("#Validate BYO subnets for a dual-stack cluster", func() {
		const (
			workerSubnetName   = "byo-workers"
			servicesSubnetName = "byo-services"
			vpcName            = "test-vpc"
			vpcSelfLink        = "https://www.googleapis.com/compute/v1/projects/test/global/networks/test-vpc"
		)

		byoConfig := func() *apisgcp.InfrastructureConfig {
			return &apisgcp.InfrastructureConfig{
				Networks: apisgcp.NetworkConfig{
					VPC:            &apisgcp.VPC{Name: vpcName},
					SubnetWorkers:  &apisgcp.SubnetReference{Name: workerSubnetName},
					SubnetServices: &apisgcp.SubnetReference{Name: servicesSubnetName},
				},
			}
		}

		BeforeEach(func() {
			infra.Spec.ProviderConfig.Raw = encode(byoConfig())
			gcpClientFactory.EXPECT().Compute(ctx, gomock.Any(), infra.Spec.SecretRef).Return(gcpComputeClient, nil)
			gcpComputeClient.EXPECT().GetNetwork(ctx, vpcName).Return(&compute.Network{Name: vpcName, SelfLink: vpcSelfLink}, nil).AnyTimes()
			Expect(c.Create(ctx, newCluster(namespace, "10.250.0.0/24", "100.96.0.0/11", "100.64.0.0/13",
				gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6))).To(Succeed())
		})

		It("should succeed when both subnets are IPV4_IPV6", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange:        "10.250.0.0/19",
				StackType:          "IPV4_IPV6",
				ExternalIpv6Prefix: "2600:1900:4000:1::/64",
				Network:            vpcSelfLink,
			}, nil)
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, servicesSubnetName).Return(&compute.Subnetwork{
				IpCidrRange:        "10.251.0.0/19",
				StackType:          "IPV4_IPV6",
				ExternalIpv6Prefix: "2600:1900:4000:2::/64",
				Network:            vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid an IPV4_ONLY worker subnet for a dual-stack cluster", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.250.0.0/19",
				StackType:   "IPV4_ONLY",
				Network:     vpcSelfLink,
			}, nil)
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, servicesSubnetName).Return(&compute.Subnetwork{
				IpCidrRange:        "10.251.0.0/19",
				StackType:          "IPV4_IPV6",
				ExternalIpv6Prefix: "2600:1900:4000:2::/64",
				Network:            vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.subnetWorkers.stackType"),
			}))))
		})

		It("should forbid an IPV4_ONLY services subnet for a dual-stack cluster", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange:        "10.250.0.0/19",
				StackType:          "IPV4_IPV6",
				ExternalIpv6Prefix: "2600:1900:4000:1::/64",
				Network:            vpcSelfLink,
			}, nil)
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, servicesSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.251.0.0/19",
				StackType:   "IPV4_ONLY",
				Network:     vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.subnetServices.stackType"),
			}))))
		})

		It("should forbid a worker subnet without an external IPv6 prefix for a dual-stack cluster", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.250.0.0/19",
				StackType:   "IPV4_IPV6",
				Network:     vpcSelfLink,
			}, nil)
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, servicesSubnetName).Return(&compute.Subnetwork{
				IpCidrRange:        "10.251.0.0/19",
				StackType:          "IPV4_IPV6",
				ExternalIpv6Prefix: "2600:1900:4000:2::/64",
				Network:            vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.subnetWorkers.ipv6AccessType"),
			}))))
		})

		It("should forbid a services subnet without an external IPv6 prefix for a dual-stack cluster", func() {
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, workerSubnetName).Return(&compute.Subnetwork{
				IpCidrRange:        "10.250.0.0/19",
				StackType:          "IPV4_IPV6",
				ExternalIpv6Prefix: "2600:1900:4000:1::/64",
				Network:            vpcSelfLink,
			}, nil)
			gcpComputeClient.EXPECT().GetSubnet(ctx, region, servicesSubnetName).Return(&compute.Subnetwork{
				IpCidrRange: "10.251.0.0/19",
				StackType:   "IPV4_IPV6",
				Network:     vpcSelfLink,
			}, nil)

			errorList := cv.Validate(ctx, infra)
			Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.subnetServices.ipv6AccessType"),
			}))))
		})
	})
})

func newCluster(namespace, nodes, pods, services string, ipFamilies ...gardencorev1beta1.IPFamily) *extensionsv1alpha1.Cluster {
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core.gardener.cloud/v1beta1",
			Kind:       "Shoot",
		},
		Spec: gardencorev1beta1.ShootSpec{
			Networking: &gardencorev1beta1.Networking{
				Nodes:      ptr.To(nodes),
				Pods:       ptr.To(pods),
				Services:   ptr.To(services),
				IPFamilies: ipFamilies,
			},
		},
	}
	return &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
			Seed:         &runtime.RawExtension{Raw: []byte("{}")},
			Shoot:        runtime.RawExtension{Raw: encode(shoot)},
		},
	}
}

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
