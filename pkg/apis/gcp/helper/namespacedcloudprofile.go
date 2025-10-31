package helper

import (
	gardencoreapi "github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

// SimulateTransformToParentFormat simulates the transformation of the given NamespacedCloudProfile and its providerConfig
// to the parent CloudProfile format. This includes the transformation of both the providerConfig and the spec.
func SimulateTransformToParentFormat(cloudProfileConfig *api.CloudProfileConfig, cloudProfile *core.NamespacedCloudProfile, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) error {
	cloudProfileConfigV1alpha1 := &v1alpha1.CloudProfileConfig{}
	if err := Scheme.Convert(cloudProfileConfig, cloudProfileConfigV1alpha1, nil); err != nil {
		return field.InternalError(field.NewPath("spec.providerConfig"), err)
	}
	namespacedCloudProfileSpecV1beta1 := gardencorev1beta1.NamespacedCloudProfileSpec{}
	if err := gardencoreapi.Scheme.Convert(&cloudProfile.Spec, &namespacedCloudProfileSpecV1beta1, nil); err != nil {
		return field.InternalError(field.NewPath("spec"), err)
	}

	// simulate transformation to parent spec format
	// - performed in mutating extension webhook
	transformedSpecConfig := TransformProviderConfigToParentFormat(cloudProfileConfigV1alpha1, capabilityDefinitions)
	// - performed in namespaced cloud profile controller
	transformedSpec := gutil.TransformSpecToParentFormat(namespacedCloudProfileSpecV1beta1, capabilityDefinitions)

	if err := Scheme.Convert(transformedSpecConfig, cloudProfileConfig, nil); err != nil {
		return field.InternalError(field.NewPath("spec.providerConfig"), err)
	}
	if err := gardencoreapi.Scheme.Convert(&transformedSpec, &cloudProfile.Spec, nil); err != nil {
		return field.InternalError(field.NewPath("spec"), err)
	}
	return nil
}

// TransformProviderConfigToParentFormat supports the migration from the deprecated architecture fields to architecture capabilities.
// Depending on whether the parent CloudProfile is in capability format or not, it transforms the given config to
// the capability format or the deprecated architecture fields format respectively.
// It assumes that the given config is either completely in the capability format or in the deprecated architecture fields format.
func TransformProviderConfigToParentFormat(config *v1alpha1.CloudProfileConfig, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) *v1alpha1.CloudProfileConfig {
	if config == nil {
		return &v1alpha1.CloudProfileConfig{}
	}

	transformedConfig := v1alpha1.CloudProfileConfig{
		TypeMeta:      config.TypeMeta,
		MachineImages: transformMachineImages(config.MachineImages, capabilityDefinitions),
	}

	return &transformedConfig
}

func transformMachineImages(images []v1alpha1.MachineImages, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []v1alpha1.MachineImages {
	result := make([]v1alpha1.MachineImages, 0, len(images))

	for _, img := range images {
		result = append(result, v1alpha1.MachineImages{
			Name:     img.Name,
			Versions: transformImageVersions(img.Versions, capabilityDefinitions),
		})
	}

	return result
}

func transformImageVersions(versions []v1alpha1.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []v1alpha1.MachineImageVersion {
	var result []v1alpha1.MachineImageVersion

	if len(capabilityDefinitions) != 0 {
		versionMap := make(map[string][]v1alpha1.MachineImageVersion)
		for _, version := range versions {
			versionMap[version.Version] = append(versionMap[version.Version], version)
		}
		for _, groupedVersions := range versionMap {
			result = append(result, transformToCapabilityFormat(groupedVersions, capabilityDefinitions))
		}
	} else {
		for _, version := range versions {
			result = append(result, transformToLegacyFormat(version, capabilityDefinitions)...)
		}
	}
	return result
}

// transformToCapabilityFormat converts multiple versions with same version string to capability format
func transformToCapabilityFormat(versions []v1alpha1.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) v1alpha1.MachineImageVersion {
	transformedVersion := v1alpha1.MachineImageVersion{
		CapabilityFlavors: []v1alpha1.MachineImageFlavor{},
	}

	for _, version := range versions {
		transformedVersion.Version = version.Version

		if len(version.CapabilityFlavors) > 0 {
			// Already in capability format, return as-is
			transformedVersion.CapabilityFlavors = append(transformedVersion.CapabilityFlavors, version.CapabilityFlavors...)
		} else {
			capabilities := gardencorev1beta1.Capabilities{}
			for _, def := range capabilityDefinitions {
				if def.Name == v1beta1constants.ArchitectureName && len(def.Values) > 1 {
					arch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)
					capabilities[v1beta1constants.ArchitectureName] = []string{arch}
				}
			}
			capabilityFlavor := v1alpha1.MachineImageFlavor{
				Capabilities: capabilities,
				Image:        version.Image,
			}
			transformedVersion.CapabilityFlavors = append(transformedVersion.CapabilityFlavors, capabilityFlavor)
		}
	}
	return transformedVersion
}

// transformToLegacyFormat converts capability format to legacy format (regions with architecture)
func transformToLegacyFormat(version v1alpha1.MachineImageVersion, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) []v1alpha1.MachineImageVersion {
	if len(version.CapabilityFlavors) == 0 {
		// Already in legacy format, return as-is
		return []v1alpha1.MachineImageVersion{version}
	}

	var transformedVersions []v1alpha1.MachineImageVersion

	for _, flavor := range version.CapabilityFlavors {
		// Extract architecture from capabilities
		arch := getFirstArchitecture(flavor.Capabilities, capabilityDefinitions)
		transformedVersion := v1alpha1.MachineImageVersion{
			Version:      version.Version,
			Architecture: ptr.To(arch),
			Image:        flavor.Image,
		}
		transformedVersions = append(transformedVersions, transformedVersion)
	}
	return transformedVersions
}

// getFirstArchitecture extracts the first architecture from capabilities, defaults to "amd64"
func getFirstArchitecture(capabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition) string {
	defaultedCapabilities := capabilities
	if len(capabilityDefinitions) > 0 {
		defaultedCapabilities = gardencorev1beta1.GetCapabilitiesWithAppliedDefaults(capabilities, capabilityDefinitions)
	}

	if defaultedCapabilities == nil {
		return v1beta1constants.ArchitectureAMD64
	}

	archList, exists := defaultedCapabilities[v1beta1constants.ArchitectureName]
	if !exists || len(archList) == 0 {
		return v1beta1constants.ArchitectureAMD64
	}

	return archList[0]
}
