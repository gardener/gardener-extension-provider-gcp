package bastion

import (
	"fmt"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"

	gcpapi "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

var _ = Describe("Bastion Options", func() {
	var (
		cluster *extensions.Cluster
		opt     Options
		bastion *extensionsv1alpha1.Bastion
	)

	BeforeEach(func() {
		cluster = createGCPTestCluster()
		opt = createTestOptions(opt)
		bastion = createTestBastion()
	})

	Context("DetermineOptions", func() {
		It("should return options", func() {
			cluster := createGCPTestCluster()
			bastion := createTestBastion()
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

	Context("check Names generations", func() {
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

	Context("check getZone", func() {
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

	Context("unMarshalProviderStatus", func() {
		It("should update a ProviderStatusRaw Object from a Byte array", func() {
			testInput := []byte(`{"zone":"us-west1-a"}`)
			res, err := unmarshalProviderStatus(testInput)
			expectedMarshalOutput := "us-west1-a"

			Expect(err).To(Not(HaveOccurred()))
			Expect(res.Zone).To(Equal(expectedMarshalOutput))
		})
	})

	Context("check getNetworkName", func() {
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

	Context("getProviderSpecificImage", func() {
		var (
			desiredVM      VmDetails
			providerImages []v1alpha1.MachineImages
		)

		BeforeEach(func() {
			desiredVM = VmDetails{
				MachineName:   "small_machine",
				Architecture:  "amd64",
				ImageBaseName: "gardenlinux",
				ImageVersion:  "1.2.3",
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
