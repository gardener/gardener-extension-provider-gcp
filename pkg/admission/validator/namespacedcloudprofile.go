// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"maps"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

// NewNamespacedCloudProfileValidator returns a new instance of a namespaced cloud profile validator.
func NewNamespacedCloudProfileValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &namespacedCloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type namespacedCloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Validate validates the given NamespacedCloudProfile objects.
func (p *namespacedCloudProfile) Validate(ctx context.Context, newObj, _ client.Object) error {
	profile, ok := newObj.(*core.NamespacedCloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if profile.DeletionTimestamp != nil {
		return nil
	}

	cpConfig := &api.CloudProfileConfig{}
	if profile.Spec.ProviderConfig != nil {
		var err error
		cpConfig, err = admission.DecodeCloudProfileConfig(p.decoder, profile.Spec.ProviderConfig)
		if err != nil {
			return err
		}
	}

	parentCloudProfile := profile.Spec.Parent
	if parentCloudProfile.Kind != constants.CloudProfileReferenceKindCloudProfile {
		return fmt.Errorf("parent reference must be of kind CloudProfile (unsupported kind: %s)", parentCloudProfile.Kind)
	}
	parentProfile := &gardencorev1beta1.CloudProfile{}
	if err := p.client.Get(ctx, client.ObjectKey{Name: parentCloudProfile.Name}, parentProfile); err != nil {
		return err
	}

	// Validate provider config as-is without transforming to parent format.
	// Mixed format (old image with architecture and new capabilityFlavors) is supported per version.
	return p.validateNamespacedCloudProfileProviderConfig(cpConfig, profile.Spec, parentProfile.Spec).ToAggregate()
}

// validateNamespacedCloudProfileProviderConfig validates the CloudProfileConfig passed with a NamespacedCloudProfile.
func (p *namespacedCloudProfile) validateNamespacedCloudProfileProviderConfig(providerConfig *api.CloudProfileConfig, profileSpec core.NamespacedCloudProfileSpec, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, p.validateMachineImages(providerConfig, profileSpec.MachineImages, parentSpec)...)

	return allErrs
}

func (p *namespacedCloudProfile) validateMachineImages(providerConfig *api.CloudProfileConfig, machineImages []core.MachineImage, parentSpec gardencorev1beta1.CloudProfileSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	capabilityDefinitions := parentSpec.MachineCapabilities

	imagesPath := field.NewPath("spec.providerConfig.machineImages")
	for i, machineImage := range providerConfig.MachineImages {
		idxPath := imagesPath.Index(i)
		allErrs = append(allErrs, validation.ValidateProviderMachineImage(machineImage, parentSpec.MachineCapabilities, idxPath)...)
	}

	namespacedImages := gutil.NewCoreImagesContext(machineImages)
	parentImages := gutil.NewV1beta1ImagesContext(parentSpec.MachineImages)
	providerImages := validation.NewProviderImagesContextLegacy(providerConfig.MachineImages)

	// Create a map of provider images grouped by version for mixed format support
	providerVersionsMap := make(map[string]map[string][]api.MachineImageVersion)
	for _, img := range providerConfig.MachineImages {
		if providerVersionsMap[img.Name] == nil {
			providerVersionsMap[img.Name] = make(map[string][]api.MachineImageVersion)
		}
		for _, v := range img.Versions {
			providerVersionsMap[img.Name][v.Version] = append(providerVersionsMap[img.Name][v.Version], v)
		}
	}

	for _, machineImage := range namespacedImages.Images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.GetImage(machineImage.Name)
		_, existsInProvider := providerImages.GetImage(machineImage.Name)
		if !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			if len(capabilityDefinitions) == 0 {
				// check that each architecture defined has a corresponding entry in the providerConfig
				for _, expectedArchitecture := range version.Architectures {
					if _, exists := providerImages.GetImageVersion(machineImage.Name, validation.VersionArchitectureKey(version.Version, expectedArchitecture)); !existsInParent && !exists {
						allErrs = append(allErrs, field.Required(imagesPath,
							fmt.Sprintf("machine image version %s@%s and architecture: %q is not defined in the NamespacedCloudProfile providerConfig",
								machineImage.Name, version.Version, expectedArchitecture),
						))
					}
				}
			} else {
				// check that each capabilityFlavor defined has a corresponding entry in the providerConfig
				// Support mixed format: group provider versions by version string
				providerVersions, versionExists := providerVersionsMap[machineImage.Name][version.Version]
				if !versionExists || len(providerVersions) == 0 {
					allErrs = append(allErrs, field.Required(imagesPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
					))
					continue
				}
				allErrs = append(allErrs, validateMachineImageCapabilitiesMixed(machineImage, version, providerVersions, capabilityDefinitions, imagesPath)...)
			}
		}
	}

	for imageIdx, machineImage := range providerConfig.MachineImages {
		// Check that the machine image version is not already defined in the parent CloudProfile.
		if _, exists := parentImages.GetImage(machineImage.Name); exists {
			for versionIdx, version := range machineImage.Versions {
				if _, exists := parentImages.GetImageVersion(machineImage.Name, version.Version); exists {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
						fmt.Sprintf("machine image version %s@%s is already defined in the parent CloudProfile", machineImage.Name, version.Version),
					))
				}
			}
		}
		// Check that the machine image version is defined in the NamespacedCloudProfile.
		if _, exists := namespacedImages.GetImage(machineImage.Name); !exists {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages").Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}
		for versionIdx, version := range machineImage.Versions {
			profileImageVersion, exists := namespacedImages.GetImageVersion(machineImage.Name, version.Version)
			if !exists {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
					fmt.Sprintf("%s@%s", machineImage.Name, version.Version),
					"machine image version is not defined in the NamespacedCloudProfile",
				))
			}

			if len(capabilityDefinitions) == 0 {
				// For non-capabilities CloudProfile, check if architecture is valid
				providerConfigArchitecture := ptr.Deref(version.Architecture, constants.ArchitectureAMD64)
				// If version doesn't exist, all architectures are excess
				if !exists || !slices.Contains(profileImageVersion.Architectures, providerConfigArchitecture) {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages"),
						fmt.Sprintf("machine image version %s@%s has an excess entry for architecture %q, which is not defined in the machineImages spec",
							machineImage.Name, version.Version, providerConfigArchitecture),
					))
				}
			} else if exists {
				// For capabilities CloudProfile, validate excess architectures in old format
				if version.Image != "" && len(version.CapabilityFlavors) == 0 {
					providerArch := ptr.Deref(version.Architecture, constants.ArchitectureAMD64)
					allErrs = append(allErrs, validateExcessArchitecture(machineImage.Name, version.Version, providerArch, profileImageVersion.CapabilityFlavors, capabilityDefinitions, imagesPath)...)
				}
			}
		}
	}

	return allErrs
}

