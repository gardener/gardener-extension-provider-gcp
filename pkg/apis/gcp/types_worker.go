// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkerConfig contains configuration settings for the worker nodes.
type WorkerConfig struct {
	metav1.TypeMeta

	// GPU contains configuration for the GPU attached to VMs.
	GPU *GPU

	// Volume contains configuration for the root disks attached to VMs.
	Volume *Volume

	// DataVolumes contains configuration for the additional disks attached to VMs.
	// +optional
	DataVolumes []DataVolume

	// MinCpuPlatform is the name of the minimum CPU platform that is to be
	// requested for the VM.
	MinCpuPlatform *string

	// Service account, with their specified scopes, authorized for this worker.
	// Service accounts generate access tokens that can be accessed through
	// the metadata server and used to authenticate applications on the
	// instance.
	// This service account should be created in advance.
	ServiceAccount *ServiceAccount

	// NodeTemplate contains resource information of the machine which is used by Cluster Autoscaler to generate nodeTemplate during scaling a nodeGroup from zero.
	NodeTemplate *extensionsv1alpha1.NodeTemplate
}

// Volume contains configuration for the additional disks attached to VMs.
type Volume struct {
	// LocalSSDInterface is the interface of that the local ssd disk supports.
	LocalSSDInterface *string

	// Encryption refers to the disk encryption details for this volume
	Encryption *DiskEncryption

	// ProvisionedIops of disk to create.
	// Only for use with disk of types like pd-extreme and hyperdisk-extreme.
	// The IOPS must be specified within defined limits.
	// If not set gcp calculates a default value taking the disk size into consideration.
	ProvisionedIops *int64 `json:"provisionedIops"`

	// ProvisionedThroughput of disk to create.
	// Only for hyperdisk-balanced or hyperdisk-throughput volumes,
	// measured in MiB per second, that the disk can handle.
	// The throughput must be specified within defined limits.
	// If not set gcp calculates a default value taking the disk size into consideration.
	ProvisionedThroughput *int64 `json:"provisionedThroughput"`
}

// DataVolume contains configuration for data volumes attached to VMs.
type DataVolume struct {
	// Name is the name of the data volume this configuration applies to.
	Name string

	// SourceImage is the image to create this disk
	// However, this parameter should only be used with particular caution.
	// For example GardenLinux works with filesystem LABELs only and creating
	// another disk form the very same image causes the LABELs to be duplicated.
	// See: https://github.com/gardener/gardener-extension-provider-gcp/issues/323
	SourceImage *string
}

// DiskEncryption encapsulates the encryption configuration for a disk.
type DiskEncryption struct {
	// KmsKeyName specifies the customer-managed encryption key (CMEK) used for encryption of the volume.
	// For creating keys, see https://cloud.google.com/kms/docs/create-key.
	// For using keys to encrypt resources, see:
	// https://cloud.google.com/compute/docs/disks/customer-managed-encryption#encrypt_a_new_persistent_disk_with_your_own_keys
	// This field is being kept optional since this would allow CSEK fields in future in lieu of CMEK fields
	KmsKeyName *string

	// KmsKeyServiceAccount specifies the service account granted the `roles/cloudkms.cryptoKeyEncrypterDecrypter` for the key name.
	// If nil/empty, then the role should be given to the Compute Engine Service Agent Account. The CESA usually has the format
	// service-PROJECT_NUMBER@compute-system.iam.gserviceaccount.com.
	//  See: https://cloud.google.com/iam/docs/service-agents#compute-engine-service-agent
	// One can add IAM roles using the gcloud CLI:
	//  gcloud projects add-iam-policy-binding projectId --member
	//	serviceAccount:name@projectIdgserviceaccount.com --role roles/cloudkms.cryptoKeyEncrypterDecrypter
	KmsKeyServiceAccount *string
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WorkerStatus contains information about created worker resources.
type WorkerStatus struct {
	metav1.TypeMeta

	// MachineImages is a list of machine images that have been used in this worker. Usually, the extension controller
	// gets the mapping from name/version to the provider-specific machine image data in its componentconfig. However, if
	// a version that is still in use gets removed from this componentconfig it cannot reconcile anymore existing `Worker`
	// resources that are still using this version. Hence, it stores the used versions in the provider status to ensure
	// reconciliation is possible.
	MachineImages []MachineImage
}

// GPU is the configuration of the GPU to be attached
type GPU struct {
	// AcceleratorType is the type of accelerator to be attached
	AcceleratorType string
	// Count is the number of accelerator to be attached
	Count int32
}

// MachineImage is a mapping from logical names and versions to GCP-specific identifiers.
type MachineImage struct {
	// Name is the logical name of the machine image.
	Name string
	// Version is the logical version of the machine image.
	Version string
	// Image is the path to the image.
	Image string
	// Architecture is the CPU architecture of the machine image.
	Architecture *string
}

// ServiceAccount is a GCP service account.
type ServiceAccount struct {
	// Email is the email address of the service account.
	Email string

	// Scopes is the list of scopes to be made available for this service.
	// account.
	Scopes []string
}
