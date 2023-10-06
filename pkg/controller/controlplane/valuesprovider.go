// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/chart"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/charts"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/apihelper"
)

const (
	caNameControlPlane                   = "ca-" + gcp.Name + "-controlplane"
	cloudControllerManagerDeploymentName = "cloud-controller-manager"
	cloudControllerManagerServerName     = "cloud-controller-manager-server"
	csiSnapshotValidationServerName      = gcp.CSISnapshotValidationName + "-server"
)

func secretConfigsFunc(namespace string) []extensionssecretsmanager.SecretConfigWithOptions {
	return []extensionssecretsmanager.SecretConfigWithOptions{
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:       caNameControlPlane,
				CommonName: caNameControlPlane,
				CertType:   secretutils.CACert,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.Persist()},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:                        cloudControllerManagerServerName,
				CommonName:                  gcp.CloudControllerManagerName,
				DNSNames:                    kutil.DNSNamesForService(gcp.CloudControllerManagerName, namespace),
				CertType:                    secretutils.ServerCert,
				SkipPublishingCACertificate: true,
			},
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane)},
		},
		{
			Config: &secretutils.CertificateSecretConfig{
				Name:                        csiSnapshotValidationServerName,
				CommonName:                  gcp.UsernamePrefix + gcp.CSISnapshotValidationName,
				DNSNames:                    kutil.DNSNamesForService(gcp.CSISnapshotValidationName, namespace),
				CertType:                    secretutils.ServerCert,
				SkipPublishingCACertificate: true,
			},
			// use current CA for signing server cert to prevent mismatches when dropping the old CA from the webhook
			// config in phase Completing
			Options: []secretsmanager.GenerateOption{secretsmanager.SignedByCA(caNameControlPlane, secretsmanager.UseCurrentCA)},
		},
	}
}

func shootAccessSecretsFunc(namespace string) []*gutil.AccessSecret {
	return []*gutil.AccessSecret{
		gutil.NewShootAccessSecret(cloudControllerManagerDeploymentName, namespace),
		gutil.NewShootAccessSecret(gcp.CSIProvisionerName, namespace),
		gutil.NewShootAccessSecret(gcp.CSIAttacherName, namespace),
		gutil.NewShootAccessSecret(gcp.CSISnapshotterName, namespace),
		gutil.NewShootAccessSecret(gcp.CSIResizerName, namespace),
		gutil.NewShootAccessSecret(gcp.CSISnapshotControllerName, namespace),
		gutil.NewShootAccessSecret(gcp.CSISnapshotValidationName, namespace),
	}
}