// validateExcessArchitecture checks if a provider's architecture is defined in the spec's capability flavors
func validateExcessArchitecture(imageName, versionStr, providerArch string, specCapabilityFlavors []core.MachineImageFlavor, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1beta1Flavors []gardencorev1beta1.MachineImageFlavor
	for _, f := range specCapabilityFlavors {
		// Manually convert core.Capabilities to v1beta1.Capabilities
		v1betaCapabilities := convertCoreCapabilitiesToV1beta1(f.Capabilities)
		v1beta1Flavors = append(v1beta1Flavors, gardencorev1beta1.MachineImageFlavor{Capabilities: v1betaCapabilities})
	}
	defaultedCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1beta1Flavors, capabilityDefinitions)

	isFound := false
	for _, flavor := range defaultedCapabilityFlavors {
		expectedArchitectures := flavor.Capabilities[constants.ArchitectureName]
		if slices.Contains(expectedArchitectures, providerArch) {
			isFound = true
			break
		}
	}
	if !isFound {
		allErrs = append(allErrs, field.Forbidden(path,
			fmt.Sprintf("machine image version %s@%s has an excess architecture %q, which is not defined in the machineImages spec",
				imageName, versionStr, providerArch)))
	}

	return allErrs
}

// convertCoreCapabilitiesToV1beta1 converts core.Capabilities to v1beta1.Capabilities manually
func convertCoreCapabilitiesToV1beta1(coreCapabilities core.Capabilities) gardencorev1beta1.Capabilities {
	v1beta1Capabilities := make(gardencorev1beta1.Capabilities)
	for k, v := range coreCapabilities {
		// Copy the slice values
		v1beta1Capabilities[k] = append([]string{}, v...)
	}
	return v1beta1Capabilities
}

