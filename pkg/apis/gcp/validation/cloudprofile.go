// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"maps"
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
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

// ValidateCloudProfileConfig validates a CloudProfileConfig object.
func ValidateCloudProfileConfig(cpConfig *apisgcp.CloudProfileConfig, machineImages []core.MachineImage, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, specPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	machineImagesPath := specPath.Child("providerConfig").Child("machineImages")
	// Validate machine images section
	allErrs = append(allErrs, validateMachineImages(cpConfig.MachineImages, capabilityDefinitions, machineImagesPath)...)

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
		providerImage, imageExists := cpConfigImagesContext.GetImage(machineImage.Name)
		if !imageExists {
			allErrs = append(allErrs, field.Required(machineImagePath,
				fmt.Sprintf("must provide an image mapping for image %q in providerConfig", machineImage.Name)))
			continue
		}
		// validate that for each machine image version entry a mapped entry in cpConfig exists
		for idxVersion, version := range machineImage.Versions {
			machineImageVersionPath := machineImagePath.Child("versions").Index(idxVersion)

			if len(capabilityDefinitions) > 0 {
				// Group provider versions by version string to handle mixed format
				// (old format may have multiple entries per version with different architectures)
				groupedVersions := helper.GroupVersionsByVersionString(providerImage.Versions)
				providerVersions, versionExists := groupedVersions[version.Version]
				if !versionExists || len(providerVersions) == 0 {
					allErrs = append(allErrs, field.Required(machineImageVersionPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the providerConfig", machineImage.Name, version.Version),
					))
					continue
				}
				allErrs = append(allErrs, validateImageFlavorMappingMixed(version, providerVersions, capabilityDefinitions, machineImage, machineImageVersionPath)...)
			} else {
				allErrs = append(allErrs, validateArchitectureMapping(version, cpConfigImages, machineImage, machineImageVersionPath)...)
			}
		}
	}
	return allErrs
}

// validateImageFlavorMappingMixed validates that each flavor in a version has a corresponding mapping.
// This function handles both the new format (capabilityFlavors) and old format (image with architecture).
// For mixed format support, multiple provider version entries may exist for the same version string.
func validateImageFlavorMappingMixed(version core.MachineImageVersion, providerVersions []apisgcp.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, machineImage core.MachineImage, machineImageVersionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Version gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1beta1Version, nil); err != nil {
		return append(allErrs, field.InternalError(machineImageVersionPath, err))
	}

	defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Version.CapabilityFlavors, capabilityDefinitions)

	capabilityFlavorsVersion := FindCapabilityFlavorsVersion(providerVersions)
	if capabilityFlavorsVersion != nil {
		// New format: validate against capabilityFlavors
		allErrs = append(allErrs, ValidateMissingCapabilityFlavors(
			machineImage.Name, version.Version,
			defaultedCapabilityFlavors,
			capabilityFlavorsVersion.CapabilityFlavors,
			capabilityDefinitions,
			machineImageVersionPath,
			"providerConfig",
			true, // include index path for CloudProfile validation
		)...)
	} else {
		// Old format: collect architectures from all provider version entries
		availableArchitectures := CollectAvailableArchitectures(providerVersions)
		allErrs = append(allErrs, ValidateMissingArchitectures(
			machineImage.Name, version.Version,
			defaultedCapabilityFlavors,
			availableArchitectures,
			machineImageVersionPath,
			"providerConfig",
			true, // include index path for CloudProfile validation
		)...)
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
		if len(version.Version) == 0 {
			allErrs = append(allErrs, field.Required(jdxPath.Child("version"), "must provide a version"))
		}

		hasCapabilityFlavors := len(version.CapabilityFlavors) > 0
		hasLegacyImage := version.Image != ""

		if len(capabilityDefinitions) > 0 {
			// When CloudProfile defines capabilities, allow either old format (image) or new format (capabilityFlavors) per version
			if hasCapabilityFlavors && hasLegacyImage {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("image"), "must not be set together with capabilityFlavors. Use one format per version."))
			} else if hasCapabilityFlavors {
				// New format: validate capabilityFlavors
				allErrs = append(allErrs, validateCapabilityFlavors(providerImage, version, capabilityDefinitions, jdxPath)...)
			} else if hasLegacyImage {
				// Old format: validate image with architecture (mixed format support)
				allErrs = append(allErrs, validateLegacyImageWithCapabilities(version, jdxPath)...)
			} else {
				// Neither format specified
				allErrs = append(allErrs, field.Required(jdxPath.Child("image"),
					fmt.Sprintf("must provide either image or capabilityFlavors for machine image %s@%s", providerImage.Name, version.Version)))
			}
		} else {
			// Without capabilities, only old format with image is supported
			if hasCapabilityFlavors {
				allErrs = append(allErrs, field.Forbidden(jdxPath.Child("capabilityFlavors"), "capabilityFlavors must not be set when cloudprofile is not using capabilities"))
			}
			allErrs = append(allErrs, validateLegacyImage(providerImage, version, jdxPath)...)
		}
	}

	return allErrs
}

