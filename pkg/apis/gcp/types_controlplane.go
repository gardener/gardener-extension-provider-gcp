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
	// DefaultStorageClass controls which storage class is marked as default.
	// Allowed values: "default" (pd-balanced), "gce-sc-hdd" (pd-standard), "gce-sc-fast" (pd-ssd),
	// "gce-sc-hd-balanced", "gce-sc-hd-throughput", "gce-sc-hd-extreme".
	// If not set, the "default" (pd-balanced) storage class is marked as default (unless ManagedDefaultStorageClass is false).
	// If ManagedDefaultStorageClass is false, this field has no effect.
	DefaultStorageClass *string
	// ManagedDefaultVolumeSnapshotClass controls if the 'default' VolumeSnapshotClass would be marked as default.
	// Set to false to suppress marking the 'default' VolumeSnapshotClass as default, allowing another VolumeSnapshotClass
	// not managed by Gardener to be set as default by the user.
	// Defaults to true.
	ManagedDefaultVolumeSnapshotClass *bool
	// CSIFilestore contains configuration for CSI Filestore driver (support for NFS volumes)
	CSIFilestore *CSIFilestore
	// HyperDiskBalanced contains configuration for the hyperdisk-balanced StorageClass (gce-sc-hd-balanced).
	// The StorageClass is only deployed when Enabled is set to true.
	HyperDiskBalanced *HyperDiskConfig
	// HyperDiskThroughput contains configuration for the hyperdisk-throughput StorageClass (gce-sc-hd-throughput).
	// The StorageClass is only deployed when Enabled is set to true.
	HyperDiskThroughput *HyperDiskConfig
	// HyperDiskExtreme contains configuration for the hyperdisk-extreme StorageClass (gce-sc-hd-extreme).
	// The StorageClass is only deployed when Enabled is set to true.
	HyperDiskExtreme *HyperDiskConfig
}

// HyperDiskConfig contains configuration for a hyperdisk StorageClass.
type HyperDiskConfig struct {
	// Enabled controls whether this hyperdisk StorageClass is deployed.
	// When true, the required performance parameters for the disk type must be provided.
	Enabled bool
	// ProvisionedIopsOnCreate sets the provisioned-iops-on-create StorageClass parameter.
	// Supported for hyperdisk-balanced and hyperdisk-extreme. Required when Enabled is true for those types.
	ProvisionedIopsOnCreate *int64
	// ProvisionedThroughputOnCreate sets the provisioned-throughput-on-create StorageClass parameter.
	// Supported for hyperdisk-balanced and hyperdisk-throughput. Required when Enabled is true for those types.
	// Value must be a valid quantity string (e.g. "140Mi").
	ProvisionedThroughputOnCreate *string
}

// CSIFilestore contains configuration for CSI Filestore driver
type CSIFilestore struct {
	// Enabled is the switch to enable the CSI Manila driver support
	Enabled bool
}