// validateMachineImageCapabilitiesMixed validates machine image capabilities with mixed format support.
// It handles both old format (image with architecture) and new format (capabilityFlavors).
func validateMachineImageCapabilitiesMixed(machineImage core.MachineImage, version core.MachineImageVersion, providerVersions []api.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var v1betaVersion gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1betaVersion, nil); err != nil {
		return append(allErrs, field.InternalError(path, err))
	}
	defaultedVersionCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1betaVersion.CapabilityFlavors, capabilityDefinitions)

	// Check if any provider version uses new format (capabilityFlavors)
	var capabilityFlavorsVersion *api.MachineImageVersion
	for i := range providerVersions {
		if len(providerVersions[i].CapabilityFlavors) > 0 {
			capabilityFlavorsVersion = &providerVersions[i]
			break
		}
	}

	if capabilityFlavorsVersion != nil {
		// New format: validate using capabilityFlavors
		allErrs = append(allErrs, validateCapabilityFlavorsFormat(machineImage, version, *capabilityFlavorsVersion, defaultedVersionCapabilityFlavors, capabilityDefinitions, path)...)
	} else {
		// Old format: validate using image with architecture
		allErrs = append(allErrs, validateLegacyFormatWithCapabilities(machineImage, version, providerVersions, defaultedVersionCapabilityFlavors, path)...)
	}

	return allErrs
}

// validateCapabilityFlavorsFormat validates when provider uses new format (capabilityFlavors)
func validateCapabilityFlavorsFormat(machineImage core.MachineImage, version core.MachineImageVersion, providerVersion api.MachineImageVersion, defaultedCapabilityFlavors []gardencorev1beta1.MachineImageFlavor, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// 1. Create an error for each capabilityFlavor in the providerConfig that is not defined in the core machine image version
	for _, providerCapabilityFlavor := range providerVersion.CapabilityFlavors {
		isFound := false
		for _, defaultedCapabilityFlavor := range defaultedCapabilityFlavors {
			defaultedProviderCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(defaultedCapabilityFlavor.Capabilities, defaultedProviderCapabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Forbidden(path,
				fmt.Sprintf("machine image version %s@%s has an excess capabilityFlavor %v, which is not defined in the NamespacedCloudProfile machineImages spec",
					machineImage.Name, version.Version, providerCapabilityFlavor.Capabilities)))
		}
	}

	// 2. Create an error for each capabilityFlavor in the core machine image version that is not defined in the providerConfig
	for _, capabilityFlavor := range defaultedCapabilityFlavors {
		isFound := false
		for _, providerCapabilityFlavor := range providerVersion.CapabilityFlavors {
			defaultedProviderCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(defaultedProviderCapabilities, capabilityFlavor.Capabilities) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(path,
				fmt.Sprintf("machine image version %s@%s and capabilityFlavor %v is not defined in the NamespacedCloudProfile providerConfig",
					machineImage.Name, version.Version, capabilityFlavor.Capabilities)))
		}
	}

	return allErrs
}

// validateLegacyFormatWithCapabilities validates when provider uses old format (image with architecture) in capabilities CloudProfile
func validateLegacyFormatWithCapabilities(machineImage core.MachineImage, version core.MachineImageVersion, providerVersions []api.MachineImageVersion, defaultedCapabilityFlavors []gardencorev1beta1.MachineImageFlavor, path *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// Collect architectures from all provider version entries
	architecturesMap := utils.CreateMapFromSlice(providerVersions, func(v api.MachineImageVersion) string {
		return ptr.Deref(v.Architecture, constants.ArchitectureAMD64)
	})
	availableArchitectures := slices.Collect(maps.Keys(architecturesMap))

	// 1. Check for excess architectures in provider that are not in spec
	for _, arch := range availableArchitectures {
		isFound := false
		for _, defaultedCapabilityFlavor := range defaultedCapabilityFlavors {
			expectedArchitectures := defaultedCapabilityFlavor.Capabilities[constants.ArchitectureName]
			if slices.Contains(expectedArchitectures, arch) {
				isFound = true
				break
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Forbidden(path,
				fmt.Sprintf("machine image version %s@%s has an excess architecture %q, which is not defined in the machineImages spec",
					machineImage.Name, version.Version, arch)))
		}
	}

	// 2. Check that each expected capability flavor has a corresponding architecture in provider
	for _, capabilityFlavor := range defaultedCapabilityFlavors {
		expectedArchitectures := capabilityFlavor.Capabilities[constants.ArchitectureName]
		for _, expectedArch := range expectedArchitectures {
			if !slices.Contains(availableArchitectures, expectedArch) {
				allErrs = append(allErrs, field.Required(path,
					fmt.Sprintf("machine image version %s@%s and capabilityFlavor %v is not defined in the NamespacedCloudProfile providerConfig",
						machineImage.Name, version.Version, capabilityFlavor.Capabilities)))
			}
		}
	}

	return allErrs
}
