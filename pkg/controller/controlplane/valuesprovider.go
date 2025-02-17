// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionssecretsmanager "github.com/gardener/gardener/extensions/pkg/util/secret/manager"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/chart"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	storagev1 "k8s.io/api/storage/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	autoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
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
	}
}

var (
	configChart = &chart.Chart{
		Name:       "cloud-provider-config",
		EmbeddedFS: charts.InternalChart,
		Path:       filepath.Join(charts.InternalChartsPath, "cloud-provider-config"),
		Objects: []*chart.Object{
			{Type: &corev1.ConfigMap{}, Name: internal.CloudProviderConfigName},
		},
	}

	controlPlaneChart = &chart.Chart{
		Name:       "seed-controlplane",
		EmbeddedFS: charts.InternalChart,
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
					{Type: &monitoringv1.ServiceMonitor{}, Name: "shoot-cloud-controller-manager"},
					{Type: &monitoringv1.PrometheusRule{}, Name: "shoot-cloud-controller-manager"},
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
				},
				Objects: []*chart.Object{
					// csi-driver-controller
					{Type: &appsv1.Deployment{}, Name: gcp.CSIControllerName},
					{Type: &corev1.ConfigMap{}, Name: gcp.CSIControllerConfigName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: gcp.CSIControllerName + "-vpa"},
					// csi-snapshot-controller
					{Type: &appsv1.Deployment{}, Name: gcp.CSISnapshotControllerName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: gcp.CSISnapshotControllerName + "-vpa"},
				},
			},
			{
				Name: gcp.IngressGCEName,
				Images: []string{
					gcp.IngressGCEImageName,
				},
				Objects: []*chart.Object{
					{Type: &appsv1.Deployment{}, Name: gcp.IngressGCEName},
					{Type: &autoscalingv1.VerticalPodAutoscaler{}, Name: gcp.IngressGCEName + "-vpa"},
					{Type: &corev1.ServiceAccount{}, Name: "glbc"},
				},
			},
		},
	}

	controlPlaneShootChart = &chart.Chart{
		Name:       "shoot-system-components",
		EmbeddedFS: charts.InternalChart,
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
				},
			},
			{
				Name: "default-http-backend",
				Images: []string{
					gcp.DefaultHTTPBackendImageName,
				},
				Objects: []*chart.Object{
					{Type: &appsv1.Deployment{}, Name: "l7-default-backend"},
					{Type: &corev1.Service{}, Name: "default-http-backend"},
					{Type: &corev1.ServiceAccount{}, Name: "glbc"},
					{Type: &rbacv1.Role{}, Name: "system:controller:glbc"},
					{Type: &rbacv1.RoleBinding{}, Name: "system:controller:glbc"},
					{Type: &rbacv1.ClusterRole{}, Name: "system:controller:glbc"},
					{Type: &rbacv1.ClusterRoleBinding{}, Name: "system:controller:glbc"},
				},
			},
		},
	}

	controlPlaneShootCRDsChart = &chart.Chart{
		Name:       "shoot-crds",
		EmbeddedFS: charts.InternalChart,
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
		EmbeddedFS: charts.InternalChart,
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
	client  k8sclient.Client
	decoder runtime.Decoder
}

// GetConfigChartValues returns the values for the config chart applied by the generic actuator.
func (vp *valuesProvider) GetConfigChartValues(
	ctx context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	// Decode providerConfig
	cpConfig := &apisgcp.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	// Decode infrastructureProviderStatus
	infraStatus := &apisgcp.InfrastructureStatus{}
	if _, _, err := vp.decoder.Decode(cp.Spec.InfrastructureProviderStatus.Raw, nil, infraStatus); err != nil {
		return nil, fmt.Errorf("could not decode infrastructureProviderStatus of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
	}

	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, vp.client, cp.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not get credentials config from secret '%s/%s': %w", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name, err)
	}

	// Get config chart values
	return getConfigChartValues(cpConfig, infraStatus, cp, credentialsConfig, isDualstackEnabled(cluster.Shoot.Spec.Networking))
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
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	// Get credentials configuration
	credentialsConfig, err := gcp.GetCredentialsConfigFromSecretReference(ctx, vp.client, cp.Spec.SecretRef)
	if err != nil {
		return nil, fmt.Errorf("could not get service account from secret '%s/%s': %w", cp.Spec.SecretRef.Namespace, cp.Spec.SecretRef.Name, err)
	}

	// TODO(rfranzke): Delete this in a future release.
	if err := kutil.DeleteObject(ctx, vp.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "csi-driver-controller-observability-config", Namespace: cp.Namespace}}); err != nil {
		return nil, fmt.Errorf("failed deleting legacy csi-driver-controller-observability-config ConfigMap: %w", err)
	}

	// TODO(rfranzke): Delete this after August 2024.
	gep19Monitoring := vp.client.Get(ctx, k8sclient.ObjectKey{Name: "prometheus-shoot", Namespace: cp.Namespace}, &appsv1.StatefulSet{}) == nil
	if gep19Monitoring {
		if err := kutil.DeleteObject(ctx, vp.client, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-observability-config", Namespace: cp.Namespace}}); err != nil {
			return nil, fmt.Errorf("failed deleting cloud-controller-manager-observability-config ConfigMap: %w", err)
		}
	}

	return vp.getControlPlaneChartValues(cpConfig, cp, cluster, secretsReader, credentialsConfig, checksums, scaledDown, gep19Monitoring)
}

