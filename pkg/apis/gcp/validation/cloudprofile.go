// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"

	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *apisgcp.CloudProfileConfig, machineImages []core.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, specPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImagesPath := specPath.Child("providerConfig").Child("machineImages")
	// Validate machine images section
	allErrs = append(allErrs, validateMachineImages(cpConfig.MachineImages, capabilityDefinitions, machineImagesPath)...)
	//if len(allErrs) > 0 {
	//	return allErrs
	//}

	allErrs = append(allErrs, validateProviderImagesMapping(cpConfig.MachineImages, machineImages, capabilityDefinitions, specPath)...)

	return allErrs
}

// verify that for each cp image a provider image exists
func validateProviderImagesMapping(cpConfigImages []apisgcp.MachineImages, machineImages []core.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	cpConfigImagesContext := NewProviderImagesContext(cpConfigImages)

	for idxImage, machineImage := range machineImages {
		if len(machineImage.Versions) == 0 {
			continue
		}
		machineImagePath := fldPath.Child("machineImages").Index(idxImage)
		// validate that for each machine image there is a corresponding cpConfig image
		_, imageExists := cpConfigImagesContext.GetImage(machineImage.Name)
		if !imageExists {
			allErrs = append(allErrs, field.Required(machineImagePath,
				fmt.Sprintf("must provide an image mapping for image %q in providerConfig", machineImage.Name)))
			continue
		}
		// validate that for each machine image version entry a mapped entry in cpConfig exists
		for idxVersion, version := range machineImage.Versions {
			machineImageVersionPath := machineImagePath.Child("versions").Index(idxVersion)

			if len(capabilityDefinitions) > 0 {
				providerVersion, versionExists := cpConfigImagesContext.GetImageVersion(machineImage.Name, version.Version)
				if !versionExists {
					allErrs = append(allErrs, field.Required(machineImageVersionPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig", machineImage.Name, version.Version),
					))
					continue
				}
				allErrs = append(allErrs, validateImageFlavorMapping(version, providerVersion, capabilityDefinitions, machineImage, machineImageVersionPath)...)
			} else {
				allErrs = append(allErrs, validateArchitectureMapping(version, cpConfigImages, machineImage, machineImageVersionPath)...)
			}
		}
	}
	return allErrs
}

// validateImageFlavorMapping validates that each flavor in a version has a corresponding mapping
func validateImageFlavorMapping(version core.MachineImageVersion, imageVersion apisgcp.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, machineImage core.MachineImage, machineImageVersionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Version gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1beta1Version, nil); err != nil {
		return append(allErrs, field.InternalError(machineImageVersionPath, err))
	}

	defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Version.CapabilityFlavors, capabilityDefinitions)
	for idxCapability, defaultedCapabilitySet := range defaultedCapabilityFlavors {
		isFound := false
		// search for the corresponding imageVersion.MachineImageFlavor
		for _, providerCapabilityFlavor := range imageVersion.CapabilityFlavors {
			providerDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(defaultedCapabilitySet.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(machineImageVersionPath.Child("capabilityFlavors").Index(idxCapability),
				fmt.Sprintf("machine image version %s@%s and capabilitySet %v is not defined in the providerConfig", machineImage.Name, version.Version, defaultedCapabilitySet.Capabilities)))
		}
	}
	return allErrs
}

// validateMachineImages validates the machine images section of CloudProfileConfig
func validateMachineImages(machineImages []apisgcp.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Ensure at least one machine image is provided
	if len(machineImages) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, "must provide at least one machine image"))
		return allErrs
	}

	// Validate each machine image
	for i, machineImage := range machineImages {
		imagePath := fldPath.Index(i)
		allErrs = append(allErrs, ValidateProviderMachineImage(machineImage, capabilityDefinitions, imagePath)...)
	}

	return allErrs
}

// ValidateProviderMachineImage validates a CloudProfileConfig MachineImages entry.
func ValidateProviderMachineImage(providerImage apisgcp.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, imagePath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(providerImage.Name) == 0 {
		allErrs = append(allErrs, field.Required(imagePath.Child("name"), "must provide a name"))
	}

	if len(providerImage.Versions) == 0 {
		allErrs = append(allErrs, field.Required(imagePath.Child("versions"), fmt.Sprintf("must provide at least one version for machine image %q", providerImage.Name)))
	}

	for j, version := range providerImage.Versions {
		jdxPath := imagePath.Child("versions").Index(j)
		if len(capabilityDefinitions) > 0 {
			if len(version.Version) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
			}

			if len(version.CapabilityFlavors) == 0 {
				allErrs = append(allErrs, field.Required(jdxPath.Child("capabilityFlavors"), "must provide a capabilityFlavor"))
			}

			for k, capabilityFlavor := range version.CapabilityFlavors {
				kdxPath := jdxPath.Child("capabilityFlavors").Index(k)
				allErrs = append(allErrs, gutil.ValidateCapabilities(capabilityFlavor.Capabilities, capabilityDefinitions, kdxPath.Child("capabilities"))...)

				if capabilityFlavor.Image == "" {
					allErrs = append(allErrs, field.Required(kdxPath.Child("image"),
						fmt.Sprintf("must provide the image field for image version %s@%s and capabilityFlavor: %v",
							providerImage.Name, version.Version, capabilityFlavor)))
				}
			}

			// ensure legacy fields are not set
			if ptr.Deref(version.Architecture, "") != "" {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("architecture"), "must not set architecture when capabilityFlavor is defined"))
			}
			if len(version.Image) != 0 {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("image"), "must not set image when capabilityFlavor is defined"))
			}
		} else {
			versionArch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)
			// validate image version image field
			if version.Image == "" {
				allErrs = append(allErrs, field.Required(
					jdxPath.Child("image"),
					fmt.Sprintf("must provide the image field for image version %s@%s and architecture: %s",
						providerImage.Name, version.Version, versionArch)))
			}
			// validate architecture field
			if !slices.Contains(v1beta1constants.ValidArchitectures, versionArch) {
				allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), versionArch, v1beta1constants.ValidArchitectures))
			}
			// ensure capability related fields are not set
			if len(version.CapabilityFlavors) != 0 {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("capabilityFlavors"), "capabilityFlavors must not be set when cloudprofile is not using capabilities"))
			}
		}
	}

	return allErrs
}

func validateArchitectureMapping(
	version core.MachineImageVersion,
	cpConfigImages []apisgcp.MachineImages,
	machineImage core.MachineImage,
	machineImageVersionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	cpConfigImagesContext := NewProviderImagesContextLegacy(cpConfigImages)

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

// NewProviderImagesContextLegacy creates a new ImagesContext for provider images.
func NewProviderImagesContextLegacy(providerImages []apisgcp.MachineImages) *gutil.ImagesContext[apisgcp.MachineImages, apisgcp.MachineImageVersion] {
	return gutil.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi apisgcp.MachineImages) string { return mi.Name }),
		func(mi apisgcp.MachineImages) map[string]apisgcp.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v apisgcp.MachineImageVersion) string { return providerMachineImageKey(v) })
		},
	)
}

// NewProviderImagesContext creates a new ImagesContext for provider images.
func NewProviderImagesContext(providerImages []apisgcp.MachineImages) *gutil.ImagesContext[apisgcp.MachineImages, apisgcp.MachineImageVersion] {
	return gutil.NewImagesContext(
		utils.CreateMapFromSlice(providerImages, func(mi apisgcp.MachineImages) string { return mi.Name }),
		func(mi apisgcp.MachineImages) map[string]apisgcp.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v apisgcp.MachineImageVersion) string { return v.Version })
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
