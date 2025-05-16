// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"maps"
	"slices"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *apisgcp.CloudProfileConfig, machineImages []core.MachineImage, capabilitiesDefinition core.Capabilities, specPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	cpConfigImagesContext := NewProviderImagesContext(cpConfig.MachineImages)

	// validate machine images
	for idxImage, machineImage := range machineImages {
		if len(machineImage.Versions) == 0 {
			continue
		}
		machineImagePath := specPath.Child("machineImages").Index(idxImage)
		// validate that for each machine image there is a corresponding cpConfig image
		providerVersion, existsInConfig := cpConfigImagesContext.GetImage(machineImage.Name)
		if !existsInConfig {
			allErrs = append(allErrs, field.Required(machineImagePath,
				fmt.Sprintf("must provide an image mapping for image %q in providerConfig", machineImage.Name)))
			continue
		}
		// validate that for each machine image version entry a mapped entry in cpConfig exists
		for idxVersion, version := range machineImage.Versions {
			machineImageVersionPath := machineImagePath.Child("versions").Index(idxVersion)

			if len(capabilitiesDefinition) == 0 {
				allErrs = validateArchitectureMapping(version, cpConfigImagesContext, machineImage, machineImageVersionPath, allErrs)
			} else {
				allErrs = validateCapabilitiesMapping(version, providerVersion, capabilitiesDefinition, machineImageVersionPath, allErrs)
			}
		}
	}

	// validate all cpConfig images fields
	for imgIdx, providerImage := range cpConfig.MachineImages {
		imagePath := specPath.Child("providerConfig").Child("machineImages").Index(imgIdx)
		if len(capabilitiesDefinition) != 0 {
			allErrs = append(allErrs, ValidateProviderMachineImage(imagePath, providerImage, capabilitiesDefinition)...)
			continue
		}

		for versionIdx, version := range providerImage.Versions {
			imageVersionPath := imagePath.Child("versions").Index(versionIdx)

			versionArch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)
			// validate image version image field
			if version.Image == "" {
				allErrs = append(allErrs, field.Required(
					imageVersionPath.Child("image"),
					fmt.Sprintf("must provide the image field for image version %s@%s and architecture: %s",
						providerImage.Name, version.Version, versionArch)))
			}
			// validate architecture field
			if !slices.Contains(v1beta1constants.ValidArchitectures, versionArch) {
				allErrs = append(allErrs, field.NotSupported(
					imageVersionPath.Child("architecture"),
					versionArch, v1beta1constants.ValidArchitectures))
			}
		}
	}

	return allErrs
}

// ValidateProviderMachineImage validates a CloudProfileConfig MachineImages entry.
func ValidateProviderMachineImage(validationPath *field.Path, providerImage apisgcp.MachineImages, capabilitiesDefinition core.Capabilities) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(providerImage.Name) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("name"), "must provide a name"))
	}

	if len(providerImage.Versions) == 0 {
		allErrs = append(allErrs, field.Required(validationPath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", providerImage.Name)))
	}

	for j, version := range providerImage.Versions {
		jdxPath := validationPath.Child("versions").Index(j)

		if len(version.Version) == 0 {
			allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
		}

		if len(version.CapabilitySets) == 0 {
			allErrs = append(allErrs, field.Required(jdxPath.Child("capabilitySets"), "must provide a capabilitySet"))
		}

		for k, capabilitySet := range version.CapabilitySets {
			kdxPath := jdxPath.Child("capabilitySets").Index(k)
			allErrs = append(allErrs, ValidateCapabilities(capabilitySet.Capabilities, capabilitiesDefinition, kdxPath.Child("capabilities"))...)

			if capabilitySet.Image == "" {
				allErrs = append(allErrs, field.Required(kdxPath.Child("image"),
					fmt.Sprintf("must provide the image field for image version %s@%s and capabilitySet: %v",
						providerImage.Name, version.Version, capabilitySet)))
			}
		}

		// ensure legacy fields are not set
		if len(ptr.Deref(version.Architecture, "")) != 0 {
			allErrs = append(allErrs, field.Forbidden(jdxPath.Child("architecture"), "must not set architecture when capabilitySet is defined"))
		}
		if len(version.Image) != 0 {
			allErrs = append(allErrs, field.Forbidden(jdxPath.Child("image"), "must not set image when capabilitySet is defined"))
		}
	}

	return allErrs
}

func validateCapabilitiesMapping(version core.MachineImageVersion, machineImage apisgcp.MachineImages, capabilitiesDefinition core.Capabilities, machineImageVersionPath *field.Path, allErrs field.ErrorList) field.ErrorList {
	versionCapabilitySets := GetVersionCapabilitySets(version, capabilitiesDefinition)

	for _, coreCapabilitySet := range versionCapabilitySets {
		isFound := false
		// search for the corresponding imageVersion.CapabilitySet
		for _, providerVersion := range machineImage.Versions {
			if version.Version == providerVersion.Version {
				for _, providerCapabilitySet := range providerVersion.CapabilitySets {
					if AreCapabilitiesEqual(coreCapabilitySet.Capabilities, providerCapabilitySet.Capabilities, capabilitiesDefinition) {
						isFound = true
					}
				}
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(machineImageVersionPath,
				fmt.Sprintf("machine image version %s@%s and capabilitySet %v is not defined in the providerConfig",
					machineImage.Name, version.Version, coreCapabilitySet.Capabilities)))
		}
	}
	return allErrs
}

func validateArchitectureMapping(version core.MachineImageVersion, cpConfigImagesContext *util.ImagesContext[apisgcp.MachineImages, apisgcp.MachineImageVersion], machineImage core.MachineImage, machineImageVersionPath *field.Path, allErrs field.ErrorList) field.ErrorList {
	for _, expectedArchitecture := range version.Architectures {
		// validate machine image version architectures
		if !slices.Contains(v1beta1constants.ValidArchitectures, expectedArchitecture) {
			allErrs = append(allErrs, field.NotSupported(
				machineImageVersionPath.Child("architectures"),
				expectedArchitecture, v1beta1constants.ValidArchitectures))
		}
		// validate machine image version with architecture x exists in cpConfig
		_, exists := cpConfigImagesContext.GetImageVersion(machineImage.Name, VersionArchitectureKey(version.Version, expectedArchitecture))
		if !exists {
			allErrs = append(allErrs, field.Required(machineImageVersionPath,
				fmt.Sprintf("machine image version %s@%s and architecture: %s is not defined in the providerConfig",
					machineImage.Name, version.Version, expectedArchitecture),
			))
			continue
		}
	}
	return allErrs
}

// NewProviderImagesContext creates a new ImagesContext for provider images.
func NewProviderImagesContext(providerImages []apisgcp.MachineImages) *util.ImagesContext[apisgcp.MachineImages, apisgcp.MachineImageVersion] {
	return util.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi apisgcp.MachineImages) string { return mi.Name }),
		func(mi apisgcp.MachineImages) map[string]apisgcp.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v apisgcp.MachineImageVersion) string { return providerMachineImageKey(v) })
		},
	)
}