var (
	configChart = &chart.Chart{
		Name:       "cloud-provider-config",
		EmbeddedFS: &charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "cloud-provider-config"),
		Objects: []*chart.Object{
			{Type: &corev1.ConfigMap{}, Name: internal.CloudProviderConfigName},
		},
	}

	controlPlaneChart = &chart.Chart{
		Name:       "seed-controlplane",
		EmbeddedFS: &charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "seed-controlplane"),
		SubCharts: []*chart.Chart{
			{
				Name:   gcp.CloudControllerManagerName,
				Images: []string{gcp.CloudControllerManagerImageName},
				Objects: []*chart.Object{
					{Type: &corev1.Service{}, Name: "cloud-controller-manager"},
					{Type: &appsv1.Deployment{}, Name: "cloud-controller-manager"},
					{Type: &corev1.ConfigMap{}, Name: "cloud-controller-manager-observability-config"},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: "cloud-controller-manager-vpa"},
				},
			},
			{
				Name: gcp.CSIControllerName,
				Images: []string{
					gcp.CSIDriverImageName,
					gcp.CSIProvisionerImageName,
					gcp.CSIAttacherImageName,
					gcp.CSISnapshotterImageName,
					gcp.CSIResizerImageName,
					gcp.CSILivenessProbeImageName,
					gcp.CSISnapshotControllerImageName,
					gcp.CSISnapshotValidationWebhookImageName,
				},
				Objects: []*chart.Object{
					// csi-driver-controller
					{Type: &appsv1.Deployment{}, Name: gcp.CSIControllerName},
					{Type: &corev1.ConfigMap{}, Name: gcp.CSIControllerConfigName},
					{Type: &corev1.ConfigMap{}, Name: gcp.CSIControllerObservabilityConfigName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: gcp.CSIControllerName + "-vpa"},
					// csi-snapshot-controller
					{Type: &appsv1.Deployment{}, Name: gcp.CSISnapshotControllerName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: gcp.CSISnapshotControllerName + "-vpa"},
					// csi-snapshot-validation-webhook
					{Type: &appsv1.Deployment{}, Name: gcp.CSISnapshotValidationName},
					{Type: &corev1.Service{}, Name: gcp.CSISnapshotValidationName},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name:       "shoot-system-components",
		EmbeddedFS: &charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-system-components"),
		SubCharts: []*chart.Chart{
			{
				Name: "cloud-controller-manager",
				Objects: []*chart.Object{
					{Type: &rbacv1.ClusterRole{}, Name: "system:controller:cloud-node-controller"},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: "system:controller:cloud-node-controller"},
					{Type: &rbacv1.ClusterRole{}, Name: "gce:cloud-provider"},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: "gce:cloud-provider"},
				},
			},
			{
				Name: gcp.CSINodeName,
				Images: []string{
					gcp.CSIDriverImageName,
					gcp.CSINodeDriverRegistrarImageName,
					gcp.CSILivenessProbeImageName,
				},
				Objects: []*chart.Object{
					// csi-driver
					{Type: &appsv1.DaemonSet{}, Name: gcp.CSINodeName},
					{Type: &storagev1.CSIDriver{}, Name: "pd.csi.storage.gke.io"},
					{Type: &corev1.ServiceAccount{}, Name: gcp.CSIDriverName},
					{Type: &rbacv1.ClusterRole{}, Name: gcp.UsernamePrefix + gcp.CSIDriverName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSIDriverName},
					{Type: &policyv1beta1.PodSecurityPolicy{}, Name: strings.Replace(gcp.UsernamePrefix+gcp.CSIDriverName, ":", ".", -1)},
					{Type: extensionscontroller.GetVerticalPodAutoscalerObject(), Name: gcp.CSINodeName},
					// csi-provisioner
					{Type: &rbacv1.ClusterRole{}, Name: gcp.UsernamePrefix + gcp.CSIProvisionerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSIProvisionerName},
					{Type: &rbacv1.Role{}, Name: gcp.UsernamePrefix + gcp.CSIProvisionerName},
					{Type: &rbacv1.RoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSIProvisionerName},
					// csi-attacher
					{Type: &rbacv1.ClusterRole{}, Name: gcp.UsernamePrefix + gcp.CSIAttacherName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSIAttacherName},
					{Type: &rbacv1.Role{}, Name: gcp.UsernamePrefix + gcp.CSIAttacherName},
					{Type: &rbacv1.RoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSIAttacherName},
					// csi-snapshot-controller
					{Type: &rbacv1.ClusterRole{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotControllerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotControllerName},
					{Type: &rbacv1.Role{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotControllerName},
					{Type: &rbacv1.RoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotControllerName},
					// csi-snapshotter
					{Type: &rbacv1.ClusterRole{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotterName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotterName},
					{Type: &rbacv1.Role{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotterName},
					{Type: &rbacv1.RoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotterName},
					// csi-resizer
					{Type: &rbacv1.ClusterRole{}, Name: gcp.UsernamePrefix + gcp.CSIResizerName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSIResizerName},
					{Type: &rbacv1.Role{}, Name: gcp.UsernamePrefix + gcp.CSIResizerName},
					{Type: &rbacv1.RoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSIResizerName},
					// csi-snapshot-validation-webhook
					{Type: &admissionregistrationv1.ValidatingWebhookConfiguration{}, Name: gcp.CSISnapshotValidationName},
					{Type: &rbacv1.ClusterRole{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotValidationName},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: gcp.UsernamePrefix + gcp.CSISnapshotValidationName},
				},
			},
		},
	}

	controlPlaneShootCRDsChart = &chart.Chart{
		Name:       "shoot-crds",
		EmbeddedFS: &charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-crds"),
		SubCharts: []*chart.Chart{
			{
				Name: "volumesnapshots",
				Objects: []*chart.Object{
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshotclasses.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshotcontents.snapshot.storage.k8s.io"},
					{Type: &apiextensionsv1.CustomResourceDefinition{}, Name: "volumesnapshots.snapshot.storage.k8s.io"},
				},
			},
		},
	}

	storageClassChart = &chart.Chart{
		Name:       "shoot-storageclasses",
		EmbeddedFS: &charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "shoot-storageclasses"),
	}
)

// NewValuesProvider creates a new ValuesProvider for the generic actuator.
func NewValuesProvider(mgr manager.Manager) genericactuator.ValuesProvider {
	return &valuesProvider{
		client:  mgr.GetClient(),
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

// valuesProvider is a ValuesProvider that provides GCP-specific values for the 2 charts applied by the generic actuator.
type valuesProvider struct {
	genericactuator.NoopValuesProvider
	client  client.Client
	decoder runtime.Decoder
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	_ *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisgcp.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// Decode infrastructureProviderStatus
	infraStatus := &apisgcp.InfrastructureStatus{}
	if _, _, err := vp.decoder.Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", kutil.ObjectName(cp), err)
	}

	// Get service account
	serviceAccount, err := gcp.GetServiceAccountFromSecretReference(ctx, vp.client, cp.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not get service account from secret '%s/%s': %w", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name, err)
	}

	// Get config chart values
	return getConfigChartValues(cpConfig, infraStatus, cp, serviceAccount)
}

// GetControlPlaneChartValues returns the values for the control plane chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
) (
	map[string]interface{},
	error,
) {
	cpConfig := &apisgcp.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", kutil.ObjectName(cp), err)
		}
	}

	// Get service account
	serviceAccount, err := gcp.GetServiceAccountFromSecretReference(ctx, vp.client, cp.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not get service account from secret '%s/%s': %w", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name, err)
	}

	// TODO(oliver-goetz): Delete this in a future release.
	if err := kutil.DeleteObject(ctx, vp.client, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-kube-apiserver-to-csi-snapshot-validation", Namespace: cp.Namespace}}); err != nil {
		return nil, fmt.Errorf("failed deleting legacy csi-snapshot-validation network policy: %w", err)
	}

	return vp.getControlPlaneChartValues(cpConfig, cp, cluster, secretsReader, serviceAccount, checksums, scaledDown)
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	_ context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	_ map[string]string,
) (
	map[string]interface{},
	error,
) {
	return getControlPlaneShootChartValues(cluster, cp, secretsReader)
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	infraStatus *apisgcp.InfrastructureStatus,
	cp *extensionsv1alpha1.ControlPlane,
	serviceAccount *gcp.ServiceAccount,
) (map[string]interface{}, error) {
	// Determine network names
	networkName, subNetworkName := getNetworkNames(infraStatus, cp)

	// Collect config chart values
	return map[string]interface{}{
		"projectID":      serviceAccount.ProjectID,
		"networkName":    networkName,
		"subNetworkName": subNetworkName,
		"zone":           cpConfig.Zone,
		"nodeTags":       cp.Namespace,
	}, nil
}

