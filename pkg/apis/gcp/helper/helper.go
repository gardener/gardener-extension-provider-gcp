// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// FindSubnetByPurpose takes a list of subnets and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindSubnetByPurpose(subnets []apisgcp.Subnet, purpose apisgcp.SubnetPurpose) (*apisgcp.Subnet, error) {
	for _, subnet := range subnets {
		if subnet.Purpose == purpose {
			return &subnet, nil
		}
	}
	return nil, fmt.Errorf("cannot find subnet with purpose %q", purpose)
}

// FindMachineImage takes a list of machine images and tries to find the first entry
// whose name, version, architecture and zone matches with the given name, version, and zone. If no such entry is
// found then an error will be returned.
func FindMachineImage(machineImages []apisgcp.MachineImage, name, version string, architecture *string) (*apisgcp.MachineImage, error) {
	for _, machineImage := range machineImages {
		if machineImage.Architecture == nil {
			machineImage.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
		}
		if machineImage.Name == name && machineImage.Version == version && ptr.Equal(architecture, machineImage.Architecture) {
			return &machineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image found with name %q, architecture %q and version %q", name, *architecture, version)
}

// FindImageFromCloudProfile takes a list of machine images, and the desired image name and version. It tries
// to find the image with the given name, architecture and version in the desired cloud profile. If it cannot be found then an error
// is returned.
func FindImageFromCloudProfile(cloudProfileConfig *apisgcp.CloudProfileConfig, imageName, imageVersion string, architecture *string) (string, error) {
	if cloudProfileConfig != nil {
		for _, machineImage := range cloudProfileConfig.MachineImages {
			if machineImage.Name != imageName {
				continue
			}
			for _, version := range machineImage.Versions {
				if imageVersion == version.Version && ptr.Equal(architecture, version.Architecture) {
					return version.Image, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not find an image for name %q and architecture %q in version %q", imageName, *architecture, imageVersion)
}