// GetControlPlaneShootChartValues returns the values for the control plane shoot chart applied by the generic actuator.
func (vp *valuesProvider) GetControlPlaneShootChartValues(
	_ context.Context,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	_ secretsmanager.Reader,
	_ map[string]string,
) (
	map[string]interface{},
	error,
) {
	return map[string]interface{}{
		gcp.CloudControllerManagerName: map[string]interface{}{"enabled": true},
		gcp.CSINodeName: map[string]interface{}{
			"enabled":           true,
			"kubernetesVersion": cluster.Shoot.Spec.Kubernetes.Version,
		},
	}, nil
}

// getConfigChartValues collects and returns the configuration chart values.
func getConfigChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	infraStatus *apisgcp.InfrastructureStatus,
	cp *extensionsv1alpha1.ControlPlane,
	credentialsConfig *gcp.CredentialsConfig,
	dualStack bool,
) (map[string]interface{}, error) {
	// Determine network names
	networkName, subNetworkName := getNetworkNames(infraStatus, cp, dualStack)

	// Collect config chart values
	return map[string]interface{}{
		"projectID":      credentialsConfig.ProjectID,
		"networkName":    networkName,
		"subNetworkName": subNetworkName,
		"zone":           cpConfig.Zone,
		"nodeTags":       cp.Namespace,
	}, nil
}

func shouldUseWorkloadIdentity(credentialsConfig *gcp.CredentialsConfig) bool {
	return credentialsConfig.Type == gcp.ExternalAccountCredentialType && len(credentialsConfig.TokenFilePath) > 0
}

// getControlPlaneChartValues collects and returns the control plane chart values.
func (vp *valuesProvider) getControlPlaneChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	credentialsConfig *gcp.CredentialsConfig,
	checksums map[string]string,
	scaledDown bool,
	gep19Monitoring bool,
) (
	map[string]interface{},
	error,
) {
	ccm, err := vp.getCCMChartValues(cpConfig, cp, cluster, secretsReader, checksums, scaledDown, gep19Monitoring, shouldUseWorkloadIdentity(credentialsConfig))
	if err != nil {
		return nil, err
	}

	csi, err := getCSIControllerChartValues(cpConfig, cp, cluster, secretsReader, credentialsConfig, checksums, scaledDown)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"global": map[string]interface{}{
			"genericTokenKubeconfigSecretName": extensionscontroller.GenericTokenKubeconfigSecretNameFromCluster(cluster),
		},
		gcp.CloudControllerManagerName: ccm,
		gcp.CSIControllerName:          csi,
		gcp.IngressGCEName: map[string]interface{}{
			"enabled":  isDualstackEnabled(cluster.Shoot.Spec.Networking),
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		},
	}, nil
}

func isDualstackEnabled(networking *gardencorev1beta1.Networking) bool {
	if networking != nil {
		return !gardencorev1beta1.IsIPv4SingleStack(networking.IPFamilies)
	}

	return false
}

// getCCMChartValues collects and returns the CCM chart values.
func (vp *valuesProvider) getCCMChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	secretsReader secretsmanager.Reader,
	checksums map[string]string,
	scaledDown bool,
	gep19Monitoring bool,
	useWorkloadIdentity bool,
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
		"podNetwork":        strings.Join(extensionscontroller.GetPodNetwork(cluster), ","),
		"allocatorType":     "RangeAllocator",
		"serviceNetwork":    strings.Join(extensionscontroller.GetServiceNetwork(cluster), ","),
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
		"gep19Monitoring":     gep19Monitoring,
		"useWorkloadIdentity": useWorkloadIdentity,
	}

	if cpConfig.CloudControllerManager != nil {
		values["featureGates"] = cpConfig.CloudControllerManager.FeatureGates
	}

	overlayEnabled, err := vp.isOverlayEnabled(cluster.Shoot.Spec.Networking)
	if err != nil {
		return nil, err
	}
	values["configureCloudRoutes"] = !overlayEnabled

	if isDualstackEnabled(cluster.Shoot.Spec.Networking) {
		values["configureCloudRoutes"] = false
		values["allocatorType"] = "CloudAllocator"
	}

	if cluster.Shoot.Spec.Kubernetes.KubeControllerManager != nil && cluster.Shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize != nil {
		if len(cluster.Shoot.Spec.Networking.IPFamilies) == 1 && cluster.Shoot.Spec.Networking.IPFamilies[0] == v1beta1.IPFamilyIPv4 {
			values["nodeCIDRMaskSizeIPv4"] = *cluster.Shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize
		}
	}

	return values, nil
}