// validateCapabilityFlavors validates the new format (capabilityFlavors) for a machine image version.
func validateCapabilityFlavors(providerImage apisgcp.MachineImages, version apisgcp.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for k, capabilityFlavor := range version.CapabilityFlavors {
		kdxPath := jdxPath.Child("capabilityFlavors").Index(k)
		allErrs = append(allErrs, gutil.ValidateCapabilities(capabilityFlavor.Capabilities, capabilityDefinitions, kdxPath.Child("capabilities"))...)

		if capabilityFlavor.Image == "" {
			allErrs = append(allErrs, field.Required(kdxPath.Child("image"),
				fmt.Sprintf("must provide the image field for image version %s@%s and capabilityFlavor: %v",
					providerImage.Name, version.Version, capabilityFlavor)))
		}
	}

	// Ensure legacy fields are not set when using new format
	if ptr.Deref(version.Architecture, "") != "" {
		allErrs = append(allErrs, field.Forbidden(jdxPath.Child("architecture"), "must not set architecture when capabilityFlavors is defined"))
	}

	return allErrs
}

// validateLegacyImageWithCapabilities validates old format (image with architecture) when CloudProfile uses capabilities.
// This allows architecture field since it will be converted to capability flavors.
func validateLegacyImageWithCapabilities(version apisgcp.MachineImageVersion, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	versionArch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)

	// Validate architecture is valid since it will be used for capability mapping
	if !slices.Contains(v1beta1constants.ValidArchitectures, versionArch) {
		allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), versionArch, v1beta1constants.ValidArchitectures))
	}

	return allErrs
}

