// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
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

	// TODO(Roncossek): Remove TransformSpecToParentFormat once all CloudProfiles have been migrated to use CapabilityFlavors and the Architecture fields are effectively forbidden or have been removed.
	if err := helper.SimulateTransformToParentFormat(cpConfig, profile, parentProfile.Spec.MachineCapabilities); err != nil {
		return err
	}

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

	for _, machineImage := range namespacedImages.Images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.GetImage(machineImage.Name)
		providerImage, existsInProvider := providerImages.GetImage(machineImage.Name)
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
				isFound := false
				for _, providerImageVersion := range providerImage.Versions {
					if providerImageVersion.Version == version.Version {
						isFound = true
						allErrs = append(allErrs, validateMachineImageCapabilities(machineImage, version, providerImageVersion, capabilityDefinitions, imagesPath)...)
					}
				}
				if !isFound {
					allErrs = append(allErrs, field.Required(imagesPath,
						fmt.Sprintf("machine image version %s@%s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name, version.Version),
					))
				}
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
				providerConfigArchitecture := ptr.Deref(version.Architecture, constants.ArchitectureAMD64)
				if !slices.Contains(profileImageVersion.Architectures, providerConfigArchitecture) {
					allErrs = append(allErrs, field.Forbidden(
						field.NewPath("spec.providerConfig.machineImages"),
						fmt.Sprintf("machine image version %s@%s has an excess entry for architecture %q, which is not defined in the machineImages spec",
							machineImage.Name, version.Version, providerConfigArchitecture),
					))
				}
			}
		}
	}

	return allErrs
}

func validateMachineImageCapabilities(machineImage core.MachineImage, version core.MachineImageVersion, providerImageVersion api.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, versionPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	var v1betaVersion gardencorev1beta1.MachineImageVersion
	if err := gardencoreapi.Scheme.Convert(&version, &v1betaVersion, nil); err != nil {
		return append(allErrs, field.InternalError(versionPath, err))
	}
	defaultedVersionCapabilityFlavors := gardencorev1beta1helper.GetImageFlavorsWithAppliedDefaults(v1betaVersion.CapabilityFlavors, capabilityDefinitions)

	// 1. Create an error for each capabilityFlavor in the providerConfig that is not defined in the core machine image version
	for _, providerCapabilityFlavor := range providerImageVersion.CapabilityFlavors {
		isFound := false
		for _, defaultedCapabilityFlavors := range defaultedVersionCapabilityFlavors {
			defaultedProviderCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(defaultedCapabilityFlavors.Capabilities, defaultedProviderCapabilities) {
				isFound = true
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Forbidden(versionPath,
				fmt.Sprintf("machine image version %s@%s has an excess capabilityFlavor %v, which is not defined in the NamespacedCloudProfile machineImages spec",
					machineImage.Name, version.Version, providerCapabilityFlavor.Capabilities)))
		}
	}

	// 2. Create an error for each capabilityFlavor in the core machine image version that is not defined in the providerConfig
	for _, capabilityFlavor := range defaultedVersionCapabilityFlavors {
		isFound := false
		for _, providerCapabilityFlavor := range providerImageVersion.CapabilityFlavors {
			defaultedProviderCapabilities := gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(providerCapabilityFlavor.Capabilities, capabilityDefinitions)
			if gardencorev1beta1helper.AreCapabilitiesEqual(defaultedProviderCapabilities, capabilityFlavor.Capabilities) {
				isFound = true
			}
		}
		if !isFound {
			allErrs = append(allErrs, field.Required(versionPath,
				fmt.Sprintf("machine image version %s@%s and capabilityFlavor %v is not defined in the NamespacedCloudProfile providerConfig",
					machineImage.Name, version.Version, capabilityFlavor.Capabilities)))
			continue
		}
	}
	return allErrs
}
