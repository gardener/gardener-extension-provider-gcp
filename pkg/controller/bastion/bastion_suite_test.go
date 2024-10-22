// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"encoding/json"
	"fmt"
	"testing"

	extensionsbastion "github.com/gardener/gardener/extensions/pkg/bastion"
	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"google.golang.org/api/compute/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	gcpapi "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

func TestBastion(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bastion Suite")
}

var _ = Describe("Bastion", func() {
	var (
		cluster *extensions.Cluster
		bastion *extensionsv1alpha1.Bastion

		opt  Options
		ctrl *gomock.Controller
	)
	BeforeEach(func() {
		cluster = createGCPTestCluster()
		bastion = createTestBastion()
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("getWorkersCIDR", func() {
		It("getWorkersCIDR", func() {
			cidr, err := getWorkersCIDR(createGCPTestCluster())
			Expect(err).To(Not(HaveOccurred()))
			Expect(cidr).To(Equal("10.250.0.0/16"))
		})
	})

	Describe("Determine options", func() {
		It("should return options", func() {
			options, err := DetermineOptions(bastion, cluster, "projectID", "vNet", "subnet")
			Expect(err).To(Not(HaveOccurred()))

			Expect(options.BastionInstanceName).To(Equal("cluster1-bastionName1-bastion-1cdc8"))
			Expect(options.Zone).To(Equal("us-west1-a"))
			Expect(options.DiskName).To(Equal("cluster1-bastionName1-bastion-1cdc8-disk"))
			Expect(options.Subnetwork).To(Equal("regions/us-west/subnetworks/subnet"))
			Expect(options.ProjectID).To(Equal("projectID"))
			Expect(options.Network).To(Equal("projects/projectID/global/networks/vNet"))
			Expect(options.WorkersCIDR).To(Equal("10.250.0.0/16"))
		})
	})

	Describe("check Names generations", func() {
		It("should generate idempotent name", func() {
			expected := "clusterName-shortName-bastion-79641"

			res, err := generateBastionBaseResourceName("clusterName", "shortName")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal(expected))

			res, err = generateBastionBaseResourceName("clusterName", "shortName")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal(expected))
		})

		It("should generate a name not exceeding a certain length", func() {
			res, err := generateBastionBaseResourceName("clusterName", "LetsExceed63LenLimit012345678901234567890123456789012345678901234567890123456789")
			Expect(err).To(Not(HaveOccurred()))
			Expect(res).To(Equal("clusterName-LetsExceed63LenLimit0-bastion-139c4"))
		})

		It("should generate a unique name even if inputs values have minor deviations", func() {
			res, _ := generateBastionBaseResourceName("1", "1")
			res2, _ := generateBastionBaseResourceName("1", "2")
			Expect(res).ToNot(Equal(res2))
		})

		baseName, _ := generateBastionBaseResourceName("clusterName", "LetsExceed63LenLimit012345678901234567890123456789012345678901234567890123456789")
		DescribeTable("should generate names and fit maximum length",
			func(input string, expectedOut string) {
				Expect(len(input)).Should(BeNumerically("<", maxLengthForResource))
				Expect(input).Should(Equal(expectedOut))
			},
			Entry("disk resource name", DiskResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-disk"),
			Entry("firewall ingress ssh resource name", FirewallIngressAllowSSHResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-allow-ssh"),
			Entry("firewall egress allow resource name", FirewallEgressAllowOnlyResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-egress-worker"),
			Entry("firewall egress deny resource name", FirewallEgressDenyAllResourceName(baseName), "clusterName-LetsExceed63LenLimit0-bastion-139c4-deny-all"),
		)
	})

	Describe("check getZone", func() {
		var testProviderStatusRaw providerStatusRaw
		It("should return an empty string", func() {
			testProviderStatusRaw = providerStatusRaw{""}
			res := getZone(cluster, "us-west", &testProviderStatusRaw)
			Expect(res).To(BeEmpty())
		})

		It("should return a zone string", func() {
			testProviderStatusRaw = providerStatusRaw{"us-west1-a"}
			res := getZone(cluster, "us-west", &testProviderStatusRaw)
			Expect(res).To(Equal("us-west1-a"))
		})
	})

	Describe("check getNetworkName", func() {
		opt = createTestOptions(opt)
		It("should return network name vpc-123", func() {
			network := &gcpapi.NetworkConfig{
				VPC: &gcpapi.VPC{
					Name: "vpc-123",
				},
			}

			testCluster := createTestCluster(network)
			clusterName := "clustername-123"
			nameWork, err := getNetworkName(testCluster, opt.ProjectID, clusterName)
			Expect(err).To(Not(HaveOccurred()))
			Expect(nameWork).To(Equal(fmt.Sprintf("projects/%s/global/networks/%s", opt.ProjectID, "vpc-123")))
		})

		It("should return network name clustername-123", func() {
			network := &gcpapi.NetworkConfig{}
			testCluster := createTestCluster(network)
			clusterName := "clustername-123"
			nameWork, err := getNetworkName(testCluster, opt.ProjectID, clusterName)
			Expect(err).To(Not(HaveOccurred()))
			Expect(nameWork).To(Equal(fmt.Sprintf("projects/%s/global/networks/%s", opt.ProjectID, clusterName)))
		})
	})

	Describe("check unMarshalProviderStatus", func() {
		It("should update a ProviderStatusRaw Object from a Byte array", func() {
			testInput := []byte(`{"zone":"us-west1-a"}`)
			res, err := unmarshalProviderStatus(testInput)
			expectedMarshalOutput := "us-west1-a"

			Expect(err).To(Not(HaveOccurred()))
			Expect(res.Zone).To(Equal(expectedMarshalOutput))
		})
	})

	Describe("check Ingress Permissions", func() {
		It("Should return a string array with ipV4 normalized addresses", func() {
			bastion.Spec.Ingress = []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "213.69.151.253/24",
				}},
			}
			res, err := ingressPermissions(bastion)
			Expect(err).To(Not(HaveOccurred()))
			Expect(res[0]).To(Equal("213.69.151.0/24"))

		})
		It("Should throw an error with invalid CIDR entry", func() {
			bastion.Spec.Ingress = []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "1234",
				}},
			}
			res, err := ingressPermissions(bastion)
			Expect(err).To(HaveOccurred())
			Expect(res).To(BeEmpty())
		})
	})

	Describe("check getProviderStatus", func() {
		It("Should return an error and nil", func() {
			res, err := getProviderStatus(bastion)
			Expect(err).To(BeNil())
			Expect(res).To(BeNil())
		})
		It("Should return a providerStatusRaw struct", func() {
			bastion.Status.ProviderStatus = &runtime.RawExtension{Raw: []byte(`{"zone":"us-west1-a"}`)}
			res, err := getProviderStatus(bastion)
			Expect(err).To(Not(HaveOccurred()))
			Expect(res.Zone).To(Equal("us-west1-a"))
		})
	})

	Describe("check PatchCIDRs ", func() {
		It("should return equally", func() {
			cidrs := []string{"213.69.151.0/24"}
			value := &compute.Firewall{SourceRanges: cidrs}
			Expect(patchCIDRs(cidrs)).To(Equal(value))
		})
	})

	Describe("getProviderSpecificImage", func() {
		var (
			desiredVM      extensionsbastion.MachineSpec
			providerImages []v1alpha1.MachineImages
		)

		BeforeEach(func() {
			desiredVM = extensionsbastion.MachineSpec{
				MachineTypeName: "small_machine",
				Architecture:    "amd64",
				ImageBaseName:   "gardenlinux",
				ImageVersion:    "1.2.3",
			}
			providerImages = createTestProviderConfig().MachineImages
		})

		It("succeed for existing image", func() {
			machineImage, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).ToNot(HaveOccurred())
			Expect(machineImage.Image).To(DeepEqual(providerImages[0].Versions[0].Image))
			Expect(machineImage.Version).To(DeepEqual(providerImages[0].Versions[0].Version))
			Expect(machineImage.Architecture).To(DeepEqual(providerImages[0].Versions[0].Architecture))
		})

		It("fail if image name does not exist", func() {
			desiredVM.ImageBaseName = "unknown"
			_, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).To(HaveOccurred())
		})

		It("fail if image version does not exist", func() {
			desiredVM.ImageVersion = "6.6.6"
			_, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).To(HaveOccurred())
		})

		It("fail if no image for given architecture exists", func() {
			desiredVM.Architecture = "x86"
			_, err := getProviderSpecificImage(providerImages, desiredVM)
			Expect(err).To(HaveOccurred())
		})
	})
})