// getControlPlaneChartValues collects and returns the control plane chart values.
func (vp *valuesProvider) getControlPlaneChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	serviceAccount *gcp.ServiceAccount,
	checksums map[string]string,
	scaledDown bool,
) (
	map[string]interface{},
	error,
) {
	ccm, err := vp.getCCMChartValues(cpConfig, cp, cluster, secretsReader, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	csi, err := getCSIControllerChartValues(cpConfig, cp, cluster, secretsReader, serviceAccount, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"global": map[string]interface{}{
			"genericTokenKubeconfigSecretName": extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster),
		},
		gcp.CloudControllerManagerName: ccm,
		gcp.CSIControllerName:          csi,
	}, nil
}

// getCCMChartValues collects and returns the CCM chart values.
func (vp *valuesProvider) getCCMChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	serverSecret, found := secretsReader.Get(cloudControllerManagerServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", cloudControllerManagerServerName)
	}

	values := map[string]interface{}{
		"enabled":           true,
		"replicas":          extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"clusterName":       cp.Namespace,
		"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		"podNetwork":        extensionscontroller.GetPodNetwork(cluster),
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
			"checksum/configmap-" + internal.CloudProviderConfigName:      checksums[internal.CloudProviderConfigName],
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
		"tlsCipherSuites": kutil.TLSCipherSuites,
		"secrets": map[string]interface{}{
			"server": serverSecret.Name,
		},
	}

	if cpConfig.CloudControllerManager != nil {
		values["featureGates"] = cpConfig.CloudControllerManager.FeatureGates
	}

	ok, err := vp.isOverlayEnabled(cluster.Shoot.Spec.Networking)
	if err != nil {
		return nil, err
	}
	values["configureCloudRoutes"] = !ok

	return values, nil
}

