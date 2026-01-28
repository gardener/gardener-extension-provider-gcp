// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

// NewCloudProfileMutator returns a new instance of a CloudProfile mutator.
func NewCloudProfileMutator(mgr manager.Manager) extensionswebhook.Mutator {
	return &cloudProfile{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type cloudProfile struct {
	client  client.Client
	decoder runtime.Decoder
}

// Mutate mutates the given CloudProfile object.
func (p *cloudProfile) Mutate(_ context.Context, newObj, _ client.Object) error {
	profile, ok := newObj.(*gardencorev1beta1.CloudProfile)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	// Skip mutation if CloudProfile is being deleted or when no capabilities used in that profile
	if profile.DeletionTimestamp != nil || profile.Spec.ProviderConfig == nil || len(profile.Spec.MachineCapabilities) == 0 {
		return nil
	}

	specConfig := &v1alpha1.CloudProfileConfig{}
	if _, _, err := p.decoder.Decode(profile.Spec.ProviderConfig.Raw, nil, specConfig); err != nil {
		return fmt.Errorf("could not decode providerConfig of cloudProfile for '%s': %w", profile.Name, err)
	}

	overwriteMachineImageCapabilityFlavors(profile, specConfig)
	return nil
}

// overwriteMachineImageCapabilityFlavors updates the capability flavors of machine images in the CloudProfile
func overwriteMachineImageCapabilityFlavors(profile *gardencorev1beta1.CloudProfile, config *v1alpha1.CloudProfileConfig) {
	for _, providerMachineImage := range config.MachineImages {
		// Find the corresponding machine image in the CloudProfile
		imageIdx := slices.IndexFunc(profile.Spec.MachineImages, func(mi gardencorev1beta1.MachineImage) bool {
			return mi.Name == providerMachineImage.Name
		})
		if imageIdx == -1 {
			continue
		}

		// Group provider versions by version string (old format may have multiple entries per version)
		groupedVersions := helper.GroupV1alpha1VersionsByVersionString(providerMachineImage.Versions)

		for versionStr, providerVersions := range groupedVersions {
			// Find the corresponding version in the CloudProfile's machine image
			versionIdx := slices.IndexFunc(profile.Spec.MachineImages[imageIdx].Versions, func(miv gardencorev1beta1.MachineImageVersion) bool {
				return miv.Version == versionStr
			})
			if versionIdx == -1 {
				continue
			}

			// Check if any version entry uses new format (capabilityFlavors)
			// If so, use that; otherwise convert old format entries to capability flavors
			var capabilityFlavors []gardencorev1beta1.MachineImageFlavor
			for _, pv := range providerVersions {
				if len(pv.CapabilityFlavors) > 0 {
					// New format: use capabilityFlavors directly
					capabilityFlavors = convertCapabilityFlavors(pv.CapabilityFlavors)
					break
				}
			}

			if len(capabilityFlavors) == 0 {
				// Old format: convert all image+architecture entries to capability flavors
				capabilityFlavors = convertVersionsToCapabilityFlavors(providerVersions)
			}

			profile.Spec.MachineImages[imageIdx].Versions[versionIdx].CapabilityFlavors = capabilityFlavors
		}
	}
}

// convertVersionsToCapabilityFlavors converts old format (image with architecture) entries to capability flavors.
// It collects unique architectures from all version entries and creates a capability flavor for each.
// Note: A similar function exists in helper.go for internal API types that also preserves image references.
// This version only extracts unique architectures for CloudProfile spec mutation.
func convertVersionsToCapabilityFlavors(versions []v1alpha1.MachineImageVersion) []gardencorev1beta1.MachineImageFlavor {
	// Collect unique architectures from all version entries
	architectureSet := make(map[string]struct{})
	for _, version := range versions {
		if version.Image != "" {
			arch := ptr.Deref(version.Architecture, v1beta1constants.ArchitectureAMD64)
			architectureSet[arch] = struct{}{}
		}
	}

	// Create a capability flavor for each unique architecture
	capabilityFlavors := make([]gardencorev1beta1.MachineImageFlavor, 0, len(architectureSet))
	for arch := range architectureSet {
		capabilityFlavors = append(capabilityFlavors, gardencorev1beta1.MachineImageFlavor{
			Capabilities: gardencorev1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{arch},
			},
		})
	}

	// Sort for deterministic output
	slices.SortFunc(capabilityFlavors, func(a, b gardencorev1beta1.MachineImageFlavor) int {
		getArch := func(f gardencorev1beta1.MachineImageFlavor) string {
			if archList := f.Capabilities[v1beta1constants.ArchitectureName]; len(archList) > 0 {
				return archList[0]
			}
			return ""
		}
		return cmp.Compare(getArch(a), getArch(b))
	})

	return capabilityFlavors
}

// convertCapabilityFlavors converts provider capability flavors to CloudProfile capability flavors
func convertCapabilityFlavors(providerFlavors []v1alpha1.MachineImageFlavor) []gardencorev1beta1.MachineImageFlavor {
	capabilityFlavors := make([]gardencorev1beta1.MachineImageFlavor, 0, len(providerFlavors))
	for _, providerFlavor := range providerFlavors {
		capabilityFlavors = append(capabilityFlavors, gardencorev1beta1.MachineImageFlavor{
			Capabilities: providerFlavor.Capabilities,
		})
	}
	return capabilityFlavors
}
