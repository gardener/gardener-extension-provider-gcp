// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneConfig contains configuration settings for the control plane.
type ControlPlaneConfig struct {
	metav1.TypeMeta `json:",inline"`

	// Zone is the GCP zone.
	Zone string `json:"zone"`

	// CloudControllerManager contains configuration settings for the cloud-controller-manager.
	// +optional
	CloudControllerManager *CloudControllerManagerConfig `json:"cloudControllerManager,omitempty"`

	// Storage contains configuration for the storage in the cluster.
	Storage *Storage `json:"storage,omitempty"`
}

// CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
type CloudControllerManagerConfig struct {
	// FeatureGates contains information about enabled feature gates.
	// +optional
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// Storage contains settings for the default StorageClass and VolumeSnapshotClass
type Storage struct {
	// ManagedDefaultStorageClass controls if the 'default' StorageClass would be marked as default. Set to false to
	// suppress marking the 'default' StorageClass as default, allowing another StorageClass not managed by Gardener
	// to be set as default by the user.
	// Defaults to true.
	// +optional
	ManagedDefaultStorageClass *bool `json:"managedDefaultStorageClass,omitempty"`
	// ManagedDefaultVolumeSnapshotClass controls if the 'default' VolumeSnapshotClass would be marked as default.
	// Set to false to suppress marking the 'default' VolumeSnapshotClass as default, allowing another VolumeSnapshotClass
	// not managed by Gardener to be set as default by the user.
	// Defaults to true.
	// +optional
	ManagedDefaultVolumeSnapshotClass *bool `json:"managedDefaultVolumeSnapshotClass,omitempty"`
}