func createShootTestStruct() *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Region: "us-west",
			Provider: gardencorev1beta1.Provider{
				InfrastructureConfig: &runtime.RawExtension{Raw: mustEncode(gcpapi.InfrastructureConfig{
					Networks: gcpapi.NetworkConfig{
						Workers: "10.250.0.0/16",
					},
				})},
			},
		},
	}
}

func createTestMachineImages() []gardencorev1beta1.MachineImage {
	return []gardencorev1beta1.MachineImage{{
		Name: "gardenlinux",
		Versions: []gardencorev1beta1.MachineImageVersion{{
			ExpirableVersion: gardencorev1beta1.ExpirableVersion{
				Version:        "1.2.3",
				Classification: ptr.To(gardencorev1beta1.ClassificationSupported),
			},
			Architectures: []string{"amd64"},
		}},
	}}
}

func createTestMachineTypes() []gardencorev1beta1.MachineType {
	return []gardencorev1beta1.MachineType{{
		CPU:          resource.MustParse("4"),
		Name:         "machineName",
		Architecture: ptr.To("amd64"),
	}}
}

func createTestProviderConfig() *v1alpha1.CloudProfileConfig {
	return &v1alpha1.CloudProfileConfig{MachineImages: []v1alpha1.MachineImages{{
		Name: "gardenlinux",
		Versions: []v1alpha1.MachineImageVersion{{
			Version:      "1.2.3",
			Image:        "/path/to/images",
			Architecture: ptr.To("amd64"),
		}},
	}}}
}