// getCSIControllerChartValues collects and returns the CSIController chart values.
func getCSIControllerChartValues(
	cpConfig *apisgcp.ControlPlaneConfig,
	_ *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
	_ secretsmanager.Reader,
	credentialsConfig *gcp.CredentialsConfig,
	checksums map[string]string,
	scaledDown bool,
) (map[string]interface{}, error) {

	values := map[string]interface{}{
		"enabled":   true,
		"replicas":  extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		"projectID": credentialsConfig.ProjectID,
		"zone":      cpConfig.Zone,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
		},
		"csiSnapshotController": map[string]interface{}{
			"replicas": extensionscontroller.GetControlPlaneReplicas(cluster, scaledDown, 1),
		},
		"useWorkloadIdentity": shouldUseWorkloadIdentity(credentialsConfig),
	}

	k8sVersion, err := semver.NewVersion(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}
	if versionutils.ConstraintK8sGreaterEqual131.Check(k8sVersion) {
		if _, ok := cluster.Shoot.Annotations[gcp.AnnotationEnableVolumeAttributesClass]; ok {
			values["csiDriver"] = map[string]interface{}{
				"storage": map[string]interface{}{
					"supportsDynamicIopsProvisioning":       []string{"hyperdisk-balanced", "hyperdisk-extreme"},
					"supportsDynamicThroughputProvisioning": []string{"hyperdisk-balanced", "hyperdisk-throughput", "hyperdisk-ml"},
				},
			}
			values["csiResizer"] = map[string]interface{}{
				"featureGates": map[string]string{
					"VolumeAttributesClass": "true",
				},
			}
			values["csiProvisioner"] = map[string]interface{}{
				"featureGates": map[string]string{
					"VolumeAttributesClass": "true",
				},
			}
		}
	}

	return values, nil
}

// getStorageClassChartValues collects and returns the shoot storage-class chart values.
func (vp *valuesProvider) GetStorageClassesChartValues(
	_ context.Context,
	cp *extensionsv1alpha1.ControlPlane,
	_ *extensionscontroller.Cluster,
) (map[string]interface{}, error) {
	managedDefaultStorageClass := true
	managedDefaultVolumeSnapshotClass := true

	// Decode providerConfig
	cpConfig := &apisgcp.ControlPlaneConfig{}
	if cp.Spec.ProviderConfig != nil {
		if _, _, err := vp.decoder.Decode(cp.Spec.ProviderConfig.Raw, nil, cpConfig); err != nil {
			return nil, fmt.Errorf("could not decode providerConfig of controlplane '%s': %w", k8sclient.ObjectKeyFromObject(cp), err)
		}
	}

	if cpConfig.Storage != nil {
		managedDefaultStorageClass = ptr.Deref(cpConfig.Storage.ManagedDefaultStorageClass, true)
		managedDefaultVolumeSnapshotClass = ptr.Deref(cpConfig.Storage.ManagedDefaultVolumeSnapshotClass, true)
	}

	return map[string]interface{}{
		"managedDefaultStorageClass":        managedDefaultStorageClass,
		"managedDefaultVolumeSnapshotClass": managedDefaultVolumeSnapshotClass,
	}, nil
}

// getNetworkNames determines the network and subnetwork names from the given infrastructure status and controlplane.
func getNetworkNames(
	infraStatus *apisgcp.InfrastructureStatus,
	cp *extensionsv1alpha1.ControlPlane,
	dualstack bool,
) (string, string) {
	networkName := infraStatus.Networks.VPC.Name
	if networkName == "" {
		networkName = cp.Namespace
	}

	subnetPurpose := apisgcp.PurposeInternal
	if dualstack {
		subnetPurpose = apisgcp.PurposeNodes
	}

	subNetworkName := ""
	subnet, _ := apihelper.FindSubnetForPurpose(infraStatus.Networks.Subnets, subnetPurpose)
	if subnet != nil {
		subNetworkName = subnet.Name
	}

	return networkName, subNetworkName
}

func (vp *valuesProvider) isOverlayEnabled(network *gardencorev1beta1.Networking) (bool, error) {
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
