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

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

// NormalizeCapabilityDefinitions ensures that capability definitions always include at least
// the architecture capability. This allows all downstream code to assume capabilities are always present,
// eliminating the need for conditional logic based on whether capabilities are defined.
func NormalizeCapabilityDefinitions(capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []gardencorev1beta1.CapabilityDefinition {
	if len(capabilityDefinitions) > 0 {
		return capabilityDefinitions
	}
	return []gardencorev1beta1.CapabilityDefinition{{
		Name:   v1beta1constants.ArchitectureName,
		Values: []string{v1beta1constants.ArchitectureAMD64, v1beta1constants.ArchitectureARM64},
	}}
}

// NormalizeMachineTypeCapabilities ensures that machine type capabilities include the architecture
// capability. This transforms the legacy architecture-based selection into capability-based selection.
// The architecture is determined in the following priority order:
// 1. If capabilities already has architecture, use it as-is
// 2. If capabilityDefinitions has exactly one architecture value, use that value
// 3. Otherwise, use workerArchitecture (defaulting to amd64)
func NormalizeMachineTypeCapabilities(capabilities gardencorev1beta1.Capabilities, workerArchitecture *string, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) gardencorev1beta1.Capabilities {
	if capabilities == nil {
		capabilities = make(gardencorev1beta1.Capabilities)
	}
	// If architecture capability is already present, return as-is
	if _, hasArch := capabilities[v1beta1constants.ArchitectureName]; hasArch {
		return capabilities
	}

	// Check if capabilityDefinitions has exactly one architecture value
	for _, def := range capabilityDefinitions {
		if def.Name == v1beta1constants.ArchitectureName && len(def.Values) == 1 {
			capabilities[v1beta1constants.ArchitectureName] = []string{def.Values[0]}
			return capabilities
		}
	}

	// Fall back to workerArchitecture or default
	arch := ptr.Deref(workerArchitecture, v1beta1constants.ArchitectureAMD64)
	capabilities[v1beta1constants.ArchitectureName] = []string{arch}
	return capabilities
}

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

// FindImageInCloudProfile takes a list of machine images and tries to find the first entry whose name, version and capabilities
// matches with the machineTypeCapabilities. If no such entry is found then an error will be returned.
// Note: capabilityDefinitions and machineTypeCapabilities are expected to be normalized
// by the caller using NormalizeCapabilityDefinitions() and NormalizeMachineTypeCapabilities()
func FindImageInCloudProfile(
	cloudProfileConfig *apisgcp.CloudProfileConfig,
	imageName, imageVersion string,
	workerArchitecture *string,
	machineTypeCapabilities gardencorev1beta1.Capabilities,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
) (*apisgcp.MachineImageFlavor, error) {
	if cloudProfileConfig == nil {
		return nil, fmt.Errorf("cloud profile config is nil")
	}
	machineImages := cloudProfileConfig.MachineImages

	for _, machineImage := range machineImages {
		if machineImage.Name != imageName {
			continue
		}

		// Collect all versions with matching version string (mixed format support)
		var matchingVersions []apisgcp.MachineImageVersion
		for _, version := range machineImage.Versions {
			if imageVersion == version.Version {
				matchingVersions = append(matchingVersions, version)
			}
		}

		if len(matchingVersions) == 0 {
			continue
		}

		// Convert old format (image with architecture) versions to capability flavors if required
		// as there may be multiple version entries for the same version with different architectures
		// the normalization for capability flavors is done here instead of the caller to keep the caller code simpler
		capabilityFlavors := convertLegacyVersionsToCapabilityFlavors(matchingVersions)

		if len(capabilityFlavors) > 0 {
			bestMatch, err := worker.FindBestImageFlavor(capabilityFlavors, machineTypeCapabilities, capabilityDefinitions)
			if err != nil {
				return nil, fmt.Errorf("could not determine best flavor %w", err)
			}
			return &bestMatch, nil
		}
	}
	return nil, fmt.Errorf("could not find an image for name %q and architecture %q in version %q", imageName, *workerArchitecture, imageVersion)
}

// convertLegacyVersionsToCapabilityFlavors converts old format (image with architecture) versions
// to capability flavors for mixed format support.
func convertLegacyVersionsToCapabilityFlavors(versions []apisgcp.MachineImageVersion) []apisgcp.MachineImageFlavor {
	var capabilityFlavors []apisgcp.MachineImageFlavor
	for _, version := range versions {
		if version.Image != "" && len(version.CapabilityFlavors) == 0 {
			arch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)
			capabilityFlavors = append(capabilityFlavors, apisgcp.MachineImageFlavor{
				Image: version.Image,
				Capabilities: gardencorev1beta1.Capabilities{
					v1beta1constants.ArchitectureName: []string{arch},
				},
			})
		} else {
			capabilityFlavors = append(capabilityFlavors, version.CapabilityFlavors...)
		}
	}
	return capabilityFlavors
}

// GroupVersionsByVersionString groups all provider versions by their version string.
// This is needed because the old format may have multiple entries for the same version
// with different architectures (mixed format support).
func GroupVersionsByVersionString(versions []apisgcp.MachineImageVersion) map[string][]apisgcp.MachineImageVersion {
	result := make(map[string][]apisgcp.MachineImageVersion)
	for _, v := range versions {
		result[v.Version] = append(result[v.Version], v)
	}
	return result
}

// GroupV1alpha1VersionsByVersionString groups all v1alpha1 provider versions by their version string.
// This is needed because the old format may have multiple entries for the same version
// with different architectures (mixed format support).
func GroupV1alpha1VersionsByVersionString(versions []v1alpha1.MachineImageVersion) map[string][]v1alpha1.MachineImageVersion {
	result := make(map[string][]v1alpha1.MachineImageVersion)
	for _, v := range versions {
		result[v.Version] = append(result[v.Version], v)
	}
	return result
}

// FindImageInWorkerStatus takes a list of machine images from the worker status and tries to find the first entry
// whose name, version and capabilities matches with the given ones. If no such entry is
// found then an error will be returned.
// Note: capabilityDefinitions and machineCapabilities are expected to be normalized by the caller.
func FindImageInWorkerStatus(machineImages []apisgcp.MachineImage, name string, version string, machineCapabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) (*apisgcp.MachineImage, error) {
	for _, statusMachineImage := range machineImages {
		if statusMachineImage.Name != name || statusMachineImage.Version != version {
			continue
		}

		// Normalize status image capabilities: if the status has Architecture but no Capabilities,
		// convert Architecture to Capabilities for compatibility checking
		statusCapabilities := statusMachineImage.Capabilities
		if len(statusCapabilities) == 0 && statusMachineImage.Architecture != nil {
			statusCapabilities = gardencorev1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{*statusMachineImage.Architecture},
			}
		}

		if gardencorev1beta1helper.AreCapabilitiesCompatible(statusCapabilities, machineCapabilities, capabilityDefinitions) {
			return &statusMachineImage, nil
		}
	}
	return nil, fmt.Errorf("no machine image found for image %q with version %q and capabilities %v", name, version, machineCapabilities)
}
