// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// Name is the name of the GCP provider.
	Name = "provider-gcp"

	// CloudControllerManagerImageName is the name of the cloud-controller-manager image.
	CloudControllerManagerImageName = "cloud-controller-manager"
	// CSIDriverImageName is the name of the csi-driver image.
	CSIDriverImageName = "csi-driver"
	// CSIProvisionerImageName is the name of the csi-provisioner image.
	CSIProvisionerImageName = "csi-provisioner"
	// CSIAttacherImageName is the name of the csi-attacher image.
	CSIAttacherImageName = "csi-attacher"
	// CSIDiskDriverTopologyKey is the label on persistent volumes that represents availability by zone.
	CSIDiskDriverTopologyKey = "topology.gke.io/zone"
	// CSISnapshotterImageName is the name of the csi-snapshotter image.
	CSISnapshotterImageName = "csi-snapshotter"
	// CSIResizerImageName is the name of the csi-resizer image.
	CSIResizerImageName = "csi-resizer"
	// CSISnapshotControllerImageName is the name of the csi-snapshot-controller image.
	CSISnapshotControllerImageName = "csi-snapshot-controller"
	// CSINodeDriverRegistrarImageName is the name of the csi-node-driver-registrar image.
	CSINodeDriverRegistrarImageName = "csi-node-driver-registrar"
	// CSILivenessProbeImageName is the name of the csi-liveness-probe image.
	CSILivenessProbeImageName = "csi-liveness-probe"
	// MachineControllerManagerProviderGCPImageName is the name of the MachineController GCP image.
	MachineControllerManagerProviderGCPImageName = "machine-controller-manager-provider-gcp"

	// ServiceAccountJSONField is the field in a secret where the service account JSON is stored at.
	ServiceAccountJSONField = "serviceaccount.json"
	// CredentialsConfigField is the field in a secret where the credentials config JSON is stored at.
	CredentialsConfigField = "credentialsConfig"

	// ServiceAccountCredentialType is the type of the credentials contained in the serviceaccount.json file.
	ServiceAccountCredentialType = "service_account"
	// ExternalAccountCredentialType is the type of the credentials contained in the credentialsConfig file.
	ExternalAccountCredentialType = "external_account"

	// CloudControllerManagerName is a constant for the name of the CloudController deployed by the worker controller.
	CloudControllerManagerName = "cloud-controller-manager"
	// CSIControllerName is a constant for the name of the CSI controller deployment in the seed.
	CSIControllerName = "csi-driver-controller"
	// CSIControllerConfigName is a constant for the name of the CSI controller config in the seed.
	CSIControllerConfigName = "csi-driver-controller-config"
	// CSINodeName is a constant for the name of the CSI node deployment in the shoot.
	CSINodeName = "csi-driver-node"
	// CSIDriverName is a constant for the name of the csi-driver component.
	CSIDriverName = "csi-driver"
	// CSIProvisionerName is a constant for the name of the csi-provisioner component.
	CSIProvisionerName = "csi-provisioner"
	// CSIAttacherName is a constant for the name of the csi-attacher component.
	CSIAttacherName = "csi-attacher"
	// CSISnapshotterName is a constant for the name of the csi-snapshotter component.
	CSISnapshotterName = "csi-snapshotter"
	// CSIResizerName is a constant for the name of the csi-resizer component.
	CSIResizerName = "csi-resizer"
	// CSISnapshotControllerName is a constant for the name of the csi-snapshot-controller component.
	CSISnapshotControllerName = "csi-snapshot-controller"
	// CSINodeDriverRegistrarName is a constant for the name of the csi-node-driver-registrar component.
	CSINodeDriverRegistrarName = "csi-node-driver-registrar"
	// CSILivenessProbeName is a constant for the name of the csi-liveness-probe component.
	CSILivenessProbeName = "csi-liveness-probe"

	// GlobalAnnotationKeyUseFlow marks how the infrastructure should be reconciled. When this is used reconciliation with flow
	// will take place. Otherwrise, Terraformer will be used.
	GlobalAnnotationKeyUseFlow = "provider.extensions.gardener.cloud/use-flow"
	// AnnotationKeyUseFlow marks how the infrastructure should be reconciled. When this is used reconciliation with flow
	// will take place. Otherwrise, Terraformer will be used.
	AnnotationKeyUseFlow = "gcp." + GlobalAnnotationKeyUseFlow
	// SeedAnnotationKeyUseFlow is the label for seeds to enable flow reconciliation for all of its shoots if value is `true`
	// or for new shoots only with value `new`
	SeedAnnotationKeyUseFlow = AnnotationKeyUseFlow
	// SeedAnnotationUseFlowValueNew is the value to restrict flow reconciliation to new shoot clusters
	SeedAnnotationUseFlowValueNew = "new"
	// AnnotationEnableVolumeAttributesClass is the annotation to use on shoots to enable VolumeAttributesClasses
	AnnotationEnableVolumeAttributesClass = "gcp.provider.extensions.gardener.cloud/enable-volume-attributes-class"

	// WorkloadIdentityMountPath is the path where the workload identity token and GCP config file are usually mounted.
	WorkloadIdentityMountPath = "/var/run/secrets/gardener.cloud/workload-identity"

	// CSISnapshotValidationName is the constant for the name of the csi-snapshot-validation-webhook component.
	// TODO(AndreasBurger): Clean up once SnapshotValidation is removed everywhere
	CSISnapshotValidationName = "csi-snapshot-validation"
)

var (
	// UsernamePrefix is a constant for the username prefix of components deployed by GCP.
	UsernamePrefix = extensionsv1alpha1.SchemeGroupVersion.Group + ":" + Name + ":"
)
