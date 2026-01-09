// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

const profileImage = "project/path/to/profile/image"

var _ = Describe("Helper", func() {
	var (
		purpose      apisgcp.SubnetPurpose = "foo"
		purposeWrong apisgcp.SubnetPurpose = "baz"
	)

	DescribeTable("#FindSubnetByPurpose",
		func(subnets []apisgcp.Subnet, purpose apisgcp.SubnetPurpose, expectedSubnet *apisgcp.Subnet, expectErr bool) {
			subnet, err := FindSubnetByPurpose(subnets, purpose)
			expectResults(subnet, expectedSubnet, err, expectErr)
		},

		Entry("list is nil", nil, purpose, nil, true),
		Entry("empty list", []apisgcp.Subnet{}, purpose, nil, true),
		Entry("entry not found", []apisgcp.Subnet{{Name: "bar", Purpose: purposeWrong}}, purpose, nil, true),
		Entry("entry exists", []apisgcp.Subnet{{Name: "bar", Purpose: purpose}}, purpose, &apisgcp.Subnet{Name: "bar", Purpose: purpose}, false),
	)

	DescribeTable("#FindMachineImage",
		func(machineImages []apisgcp.MachineImage, name, version string, architecture *string, expectedMachineImage *apisgcp.MachineImage, expectErr bool) {
			machineImage, err := FindMachineImage(machineImages, name, version, architecture)
			expectResults(machineImage, expectedMachineImage, err, expectErr)
		},

		Entry("list is nil", nil, "foo", "1.2.3", ptr.To("foo"), nil, true),
		Entry("empty list", []apisgcp.MachineImage{}, "foo", "1.2.3", ptr.To("foo"), nil, true),
		Entry("entry not found (no name)", []apisgcp.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123"}}, "foo", "1.2.3", ptr.To("foo"), nil, true),
		Entry("entry not found (no version)", []apisgcp.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123"}}, "foo", "1.2.4", ptr.To("foo"), nil, true),
		Entry("entry not found (no architecture)", []apisgcp.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: ptr.To("foo")}}, "foo", "1.2.4", ptr.To("foo"), nil, true),
		Entry("entry exists if architecture is nil", []apisgcp.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123"}}, "bar", "1.2.3", ptr.To("amd64"), &apisgcp.MachineImage{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: ptr.To("amd64")}, false),
		Entry("entry exists", []apisgcp.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: ptr.To("foo")}}, "bar", "1.2.3", ptr.To("foo"), &apisgcp.MachineImage{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: ptr.To("foo")}, false),
	)

	DescribeTable("#FindImage",
		func(profileImages []apisgcp.MachineImages, imageName, version string, architecture *string, expectedImage string) {
			cfg := &apisgcp.CloudProfileConfig{}
			cfg.MachineImages = profileImages
			image, err := FindImageFromCloudProfile(cfg, imageName, version, architecture)

			Expect(image).To(Equal(expectedImage))
			if expectedImage != "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("list is nil", nil, "ubuntu", "1", ptr.To("foo"), ""),

		Entry("profile empty list", []apisgcp.MachineImages{}, "ubuntu", "1", ptr.To("foo"), ""),
		Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", ptr.To("foo")), "ubuntu", "1", ptr.To("foo"), ""),
		Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", ptr.To("foo")), "ubuntu", "1", ptr.To("foo"), ""),
		Entry("profile entry not found (no architecture)", makeProfileMachineImages("ubuntu", "2", ptr.To("bar")), "ubuntu", "1", ptr.To("foo"), ""),
		Entry("profile entry", makeProfileMachineImages("ubuntu", "1", ptr.To("foo")), "ubuntu", "1", ptr.To("foo"), profileImage),
	)
})

func makeProfileMachineImages(name, version string, architecture *string) []apisgcp.MachineImages {
	var versions []apisgcp.MachineImageVersion
	versions = append(versions, apisgcp.MachineImageVersion{
		Version:      version,
		Image:        profileImage,
		Architecture: architecture,
	})

	return []apisgcp.MachineImages{
		{
			Name:     name,
			Versions: versions,
		},
	}
}

func expectResults(result, expected interface{}, err error, expectErr bool) {
	if !expectErr {
		Expect(result).To(Equal(expected))
		Expect(err).NotTo(HaveOccurred())
	} else {
		Expect(result).To(BeNil())
		Expect(err).To(HaveOccurred())
	}
}