// validateLegacyImage validates old format (image with architecture) when CloudProfile does not use capabilities.
func validateLegacyImage(providerImage apisgcp.MachineImages, version apisgcp.MachineImageVersion, jdxPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	versionArch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)

	// Validate image version image field
	if version.Image == "" {
		allErrs = append(allErrs, field.Required(
			jdxPath.Child("image"),
			fmt.Sprintf("must provide the image field for image version %s@%s and architecture: %s",
				providerImage.Name, version.Version, versionArch)))
	}
	// Validate architecture field
	if !slices.Contains(v1beta1constants.ValidArchitectures, versionArch) {
		allErrs = append(allErrs, field.NotSupported(jdxPath.Child("architecture"), versionArch, v1beta1constants.ValidArchitectures))
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

// FindCapabilityFlavorsVersion finds the first provider version that uses the new format (capabilityFlavors).
// Returns nil if no version uses the new format.
func FindCapabilityFlavorsVersion(providerVersions []apisgcp.MachineImageVersion) *apisgcp.MachineImageVersion {
	for i := range providerVersions {
		if len(providerVersions[i].CapabilityFlavors) > 0 {
			return &providerVersions[i]
		}
	}
	return nil
}

// CollectAvailableArchitectures collects unique architectures from all provider version entries (old format).
func CollectAvailableArchitectures(providerVersions []apisgcp.MachineImageVersion) []string {
	architecturesMap := utils.CreateMapFromSlice(providerVersions, func(v apisgcp.MachineImageVersion) string {
		return ptr.Deref(v.Architecture, v1beta1constants.ArchitectureAMD64)
	})
	return slices.Collect(maps.Keys(architecturesMap))
}

// ValidateMissingCapabilityFlavors checks that all expected capability flavors from the spec are defined in the provider config.
// If includeIndexPath is true, adds .capabilityFlavors[idx] to the field path.
func ValidateMissingCapabilityFlavors(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	providerCapabilityFlavors []apisgcp.MachineImageFlavor,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
	path *field.Path,
	errorMsgSuffix string,
	includeIndexPath bool,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for idxCapability, specCapabilitySet := range defaultedSpecCapabilities {
		isFound := false
		for _, providerCapabilityFlavor := range providerCapabilityFlavors {
			providerDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(specCapabilitySet.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			errPath := path
			if includeIndexPath {
				errPath = path.Child("capabilityFlavors").Index(idxCapability)
			}
			allErrs = append(allErrs, field.Required(errPath,
				fmt.Sprintf("machine image version %s@%s and capabilityFlavor %v is not defined in the %s", imageName, versionStr, specCapabilitySet.Capabilities, errorMsgSuffix)))
		}
	}

	return allErrs
}

// ValidateExcessCapabilityFlavors checks that the provider config doesn't have extra capability flavors not defined in the spec.
func ValidateExcessCapabilityFlavors(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	providerCapabilityFlavors []apisgcp.MachineImageFlavor,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
	path *field.Path,
	errorMsgSuffix string,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, providerCapabilityFlavor := range providerCapabilityFlavors {
		isFound := false
		for _, specCapabilitySet := range defaultedSpecCapabilities {
			providerDefaultedCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(specCapabilitySet.Capabilities, providerDefaultedCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Forbidden(path,
				fmt.Sprintf("machine image version %s@%s has an excess capabilityFlavor %v, which is not defined in the %s", imageName, versionStr, providerCapabilityFlavor.Capabilities, errorMsgSuffix)))
		}
	}

	return allErrs
}

// ValidateMissingArchitectures checks that all expected architectures from capability flavors are available in the provider.
// If includeIndexPath is true, adds .capabilityFlavors[idx] to the field path.
func ValidateMissingArchitectures(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	availableArchitectures []string,
	path *field.Path,
	errorMsgSuffix string,
	includeIndexPath bool,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for idxCapability, specCapabilitySet := range defaultedSpecCapabilities {
		expectedArchitectures := specCapabilitySet.Capabilities[v1beta1constants.ArchitectureName]
		for _, expectedArch := range expectedArchitectures {
			if !slices.Contains(availableArchitectures, expectedArch) {
				errPath := path
				if includeIndexPath {
					errPath = path.Child("capabilityFlavors").Index(idxCapability)
				}
				allErrs = append(allErrs, field.Required(errPath,
					fmt.Sprintf("machine image version %s@%s and capabilityFlavor %v is not defined in the %s", imageName, versionStr, specCapabilitySet.Capabilities, errorMsgSuffix)))
			}
		}
	}

	return allErrs
}

// ValidateExcessArchitectures checks that the provider doesn't have extra architectures not defined in the spec capabilities.
func ValidateExcessArchitectures(
	imageName, versionStr string,
	defaultedSpecCapabilities []gardencorev1beta1.MachineImageFlavor,
	availableArchitectures []string,
	path *field.Path,
	errorMsgSuffix string,
) field.ErrorList {
	allErrs := field.ErrorList{}

	for _, arch := range availableArchitectures {
		isFound := false
		for _, specCapabilitySet := range defaultedSpecCapabilities {
			expectedArchitectures := specCapabilitySet.Capabilities[v1beta1constants.ArchitectureName]
			if slices.Contains(expectedArchitectures, arch) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Forbidden(path,
				fmt.Sprintf("machine image version %s@%s has an excess architecture %q, which is not defined in the %s", imageName, versionStr, arch, errorMsgSuffix)))
		}
	}

	return allErrs
}
