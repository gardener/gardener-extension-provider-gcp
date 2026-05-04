// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	// DefaultStorageClass controls which storage class is marked as default.
	// Allowed values: "default" (pd-balanced), "gce-sc-hdd" (pd-standard), "gce-sc-fast" (pd-ssd),
	// "gce-sc-hd-balanced", "gce-sc-hd-throughput", "gce-sc-hd-extreme".
	// If not set, the "default" (pd-balanced) storage class is marked as default (unless ManagedDefaultStorageClass is false).
	// If ManagedDefaultStorageClass is false, this field has no effect.
	// +optional
	DefaultStorageClass *string `json:"defaultStorageClass,omitempty"`
	// ManagedDefaultVolumeSnapshotClass controls if the 'default' VolumeSnapshotClass would be marked as default.
	// Set to false to suppress marking the 'default' VolumeSnapshotClass as default, allowing another VolumeSnapshotClass
	// not managed by Gardener to be set as default by the user.
	// Defaults to true.
	// +optional
	ManagedDefaultVolumeSnapshotClass *bool `json:"managedDefaultVolumeSnapshotClass,omitempty"`
	// CSIFilestore contains configuration for CSI Filestore driver (support for NFS volumes)
	// +optional
	CSIFilestore *CSIFilestore `json:"csiFilestore,omitempty"`
	// HyperDiskBalanced contains configuration for the hyperdisk-balanced StorageClass (gce-sc-hd-balanced).
	// +optional
	HyperDiskBalanced *HyperDiskConfig `json:"hyperDiskBalanced,omitempty"`
	// HyperDiskThroughput contains configuration for the hyperdisk-throughput StorageClass (gce-sc-hd-throughput).
	// +optional
	HyperDiskThroughput *HyperDiskConfig `json:"hyperDiskThroughput,omitempty"`
	// HyperDiskExtreme contains configuration for the hyperdisk-extreme StorageClass (gce-sc-hd-extreme).
	// +optional
	HyperDiskExtreme *HyperDiskConfig `json:"hyperDiskExtreme,omitempty"`
}

// HyperDiskConfig contains performance parameters for a hyperdisk StorageClass.
type HyperDiskConfig struct {
	// ProvisionedIopsOnCreate sets the provisioned-iops-on-create StorageClass parameter.
	// Supported for hyperdisk-balanced and hyperdisk-extreme.
	// +optional
	ProvisionedIopsOnCreate *int64 `json:"provisionedIopsOnCreate,omitempty"`
	// ProvisionedThroughputOnCreate sets the provisioned-throughput-on-create StorageClass parameter.
	// Supported for hyperdisk-balanced and hyperdisk-throughput.
	// Value must be a valid quantity string (e.g. "140Mi").
	// +optional
	ProvisionedThroughputOnCreate *string `json:"provisionedThroughputOnCreate,omitempty"`
}

// CSIFilestore contains configuration for CSI Filestore driver
type CSIFilestore struct {
	// Enabled is the switch to enable the CSI Manila driver support
	Enabled bool `json:"enabled"`
}