// getCSIControllerChartValues collects and returns the CSIController chart values.
func getCSIControllerChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	serviceAccount *gcp.ServiceAccount,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {
	serverSecret, found := secretsReader.Get(csiSnapshotValidationServerName)
	if !found {
		return nil, fmt.Errorf("secret %q not found", csiSnapshotValidationServerName)
	}

	return map[string]interface{}{
		"enabled":   true,
		"replicas":  extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"projectID": serviceAccount.ProjectID,
		"zone":      cpConfig.Zone,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
		},
		"csiSnapshotController": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		},
		"csiSnapshotValidationWebhook": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
			"secrets": map[string]interface{}{
				"server": serverSecret.Name,
			},
			"topologyAwareRoutingEnabled": gardencorev1beta1helper.IsTopologyAwareRoutingForShootControlPlaneEnabled(cluster.Seed, cluster.Shoot),
		},
	}, nil
}

// getControlPlaneShootChartValues collects and returns the control plane shoot chart values.
func getControlPlaneShootChartValues(
	cluster *extensionscontroller.Cluster,
	cp *extensionsv1alpha1.ControlPlane,
	secretsReader secretsmanager.Reader,
) (map[string]interface{}, error) {
	kubernetesVersion := cluster.Shoot.Spec.Kubernetes.Version
	caSecret, found := secretsReader.Get(caNameControlPlane)
	if !found {
		return nil, fmt.Errorf("secret %q not found", caNameControlPlane)
	}

	return map[string]interface{}{
		gcp.CloudControllerManagerName: map[string]interface{}{"enabled": true},
		gcp.CSINodeName: map[string]interface{}{
			"enabled":           true,
			"kubernetesVersion": kubernetesVersion,
			"vpaEnabled":        gardencorev1beta1helper.ShootWantsVerticalPodAutoscaler(cluster.Shoot),
			"webhookConfig": map[string]interface{}{
				"url":      "https://" + gcp.CSISnapshotValidationName + "." + cp.Namespace + "/volumesnapshot",
				"caBundle": string(caSecret.Data[secretutils.DataKeyCertificateBundle]),
			},
			"pspDisabled": gardencorev1beta1helper.IsPSPDisabled(cluster.Shoot),
		},
	}, nil
}

// getNetworkNames determines the network and subnetwork names from the given infrastructure status and controlplane.
func getNetworkNames(
	infraStatus *apisgcp.InfrastructureStatus,
	cp *extensionsv1alpha1.ControlPlane,
) (string, string) {
	networkName := infraStatus.Networks.VPC.Name
	if networkName == "" {
		networkName = cp.Namespace
	}

	subNetworkName := ""
	subnet, _ := apihelper.FindSubnetForPurpose(infraStatus.Networks.Subnets, apisgcp.PurposeInternal)
	if subnet != nil {
		subNetworkName = subnet.Name
	}

	return networkName, subNetworkName
}

func (vp *valuesProvider) isOverlayEnabled(network *v1beta1.Networking) (bool, error) {
	if network == nil || network.ProviderConfig == nil {
		return true, nil
	}

	// should not happen in practice because we will receive a RawExtension with Raw populated in production.
	networkProviderConfig, err := network.ProviderConfig.MarshalJSON()
	if err != nil {
		return false, err
	}
	if string(networkProviderConfig) == "null" {
		return true, nil
	}
	var networkConfig map[string]interface{}
	if err := json.Unmarshal(networkProviderConfig, &networkConfig); err != nil {
		return false, err
	}
	if overlay, ok := networkConfig["overlay"].(map[string]interface{}); ok {
		return overlay["enabled"].(bool), nil
	}
	return true, nil
}
