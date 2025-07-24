// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ControlPlaneConfig contains configuration settings for the control plane.
type ControlPlaneConfig struct {
	metav1.TypeMeta

	// Zone is the GCP zone.
	Zone string

	// CloudControllerManager contains configuration settings for the cloud-controller-manager.
	CloudControllerManager *CloudControllerManagerConfig

	// Storage contains configuration for the storage in the cluster.
	Storage *Storage
}

// CloudControllerManagerConfig contains configuration settings for the cloud-controller-manager.
type CloudControllerManagerConfig struct {
	// FeatureGates contains information about enabled feature gates.
	FeatureGates map[string]bool
}

// Storage contains settings for the default StorageClass and VolumeSnapshotClass
type Storage struct {
	// ManagedDefaultStorageClass controls if the 'default' StorageClass would be marked as default. Set to false to
	// suppress marking the 'default' StorageClass as default, allowing another StorageClass not managed by Gardener
	// to be set as default by the user.
	// Defaults to true.
	ManagedDefaultStorageClass *bool
	// ManagedDefaultVolumeSnapshotClass controls if the 'default' VolumeSnapshotClass would be marked as default.
	// Set to false to suppress marking the 'default' VolumeSnapshotClass as default, allowing another VolumeSnapshotClass
	// not managed by Gardener to be set as default by the user.
	// Defaults to true.
	ManagedDefaultVolumeSnapshotClass *bool
	// CSIFilestore contains configuration for CSI Filestore driver (support for NFS volumes)
	CSIFilestore *CSIFilestore
}

// CSIFilestore contains configuration for CSI Filestore driver
type CSIFilestore struct {
	// Enabled is the switch to enable the CSI Manila driver support
	Enabled bool
}
