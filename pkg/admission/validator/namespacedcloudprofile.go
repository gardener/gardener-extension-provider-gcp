// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/gardener"
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

	providerConfigPath := field.NewPath("spec.providerConfig")
	allErrs = append(allErrs, validation.ValidateCloudProfileConfig(providerConfig, machineImages, providerConfigPath)...)

	profileImages := gardener.NewCoreImagesContext(machineImages)
	parentImages := gardener.NewV1beta1ImagesContext(parentSpec.MachineImages)
	providerImages := validation.NewProviderImagesContext(providerConfig.MachineImages)

	for _, machineImage := range profileImages.Images {
		// Check that for each new image version defined in the NamespacedCloudProfile, the image is also defined in the providerConfig.
		_, existsInParent := parentImages.GetImage(machineImage.Name)
		if _, existsInProvider := providerImages.GetImage(machineImage.Name); !existsInParent && !existsInProvider {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages"),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile providerConfig", machineImage.Name),
			))
			continue
		}
		for _, version := range machineImage.Versions {
			_, existsInParent := parentImages.GetImageVersion(machineImage.Name, version.Version)
			for _, expectedArchitecture := range version.Architectures {
				if _, exists := providerImages.GetImageVersion(machineImage.Name, validation.VersionArchitectureKey(version.Version, expectedArchitecture)); !existsInParent && !exists {
					allErrs = append(allErrs, field.Required(
						field.NewPath("spec.providerConfig.machineImages"),
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
		if _, exists := profileImages.GetImage(machineImage.Name); !exists {
			allErrs = append(allErrs, field.Required(
				field.NewPath("spec.providerConfig.machineImages").Index(imageIdx),
				fmt.Sprintf("machine image %s is not defined in the NamespacedCloudProfile .spec.machineImages", machineImage.Name),
			))
			continue
		}
		for versionIdx, version := range machineImage.Versions {
			profileImageVersion, exists := profileImages.GetImageVersion(machineImage.Name, version.Version)
			if !exists {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec.providerConfig.machineImages").Index(imageIdx).Child("versions").Index(versionIdx),
					fmt.Sprintf("%s@%s", machineImage.Name, version.Version),
					"machine image version is not defined in the NamespacedCloudProfile",
				))
			}
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

	return allErrs
}
