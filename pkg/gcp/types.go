// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	// IngressGCEImageName is the name of the ingress-gce image.
	IngressGCEImageName = "ingress-gce"
	// DefaultHTTPBackendImageName is the name of the default-http-backend image.
	DefaultHTTPBackendImageName = "ingress-default-backend"
	// CSIDriverImageName is the name of the csi-driver image.
	CSIDriverImageName = "csi-driver"
	// CSIFilestoreDriverImageName is the name of the csi-filestore-driver image.
	CSIFilestoreDriverImageName = "csi-driver-filestore"
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
	// IngressGCEName is a constant for the name of the ingress-gce deployment in the seed.
	IngressGCEName = "ingress-gce"
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
	// CSIFilestoreControllerConfigName is a constant for the name of the CSI filestore controller config in the seed.
	CSIFilestoreControllerConfigName = "csi-filestore-controller-config"
	// CSIFilestoreControllerName is a constant for the name of the CSI filestore controller deployment in the seed.
	CSIFilestoreControllerName = "csi-driver-filestore-controller"
	// CSIFilestoreNodeName is a constant for the name of the CSI node deployment in the shoot.
	CSIFilestoreNodeName = "csi-driver-filestore-node"

	// AnnotationEnableVolumeAttributesClass is the annotation to use on shoots to enable VolumeAttributesClasses
	AnnotationEnableVolumeAttributesClass = "gcp.provider.extensions.gardener.cloud/enable-volume-attributes-class"

	// WorkloadIdentityMountPath is the path where the workload identity token and GCP config file are usually mounted.
	WorkloadIdentityMountPath = "/var/run/secrets/gardener.cloud/workload-identity"

	// CSISnapshotValidationName is the constant for the name of the csi-snapshot-validation-webhook component.
	// TODO(AndreasBurger): Clean up once SnapshotValidation is removed everywhere
	CSISnapshotValidationName = "csi-snapshot-validation"
)

// UsernamePrefix is a constant for the username prefix of components deployed by GCP.
var UsernamePrefix = extensionsv1alpha1.SchemeGroupVersion.Group + ":" + Name + ":"
