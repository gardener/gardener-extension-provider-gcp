// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

// FindSubnetByPurpose takes a list of subnets and tries to find the first entry
// whose purpose matches with the given purpose. If no such entry is found then an error will be
// returned.
func FindSubnetByPurpose(subnets []api.Subnet, purpose api.SubnetPurpose) (*api.Subnet, error) {
	for _, subnet := range subnets {
		if subnet.Purpose == purpose {
			return &subnet, nil
		}
	}
	return nil, fmt.Errorf("cannot find subnet with purpose %q", purpose)
}

// FindImageFromCloudProfile takes a list of machine images, and the desired image name and version. It tries
// to find the image with the given name, architecture and version in the desired cloud profile. If it cannot be found then an error
// is returned.
func FindImageFromCloudProfile(cloudProfileConfig *api.CloudProfileConfig, imageName, imageVersion string, architecture *string) (string, error) {
	if cloudProfileConfig != nil {
		for _, machineImage := range cloudProfileConfig.MachineImages {
			if machineImage.Name != imageName {
				continue
			}
			for _, version := range machineImage.Versions {
				if version.Architecture == nil {
					version.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
				}
				if imageVersion == version.Version && ptr.Equal(architecture, version.Architecture) {
					return version.Image, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not find an image for name %q and architecture %q in version %q", imageName, *architecture, imageVersion)
}

// FindImageInCloudProfile takes a list of machine images and tries to find the first entry
// whose name, version, region, architecture, capabilities and zone matches with the given ones. If no such entry is
// found then an error will be returned.
func FindImageInCloudProfile(
	cloudProfileConfig *api.CloudProfileConfig,
	imageName, imageVersion string,
	architecture *string,
	machineTypeCapabilities gardencorev1beta1.Capabilities,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
) (*api.MachineImageFlavor, error) {
	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil")
	}
	machineImages := cloudProfileConfig.MachineImages

	capabilitySet, err := findMachineImageFlavor(machineImages, imageName, imageVersion, architecture, machineTypeCapabilities, capabilityDefinitions)
	if err != nil {
		return nil, fmt.Errorf("could not find an image for name %q and architecture %q in version %q: %w", imageName, *architecture, imageVersion, err)
	}

	return capabilitySet, nil
}

func findMachineImageFlavor(
	machineImages []api.MachineImages,
	imageName, imageVersion string,
	architecture *string,
	machineTypeCapabilities gardencorev1beta1.Capabilities,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
) (*api.MachineImageFlavor, error) {
	for _, machineImage := range machineImages {
		if machineImage.Name != imageName {
			continue
		}
		for _, version := range machineImage.Versions {
			if imageVersion != version.Version {
				continue
			}

			if len(capabilityDefinitions) == 0 {
				if ptr.Equal(architecture, version.Architecture) {
					return &api.MachineImageFlavor{
						Image:        version.Image,
						Capabilities: gardencorev1beta1.Capabilities{},
					}, nil
				}

				continue
			}

			bestMatch, err := worker.FindBestImageFlavor(version.CapabilityFlavors, machineTypeCapabilities, capabilityDefinitions)
			if err != nil {
				return nil, fmt.Errorf("could not determine best flavor %w", err)
			}

			return &bestMatch, nil
		}
	}
	return nil, fmt.Errorf("version not found")
}

// FindImageInWorkerStatus takes a list of machine images from the worker status and tries to find the first entry
// whose name, version, architecture, capabilities and zone matches with the given ones. If no such entry is
// found then an error will be returned.
func FindImageInWorkerStatus(machineImages []api.MachineImage, name string, version string, architecture *string, machineCapabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) (*api.MachineImage, error) {
	// If no capabilityDefinitions are specified, return the (legacy) architecture format field as no Capabilities are used.
	if len(capabilityDefinitions) == 0 {
		for _, statusMachineImage := range machineImages {
			if statusMachineImage.Architecture == nil {
				statusMachineImage.Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
			}
			if statusMachineImage.Name == name && statusMachineImage.Version == version && ptr.Equal(architecture, statusMachineImage.Architecture) {
				return &statusMachineImage, nil
			}
		}
		return nil, fmt.Errorf("no machine image found for image %q with version %q and architecture %q", name, version, *architecture)
	}

	// If capabilityDefinitions are specified, we need to find the best matching capability set.
	for _, statusMachineImage := range machineImages {
		var statusMachineImageV1alpha1 v1alpha1.MachineImage
		if err := v1alpha1.Convert_gcp_MachineImage_To_v1alpha1_MachineImage(&statusMachineImage, &statusMachineImageV1alpha1, nil); err != nil {
			return nil, fmt.Errorf("failed to convert machine image: %w", err)
		}
		if statusMachineImage.Name == name && statusMachineImage.Version == version && gardencorev1beta1helper.AreCapabilitiesCompatible(statusMachineImageV1alpha1.Capabilities, machineCapabilities, capabilityDefinitions) {
			return &statusMachineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image found for image %q with version %q and capabilities %v", name, version, machineCapabilities)
}
