// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

const profileImage = "project/path/to/profile/image"

var _ = Describe("Helper", func() {
	var (
		purpose      api.SubnetPurpose = "amd64"
		purposeWrong api.SubnetPurpose = "baz"
	)

	DescribeTable("#FindSubnetByPurpose",
		func(subnets []api.Subnet, purpose api.SubnetPurpose, expectedSubnet *api.Subnet, expectErr bool) {
			subnet, err := FindSubnetByPurpose(subnets, purpose)
			expectResults(subnet, expectedSubnet, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, true),
		Entry("empty list", []api.Subnet{}, purpose, nil, true),
		Entry("entry not found", []api.Subnet{{Name: "bar", Purpose: purposeWrong}}, purpose, nil, true),
		Entry("entry exists", []api.Subnet{{Name: "bar", Purpose: purpose}}, purpose, &api.Subnet{Name: "bar", Purpose: purpose}, false),
	)

	DescribeTableSubtree("Select Worker Images", func(hasCapabilities bool) {
		var capabilityDefinitions []v1beta1.CapabilityDefinition
		var machineTypeCapabilities v1beta1.Capabilities
		var imageCapabilities v1beta1.Capabilities

		if hasCapabilities {
			capabilityDefinitions = []v1beta1.CapabilityDefinition{
				{Name: "architecture", Values: []string{"amd64", "arm64"}},
				{Name: "capability1", Values: []string{"value1", "value2", "value3"}},
			}
			machineTypeCapabilities = v1beta1.Capabilities{
				"architecture": []string{"amd64"},
				"capability1":  []string{"value2"},
			}
			imageCapabilities = v1beta1.Capabilities{
				"architecture": []string{"amd64"},
				"capability1":  []string{"value2"},
			}
		}

		DescribeTable("#FindImageInWorkerStatus",
			func(machineImages []api.MachineImage, name, version string, architecture *string, expectedMachineImage *api.MachineImage, expectErr bool) {
				if hasCapabilities {
					machineTypeCapabilities["architecture"] = []string{*architecture}
					if expectedMachineImage != nil {
						expectedMachineImage.Capabilities = imageCapabilities
						expectedMachineImage.Architecture = nil
					}
				}

				machineImage, err := FindImageInWorkerStatus(machineImages, name, version, architecture, machineTypeCapabilities, capabilityDefinitions)
				expectResults(machineImage, expectedMachineImage, err, expectErr)
			},

			Entry("list is nil", nil, "amd64", "1.2.3", ptr.To("amd64"), nil, true),
			Entry("empty list", []api.MachineImage{}, "amd64", "1.2.3", ptr.To("amd64"), nil, true),

			Entry("entry not found (no name)", makeStatusMachineImages("bar", "1.2.3", "image123", ptr.To("amd64"), imageCapabilities), "amd64", "1.2.3", ptr.To("amd64"), nil, true),
			Entry("entry not found (no version)", makeStatusMachineImages("bar", "1.2.3", "image123", ptr.To("amd64"), imageCapabilities), "amd64", "1.2.4", ptr.To("amd64"), nil, true),
			Entry("entry not found (no architecture)", makeStatusMachineImages("bar", "1.2.3", "image123", ptr.To("arm64"), imageCapabilities), "amd64", "1.2.3", ptr.To("amd64"), nil, true),
			Entry("entry exists if architecture is nil", makeStatusMachineImages("bar", "1.2.3", "image123", nil, imageCapabilities), "bar", "1.2.3", ptr.To("amd64"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: ptr.To("amd64")}, false),
			Entry("entry exists", makeStatusMachineImages("bar", "1.2.3", "image123", ptr.To("amd64"), imageCapabilities), "bar", "1.2.3", ptr.To("amd64"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: ptr.To("amd64")}, false),
		)

		DescribeTable("#FindImageInCloudProfile",
			func(profileImages []api.MachineImages, imageName, version string, architecture *string, expectedImage string) {
				if hasCapabilities {
					machineTypeCapabilities["architecture"] = []string{*architecture}
				}

				cfg := &api.CloudProfileConfig{}
				cfg.MachineImages = profileImages

				image, err := FindImageInCloudProfile(cfg, imageName, version, architecture, machineTypeCapabilities, capabilityDefinitions)

				if expectedImage != "" {
					Expect(err).NotTo(HaveOccurred())
					Expect(image.Image).To(Equal(expectedImage))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},

			Entry("list is nil", nil, "ubuntu", "1", ptr.To("amd64"), ""),

			Entry("profile empty list", []api.MachineImages{}, "ubuntu", "1", ptr.To("amd64"), ""),
			Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", ptr.To("amd64"), ""),
			Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", ptr.To("amd64"), ""),
			Entry("profile entry not found (no architecture)", makeProfileMachineImages("ubuntu", "2", ptr.To("bar"), imageCapabilities), "ubuntu", "1", ptr.To("amd64"), ""),
			Entry("profile entry exists", makeProfileMachineImages("ubuntu", "1", ptr.To("amd64"), imageCapabilities), "ubuntu", "1", ptr.To("amd64"), profileImage),
		)
	},
		Entry("with capabilities", true),
		Entry("without capabilities", false),
	)
})

//nolint:unparam
func makeStatusMachineImages(name, version, image string, arch *string, capabilities v1beta1.Capabilities) []api.MachineImage {
	if capabilities != nil {
		capabilities["architecture"] = []string{ptr.Deref(arch, "")}
		return []api.MachineImage{
			{
				Name:         name,
				Version:      version,
				Image:        image,
				Capabilities: capabilities,
			},
		}
	}
	return []api.MachineImage{
		{
			Name:         name,
			Version:      version,
			Image:        image,
			Architecture: arch,
		},
	}
}
func makeProfileMachineImages(name, version string, architecture *string, capabilities v1beta1.Capabilities) []api.MachineImages {
	GinkgoT().Helper()
	versions := []api.MachineImageVersion{{
		Version: version,
	}}

	if capabilities == nil {
		versions[0].Image = profileImage
		versions[0].Architecture = architecture
	} else {
		versions[0].CapabilityFlavors = []api.MachineImageFlavor{{
			Capabilities: capabilities,
			Image:        profileImage,
		}}
	}

	return []api.MachineImages{
		{
			Name:     name,
			Versions: versions,
		},
	}
}

func expectResults(result, expected interface{}, err error, expectErr bool) {
	GinkgoT().Helper()
	if !expectErr {
		Expect(result).To(Equal(expected))
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(result).To(BeNil())
		Expect(err).To(HaveOccurred())
	}
}

var _ = Describe("Helper - Mixed Format", func() {
	var (
		capabilityDefinitions   []v1beta1.CapabilityDefinition
		machineTypeCapabilities v1beta1.Capabilities
	)

	BeforeEach(func() {
		capabilityDefinitions = []v1beta1.CapabilityDefinition{
			{Name: "architecture", Values: []string{"amd64", "arm64"}},
		}
		machineTypeCapabilities = v1beta1.Capabilities{
			"architecture": []string{"amd64"},
		}
	})

	Describe("#FindImageInCloudProfile - Mixed Format", func() {
		It("should find image using old format (image with architecture) when capabilities defined", func() {
			// Provider uses old format but CloudProfile has capabilities
			profileImages := []api.MachineImages{
				{
					Name: "ubuntu",
					Versions: []api.MachineImageVersion{
						{Version: "1.0", Image: "projects/my-project/global/images/ubuntu-amd64", Architecture: ptr.To("amd64")},
						{Version: "1.0", Image: "projects/my-project/global/images/ubuntu-arm64", Architecture: ptr.To("arm64")},
					},
				},
			}
			cfg := &api.CloudProfileConfig{MachineImages: profileImages}

			image, err := FindImageInCloudProfile(cfg, "ubuntu", "1.0", ptr.To("amd64"), machineTypeCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(image.Image).To(Equal("projects/my-project/global/images/ubuntu-amd64"))
		})

		It("should find image using new format (capabilityFlavors) when capabilities defined", func() {
			profileImages := []api.MachineImages{
				{
					Name: "ubuntu",
					Versions: []api.MachineImageVersion{
						{
							Version: "1.0",
							CapabilityFlavors: []api.MachineImageFlavor{
								{Image: "projects/my-project/global/images/ubuntu-amd64", Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}}},
								{Image: "projects/my-project/global/images/ubuntu-arm64", Capabilities: v1beta1.Capabilities{"architecture": []string{"arm64"}}},
							},
						},
					},
				},
			}
			cfg := &api.CloudProfileConfig{MachineImages: profileImages}

			image, err := FindImageInCloudProfile(cfg, "ubuntu", "1.0", ptr.To("amd64"), machineTypeCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(image.Image).To(Equal("projects/my-project/global/images/ubuntu-amd64"))
		})

		It("should prefer new format over old format when both exist", func() {
			// This shouldn't normally happen but tests priority
			profileImages := []api.MachineImages{
				{
					Name: "ubuntu",
					Versions: []api.MachineImageVersion{
						{
							Version: "1.0",
							CapabilityFlavors: []api.MachineImageFlavor{
								{Image: "new-format-image", Capabilities: v1beta1.Capabilities{"architecture": []string{"amd64"}}},
							},
						},
						{Version: "1.0", Image: "old-format-image", Architecture: ptr.To("amd64")},
					},
				},
			}
			cfg := &api.CloudProfileConfig{MachineImages: profileImages}

			image, err := FindImageInCloudProfile(cfg, "ubuntu", "1.0", ptr.To("amd64"), machineTypeCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(image.Image).To(Equal("new-format-image"))
		})

		It("should find arm64 image using old format with capabilities", func() {
			machineTypeCapabilities["architecture"] = []string{"arm64"}
			profileImages := []api.MachineImages{
				{
					Name: "ubuntu",
					Versions: []api.MachineImageVersion{
						{Version: "1.0", Image: "projects/my-project/global/images/ubuntu-amd64", Architecture: ptr.To("amd64")},
						{Version: "1.0", Image: "projects/my-project/global/images/ubuntu-arm64", Architecture: ptr.To("arm64")},
					},
				},
			}
			cfg := &api.CloudProfileConfig{MachineImages: profileImages}

			image, err := FindImageInCloudProfile(cfg, "ubuntu", "1.0", ptr.To("arm64"), machineTypeCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(image.Image).To(Equal("projects/my-project/global/images/ubuntu-arm64"))
		})
	})
})