func createGCPTestCluster() *extensions.Cluster {
	return &controller.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster1"},
		Shoot:      createShootTestStruct(),
		CloudProfile: &gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				Regions: []gardencorev1beta1.Region{
					{Name: "regionName"},
					{Name: "us-west", Zones: []gardencorev1beta1.AvailabilityZone{
						{Name: "us-west1-a"},
						{Name: "us-west1-b"},
					}},
				},
				MachineImages: createTestMachineImages(),
				MachineTypes:  createTestMachineTypes(),
				ProviderConfig: &runtime.RawExtension{
					Raw: mustEncode(createTestProviderConfig()),
				},
			},
		},
	}
}

func createTestBastion() *extensionsv1alpha1.Bastion {
	return &extensionsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bastionName1",
		},
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{},
			UserData:    nil,
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: "213.69.151.0/24",
				}},
			},
		},
	}
}

func createTestCluster(networkConfig *gcpapi.NetworkConfig) *extensions.Cluster {
	bytes, _ := json.Marshal(&gcpapi.InfrastructureConfig{
		Networks: *networkConfig,
	})
	return &extensions.Cluster{
		Shoot: &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					InfrastructureConfig: &runtime.RawExtension{Raw: bytes},
				},
			},
		},
	}
}

func createTestOptions(opt Options) Options {
	opt.ProjectID = "test-project"
	opt.Zone = "us-west1-a"
	opt.BastionInstanceName = "test-bastion1"
	return opt
}

func getNetworkName(cluster *extensions.Cluster, projectID string, clusterName string) (string, error) {
	infrastructureConfig := &gcpapi.InfrastructureConfig{}
	err := json.Unmarshal(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, infrastructureConfig)
	if err != nil {
		return "", err
	}

	networkName := fmt.Sprintf("projects/%s/global/networks/%s", projectID, clusterName)

	if infrastructureConfig.Networks.VPC != nil {
		networkName = fmt.Sprintf("projects/%s/global/networks/%s", projectID, infrastructureConfig.Networks.VPC.Name)
	}

	return networkName, nil
}

func mustEncode(object any) []byte {
	data, err := json.Marshal(object)
	Expect(err).ToNot(HaveOccurred())
	return data
}