func providerMachineImageKey(v apisgcp.MachineImageVersion) string {
	return VersionArchitectureKey(v.Version, ptr.Deref(v.Architecture, v1beta1constants.ArchitectureAMD64))
}

// VersionArchitectureKey returns a key for a version and architecture.
func VersionArchitectureKey(version, architecture string) string {
	return version + "-" + architecture
}

// ValidateCapabilities validates the capabilities of a machine type or machine image.
// It checks if the capabilities are supported by the cloud profile and if the architecture is defined correctly.
// It returns a list of field errors if any validation fails.
// THIS FUNCTION SHOULD BE MOVED TO GARDENER CORE AS IT WILL BE USED BY OTHER PROVIDERS AS WELL
func ValidateCapabilities(capabilities core.Capabilities, capabilitiesDefinition core.Capabilities, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	supportedCapabilityKeys := slices.Collect(maps.Keys(capabilitiesDefinition))

	for capabilityKey, capability := range capabilities {
		supportedValues, keyExists := capabilitiesDefinition[capabilityKey]
		if !keyExists {
			allErrs = append(allErrs, field.NotSupported(fldPath, capabilityKey, supportedCapabilityKeys))
			continue
		}
		for i, value := range capability {
			if !slices.Contains(supportedValues, value) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child(capabilityKey).Index(i), value, supportedValues))
			}
		}
	}

	// Check additional requirements for architecture
	//  must be defined when multiple architectures are supported by the cloud profile
	supportedArchitectures := capabilitiesDefinition[v1beta1constants.ArchitectureName]
	architectures := capabilities[v1beta1constants.ArchitectureName]
	if len(supportedArchitectures) > 1 && len(architectures) != 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(v1beta1constants.ArchitectureName), architectures, "must define exactly one architecture when multiple architectures are supported by the cloud profile"))
	}

	return allErrs
}

// GetVersionCapabilitySets returns the capability for a given machine image version and adds the default capabilitySet if applicable.
// THIS FUNCTION SHOULD BE MOVED TO GARDENER CORE AS IT WILL BE USED BY OTHER PROVIDERS AS WELL
func GetVersionCapabilitySets(version core.MachineImageVersion, capabilitiesDefinition core.Capabilities) []core.CapabilitySet {
	versionCapabilitySets := version.CapabilitySets
	if len(version.CapabilitySets) == 0 {
		// It is allowed not to define capabilitySets in the machine image version if there is only one architecture
		// if so the capabilityDefinition is used as default
		if len(capabilitiesDefinition[v1beta1constants.ArchitectureName]) == 1 {
			versionCapabilitySets = []core.CapabilitySet{{Capabilities: capabilitiesDefinition}}
		}
	}
	return versionCapabilitySets
}

// AreCapabilitiesEqual checks if two capabilities are equal.
// It compares the keys and values of the capabilities maps.
// THIS FUNCTION SHOULD BE MOVED TO GARDENER CORE AS IT WILL BE USED BY OTHER PROVIDERS AS WELL
func AreCapabilitiesEqual(a, b, capabilitiesDefinition core.Capabilities) bool {
	a = SetDefaultCapabilities(a, capabilitiesDefinition)
	b = SetDefaultCapabilities(b, capabilitiesDefinition)
	for key, valuesA := range a {
		valuesB, exists := b[key]
		if !exists || len(valuesA) != len(valuesB) {
			return false
		}
		for _, value := range valuesA {
			if !slices.Contains(valuesB, value) {
				return false
			}
		}
	}
	return true
}

// SetDefaultCapabilities sets the default capabilities based on a capabilitiesDefinition for a machine type or machine image.
// THIS FUNCTION SHOULD BE MOVED TO GARDENER CORE AS IT WILL BE USED BY OTHER PROVIDERS AS WELL
func SetDefaultCapabilities(capabilities, capabilitiesDefinition core.Capabilities) core.Capabilities {
	if len(capabilities) == 0 {
		capabilities = make(core.Capabilities)
	}

	for key, values := range capabilitiesDefinition {
		if _, exists := capabilities[key]; !exists {
			capabilities[key] = values
		}
	}

	return capabilities
}
