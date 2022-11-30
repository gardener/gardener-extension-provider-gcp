// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper_test

import (
	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const profileImage = "project/path/to/profile/image"

var _ = Describe("Helper", func() {
	var (
		purpose      api.SubnetPurpose = "foo"
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

	DescribeTable("#FindMachineImage",
		func(machineImages []api.MachineImage, name, version string, architecture *string, expectedMachineImage *api.MachineImage, expectErr bool) {
			machineImage, err := FindMachineImage(machineImages, name, version, architecture)
			expectResults(machineImage, expectedMachineImage, err, expectErr)
		},

		Entry("list is nil", nil, "foo", "1.2.3", pointer.String("foo"), nil, true),
		Entry("empty list", []api.MachineImage{}, "foo", "1.2.3", pointer.String("foo"), nil, true),
		Entry("entry not found (no name)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123"}}, "foo", "1.2.3", pointer.String("foo"), nil, true),
		Entry("entry not found (no version)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123"}}, "foo", "1.2.4", pointer.String("foo"), nil, true),
		Entry("entry not found (no architecture)", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: pointer.String("foo")}}, "foo", "1.2.4", pointer.String("foo"), nil, true),
		Entry("entry exists if architecture is nil", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123"}}, "bar", "1.2.3", pointer.String("amd64"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: pointer.String("amd64")}, false),
		Entry("entry exists", []api.MachineImage{{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: pointer.String("foo")}}, "bar", "1.2.3", pointer.String("foo"), &api.MachineImage{Name: "bar", Version: "1.2.3", Image: "image123", Architecture: pointer.String("foo")}, false),
	)

	DescribeTable("#FindImage",
		func(profileImages []api.MachineImages, imageName, version string, architecture *string, expectedImage string) {
			cfg := &api.CloudProfileConfig{}
			cfg.MachineImages = profileImages
			image, err := FindImageFromCloudProfile(cfg, imageName, version, architecture)

			Expect(image).To(Equal(expectedImage))
			if expectedImage != "" {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("list is nil", nil, "ubuntu", "1", pointer.String("foo"), ""),

		Entry("profile empty list", []api.MachineImages{}, "ubuntu", "1", pointer.String("foo"), ""),
		Entry("profile entry not found (image does not exist)", makeProfileMachineImages("debian", "1", pointer.String("foo")), "ubuntu", "1", pointer.String("foo"), ""),
		Entry("profile entry not found (version does not exist)", makeProfileMachineImages("ubuntu", "2", pointer.String("foo")), "ubuntu", "1", pointer.String("foo"), ""),
		Entry("profile entry not found (no architecture)", makeProfileMachineImages("ubuntu", "2", pointer.String("bar")), "ubuntu", "1", pointer.String("foo"), ""),
		Entry("profile entry", makeProfileMachineImages("ubuntu", "1", pointer.String("foo")), "ubuntu", "1", pointer.String("foo"), profileImage),
	)
})

func makeProfileMachineImages(name, version string, architecture *string) []api.MachineImages {
	var versions []api.MachineImageVersion
	versions = append(versions, api.MachineImageVersion{
		Version:      version,
		Image:        profileImage,
		Architecture: architecture,
	})

	return []api.MachineImages{
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
