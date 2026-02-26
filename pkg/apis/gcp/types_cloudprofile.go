// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudProfileConfig contains provider-specific configuration that is embedded into Gardener's `CloudProfile`
// resource.
type CloudProfileConfig struct {
	metav1.TypeMeta
	// MachineImages is the list of machine images that are understood by the controller. It maps
	// logical names and versions to provider-specific identifiers.
	MachineImages []MachineImages
}

// MachineImages is a mapping from logical names and versions to provider-specific identifiers.
type MachineImages struct {
	// Name is the logical name of the machine image.
	Name string
	// Versions contains versions and a provider-specific identifier.
	Versions []MachineImageVersion
}

// MachineImageVersion contains a version and a provider-specific identifier.
type MachineImageVersion struct {
	// Version is the version of the image.
	Version string
	// Image is the path to the image.
	Image string
	// Architecture is the CPU architecture of the machine image.
	Architecture *string
	// CapabilityFlavors is a collection of all images for that version with capabilities.
	CapabilityFlavors []MachineImageFlavor
}

// GetCapabilities returns the Capabilities of a MachineImageFlavor
func (cs MachineImageFlavor) GetCapabilities() gardencorev1beta1.Capabilities {
	return cs.Capabilities
}

// MachineImageFlavor is a flavor of the machine image version that supports a specific set of capabilities.
type MachineImageFlavor struct {
	// Capabilities is the set of capabilities that are supported by the AMIs in this set.
	Capabilities gardencorev1beta1.Capabilities
	// Image is the path to the image.
	Image string
}
