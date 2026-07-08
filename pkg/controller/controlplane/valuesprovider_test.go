// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"encoding/json"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"
)

const (
	namespace                        = "test"
	genericTokenKubeconfigSecretName = "generic-token-kubeconfig-92e9ae14"
)

var _ = Describe("ValuesProvider", func() {
	var (
		ctx = context.TODO()

		scheme = runtime.NewScheme()
		_      = apisgcp.AddToScheme(scheme)
		_      = corev1.AddToScheme(scheme)
		_      = appsv1.AddToScheme(scheme)
		_      = policyv1.AddToScheme(scheme)
		_      = vpaautoscalingv1.AddToScheme(scheme)

		vp  genericactuator.ValuesProvider
		mgr *test.FakeManager

		cp = &extensionsv1alpha1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "control-plane",
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.ControlPlaneSpec{
				SecretRef: corev1.SecretReference{
					Name:      v1beta1constants.SecretNameCloudProvider,
					Namespace: namespace,
				},
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisgcp.ControlPlaneConfig{
							TypeMeta: metav1.TypeMeta{
								Kind:       "ControlPlaneConfig",
								APIVersion: apisgcp.SchemeGroupVersion.String(),
							},
							Zone: "europe-west1a",
							CloudControllerManager: &apisgcp.CloudControllerManagerConfig{
								FeatureGates: map[string]bool{
									"SomeKubernetesFeature": true,
								},
							},
							Storage: &apisgcp.Storage{
								ManagedDefaultStorageClass:        ptr.To(true),
								ManagedDefaultVolumeSnapshotClass: ptr.To(true),
							},
						}),
					},
				},
				InfrastructureProviderStatus: &runtime.RawExtension{
					Raw: encode(&apisgcp.InfrastructureStatus{
						Networks: apisgcp.NetworkStatus{
							VPC: apisgcp.VPC{
								Name: "vpc-1234",
							},
							Subnets: []apisgcp.Subnet{
								{
									Name:    "subnet-acbd1234",
									Purpose: apisgcp.PurposeInternal,
								},
								{
									Name:    "subnet-nodes1234",
									Purpose: apisgcp.PurposeNodes,
								},
							},
						},
					}),
				},
			},
		}

		cidr = "10.250.0.0/19"

		projectID = "abc"
		zone      = "europe-west1a"

		cpSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.SecretNameCloudProvider,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				gcp.ServiceAccountJSONField: []byte(`{"project_id":"` + projectID + `"}`),
			},
		}

		checksums = map[string]string{
			v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			internal.CloudProviderConfigName:         "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
		}

		// Checksums computed locally from the same input values that render each configmap.
		// See computeIngressGCECloudProviderConfigChecksum and computeCSICloudProviderConfigChecksum.
		ingressGCEConfigChecksum = "61bfad39fdd7c5ad86b28bcb6355c8150f3aa2e23a82b4c44c902265efecbe82"
		csiConfigChecksum        = "9275b3ff5c8701e655c58605973e10cbe2656479f034488044a2ea625cabebed"

		enabledTrue = map[string]interface{}{"enabled": true}

		fakeClient         client.Client
		fakeSecretsManager secretsmanager.Interface

		cluster *extensionscontroller.Cluster
	)

	BeforeEach(func() {
		c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(cpSecret).Build()

		mgr = &test.FakeManager{Client: c, Scheme: scheme}
		vp = NewValuesProvider(mgr)

		fakeClient = fakeclient.NewClientBuilder().Build()
		fakeSecretsManager = fakesecretsmanager.New(fakeClient, namespace)

		cluster = &extensionscontroller.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"generic-token-kubeconfig.secret.gardener.cloud/name": genericTokenKubeconfigSecretName,
				},
			},
			Seed: &gardencorev1beta1.Seed{},
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{
							{
								Name: "worker",
							},
						},
					},
					Networking: &gardencorev1beta1.Networking{
						IPFamilies: []gardencorev1beta1.IPFamily{
							gardencorev1beta1.IPFamilyIPv4,
						},
						Pods:     &cidr,
						Services: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.30.14",
					},
				},
			},
		}
	})

	Describe("#GetConfigChartValues", func() {
		It("should return correct config chart values", func() {
			values, err := vp.GetConfigChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"projectID":           projectID,
				"networkName":         "vpc-1234",
				"subNetworkName":      "subnet-acbd1234",
				"subNetworkNameNodes": "subnet-nodes1234",
				"zone":                zone,
				"nodeTags":            namespace,
			}))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		ccmChartValues := utils.MergeMaps(enabledTrue, map[string]interface{}{
			"replicas":       1,
			"clusterName":    namespace,
			"podNetwork":     cidr,
			"serviceNetwork": cidr,
			"podAnnotations": map[string]interface{}{
				"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
				"checksum/configmap-" + internal.CloudProviderConfigName:      "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
			},
			"podLabels": map[string]interface{}{
				"maintenance.gardener.cloud/restart": "true",
			},
			"featureGates": map[string]bool{
				"SomeKubernetesFeature": true,
			},
			"tlsCipherSuites": []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305",
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305",
			},
			"secrets": map[string]interface{}{
				"server": "cloud-controller-manager-server",
			},
			"configureCloudRoutes": false,
		})

		BeforeEach(func() {
			By("creating secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-provider-gcp-controlplane", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-server", Namespace: namespace}})).To(Succeed())
		})

		It("should return correct control plane chart values", func() {
			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				gcp.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion":   cluster.Shoot.Spec.Kubernetes.Version,
					"allocatorType":       "RangeAllocator",
					"useWorkloadIdentity": false,
				}),
				gcp.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + gcp.CSIControllerConfigName:           csiConfigChecksum,
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
					"useWorkloadIdentity": false,
					"enableDataCache":     false,
				}),
				gcp.CSIFilestoreControllerName: map[string]interface{}{
					"enabled":   false,
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + gcp.CSIFilestoreControllerConfigName:  csiConfigChecksum,
					},
					"useWorkloadIdentity": false,
				},
				gcp.IngressGCEName: map[string]interface{}{
					"enabled":             false,
					"replicas":            0,
					"useWorkloadIdentity": false,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider:      checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + internal.CloudProviderConfigIngressGCEName: ingressGCEConfigChecksum,
					},
				},
			}))
		})

		It("should return correct control plane chart values (dualstack)", func() {
			dualstackShoot := cluster.Shoot.DeepCopy()
			dualstackShoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{
				gardencorev1beta1.IPFamilyIPv4,
				gardencorev1beta1.IPFamilyIPv6,
			}
			dualstackShoot.Status.Networking = &gardencorev1beta1.NetworkingStatus{
				Nodes: []string{"<node-ipv4>", "<node-ipv6>"},
			}
			cluster.Shoot = dualstackShoot
			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				gcp.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion":   cluster.Shoot.Spec.Kubernetes.Version,
					"allocatorType":       "CloudAllocator",
					"useWorkloadIdentity": false,
				}),
				gcp.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + gcp.CSIControllerConfigName:           csiConfigChecksum,
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
					"useWorkloadIdentity": false,
					"enableDataCache":     false,
				}),
				gcp.CSIFilestoreControllerName: map[string]interface{}{
					"enabled":   false,
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + gcp.CSIFilestoreControllerConfigName:  csiConfigChecksum,
					},
					"useWorkloadIdentity": false,
				},
				gcp.IngressGCEName: map[string]interface{}{
					"enabled":             true,
					"replicas":            1,
					"useWorkloadIdentity": false,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider:      checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + internal.CloudProviderConfigIngressGCEName: ingressGCEConfigChecksum,
					},
				},
			}))
		})

		It("should return correct control plane chart values (post migration from dualstack)", func() {
			dualstackShoot := cluster.Shoot.DeepCopy()
			dualstackShoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{
				gardencorev1beta1.IPFamilyIPv4,
			}
			dualstackShoot.Status.Networking = &gardencorev1beta1.NetworkingStatus{
				Nodes: []string{"<node-ipv4>", "<node-ipv6>"},
			}
			cluster.Shoot = dualstackShoot
			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"global": map[string]interface{}{
					"genericTokenKubeconfigSecretName": genericTokenKubeconfigSecretName,
				},
				gcp.CloudControllerManagerName: utils.MergeMaps(ccmChartValues, map[string]interface{}{
					"kubernetesVersion":   cluster.Shoot.Spec.Kubernetes.Version,
					"allocatorType":       "CloudAllocator",
					"useWorkloadIdentity": false,
				}),
				gcp.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + gcp.CSIControllerConfigName:           csiConfigChecksum,
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
					"useWorkloadIdentity": false,
					"enableDataCache":     false,
				}),
				gcp.CSIFilestoreControllerName: map[string]interface{}{
					"enabled":   false,
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + gcp.CSIFilestoreControllerConfigName:  csiConfigChecksum,
					},
					"useWorkloadIdentity": false,
				},
				gcp.IngressGCEName: map[string]interface{}{
					"enabled":             true,
					"replicas":            0,
					"useWorkloadIdentity": false,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider:      checksums[v1beta1constants.SecretNameCloudProvider],
						"checksum/configmap-" + internal.CloudProviderConfigIngressGCEName: ingressGCEConfigChecksum,
					},
				},
			}))
		})

		It("should return correct control plane chart values for clusters without overlay", func() {
			shootWithoutOverlay := cluster.Shoot.DeepCopy()
			shootWithoutOverlay.Spec.Networking.Type = ptr.To("calico")
			shootWithoutOverlay.Spec.Networking.ProviderConfig = &runtime.RawExtension{
				Raw: []byte(`{"overlay":{"enabled":false}}`),
			}
			cluster.Shoot = shootWithoutOverlay
			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values[gcp.CloudControllerManagerName]).To(Equal(utils.MergeMaps(ccmChartValues, map[string]interface{}{
				"kubernetesVersion":    cluster.Shoot.Spec.Kubernetes.Version,
				"configureCloudRoutes": true,
				"allocatorType":        "RangeAllocator",
				"useWorkloadIdentity":  false,
			})))
		})

		It("should return correct control plane chart values for clusters with custom node-cidr-mask-size", func() {
			shootWithNodeCMS := cluster.Shoot.DeepCopy()
			shootWithNodeCMS.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{
				gardencorev1beta1.IPFamilyIPv4,
			}
			shootWithNodeCMS.Spec.Kubernetes.KubeControllerManager = &gardencorev1beta1.KubeControllerManagerConfig{
				NodeCIDRMaskSize: ptr.To(int32(22)),
			}
			cluster.Shoot = shootWithNodeCMS
			values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(values[gcp.CloudControllerManagerName]).To(Equal(utils.MergeMaps(ccmChartValues, map[string]interface{}{
				"kubernetesVersion":    cluster.Shoot.Spec.Kubernetes.Version,
				"nodeCIDRMaskSizeIPv4": int32(22),
				"allocatorType":        "RangeAllocator",
				"useWorkloadIdentity":  false,
			})))
		})

		DescribeTable(
			"topologyAwareRoutingEnabled value",
			func(seedSettings *gardencorev1beta1.SeedSettings, shootControlPlane *gardencorev1beta1.ControlPlane) {
				cluster.Seed = &gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Settings: seedSettings,
					},
				}
				cluster.Shoot.Spec.ControlPlane = shootControlPlane

				values, err := vp.GetControlPlaneChartValues(ctx, cp, cluster, fakeSecretsManager, checksums, false)
				Expect(err).NotTo(HaveOccurred())
				Expect(values).To(HaveKey(gcp.CSIControllerName))
			},

			Entry(
				"seed setting is nil, shoot control plane is not HA",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry(
				"seed setting is disabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry(
				"seed setting is enabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry(
				"seed setting is nil, shoot control plane is HA with failure tolerance type 'zone'",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry(
				"seed setting is disabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry(
				"seed setting is enabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}},
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
		)
	})

	Describe("#GetControlPlaneShootChartValues", func() {
		BeforeEach(func() {
			By("creating secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-provider-gcp-controlplane", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-server", Namespace: namespace}})).To(Succeed())
		})

		It("should return correct shoot control plane chart values", func() {
			values, err := vp.GetControlPlaneShootChartValues(ctx, cp, cluster, fakeSecretsManager, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				gcp.CloudControllerManagerName: enabledTrue,
				gcp.CSINodeName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"kubernetesVersion": "1.30.14",
					"enabled":           true,
					"enableDataCache":   false,
				}),
				gcp.CSIFilestoreNodeName: map[string]interface{}{
					"enabled": false,
				},
				"default-http-backend": map[string]interface{}{
					"enabled": IsDualStackEnabled(cluster.Shoot.Spec.Networking, cluster.Shoot.Status.Networking),
				},
				"calico-mutating-admission-policy": map[string]interface{}{
					"enabled": false,
				},
			}))
		})
	})
	Describe("#GetStorageClassesChartValues()", func() {
		It("should return correct storage class chart values when using managed classes", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"defaultStorageClass":               "default",
				"managedDefaultVolumeSnapshotClass": true,
				"filestore": map[string]interface{}{
					"enabled": false,
					"network": "vpc-1234",
				},
				"hyperdisk": map[string]interface{}{
					"balanced":   map[string]interface{}(nil),
					"throughput": map[string]interface{}(nil),
					"extreme":    map[string]interface{}(nil),
				},
			}))
		})

		It("should return correct storage class chart values when not using managed classes", func() {
			cp.Spec.ProviderConfig.Raw = encode(&apisgcp.ControlPlaneConfig{
				Storage: &apisgcp.Storage{
					ManagedDefaultStorageClass:        ptr.To(false),
					ManagedDefaultVolumeSnapshotClass: ptr.To(false),
				},
			})

			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"defaultStorageClass":               "",
				"managedDefaultVolumeSnapshotClass": false,
				"filestore": map[string]interface{}{
					"enabled": false,
					"network": "vpc-1234",
				},
				"hyperdisk": map[string]interface{}{
					"balanced":   map[string]interface{}(nil),
					"throughput": map[string]interface{}(nil),
					"extreme":    map[string]interface{}(nil),
				},
			}))
		})

		It("should return correct storage class chart values when using hyperdisk-balanced as default", func() {
			cp.Spec.ProviderConfig.Raw = encode(&apisgcp.ControlPlaneConfig{
				Storage: &apisgcp.Storage{
					DefaultStorageClass: ptr.To("gce-sc-hd-balanced"),
				},
			})

			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"defaultStorageClass":               "gce-sc-hd-balanced",
				"managedDefaultVolumeSnapshotClass": true,
				"filestore": map[string]interface{}{
					"enabled": false,
					"network": "vpc-1234",
				},
				"hyperdisk": map[string]interface{}{
					"balanced":   map[string]interface{}(nil),
					"throughput": map[string]interface{}(nil),
					"extreme":    map[string]interface{}(nil),
				},
			}))
		})

		It("should ignore DefaultStorageClass when ManagedDefaultStorageClass is false", func() {
			cp.Spec.ProviderConfig.Raw = encode(&apisgcp.ControlPlaneConfig{
				Storage: &apisgcp.Storage{
					ManagedDefaultStorageClass: ptr.To(false),
					DefaultStorageClass:        ptr.To("gce-sc-hd-balanced"),
				},
			})

			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"defaultStorageClass":               "",
				"managedDefaultVolumeSnapshotClass": true,
				"filestore": map[string]interface{}{
					"enabled": false,
					"network": "vpc-1234",
				},
				"hyperdisk": map[string]interface{}{
					"balanced":   map[string]interface{}(nil),
					"throughput": map[string]interface{}(nil),
					"extreme":    map[string]interface{}(nil),
				},
			}))
		})

		It("should return hyperdisk parameters in storage class chart values", func() {
			cp.Spec.ProviderConfig.Raw = encode(&apisgcp.ControlPlaneConfig{
				Storage: &apisgcp.Storage{
					HyperDiskBalanced: &apisgcp.HyperDiskConfig{
						Enabled:                       true,
						ProvisionedIopsOnCreate:       ptr.To[int64](3000),
						ProvisionedThroughputOnCreate: ptr.To("140Mi"),
					},
					HyperDiskThroughput: &apisgcp.HyperDiskConfig{
						Enabled:                       true,
						ProvisionedThroughputOnCreate: ptr.To("250Mi"),
					},
					HyperDiskExtreme: &apisgcp.HyperDiskConfig{
						Enabled:                 true,
						ProvisionedIopsOnCreate: ptr.To[int64](10000),
					},
				},
			})

			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"defaultStorageClass":               "default",
				"managedDefaultVolumeSnapshotClass": true,
				"filestore": map[string]interface{}{
					"enabled": false,
					"network": "vpc-1234",
				},
				"hyperdisk": map[string]interface{}{
					"balanced": map[string]interface{}{
						"provisionedIopsOnCreate":       int64(3000),
						"provisionedThroughputOnCreate": "140Mi",
					},
					"throughput": map[string]interface{}{
						"provisionedThroughputOnCreate": "250Mi",
					},
					"extreme": map[string]interface{}{
						"provisionedIopsOnCreate": int64(10000),
					},
				},
			}))
		})
	})

	Describe("#isMutatingAdmissionPolicyEnabled", func() {
		var testCluster *extensionscontroller.Cluster

		BeforeEach(func() {
			calico := "calico"
			testCluster = &extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Networking: &gardencorev1beta1.Networking{
							Type: &calico,
						},
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.33.0",
						},
					},
				},
			}
		})

		It("should return false if KubeAPIServer is nil", func() {
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false if feature gates are nil", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false if MutatingAdmissionPolicy feature gate is not set", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"SomeOtherGate": true},
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false if MutatingAdmissionPolicy feature gate is disabled", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": false},
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false if RuntimeConfig is nil", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false if neither v1alpha1 nor v1beta1 is enabled in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
				RuntimeConfig: map[string]bool{"some.other/v1": true},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return true if feature gate is enabled and v1alpha1 is in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
				RuntimeConfig: map[string]bool{"admissionregistration.k8s.io/v1alpha1": true},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeTrue())
		})

		It("should return false for K8s < 1.34 if feature gate is enabled and only v1beta1 is in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
				RuntimeConfig: map[string]bool{"admissionregistration.k8s.io/v1beta1": true},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return true for K8s < 1.34 if feature gate is enabled and both v1alpha1 and v1beta1 are in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
				RuntimeConfig: map[string]bool{
					"admissionregistration.k8s.io/v1alpha1": true,
					"admissionregistration.k8s.io/v1beta1":  true,
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeTrue())
		})

		It("should return false for K8s >= 1.34 and < 1.36 without any feature gate or RuntimeConfig (beta, disabled by default)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.34.0"
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false for K8s 1.35 without KubeAPIServer config (beta, disabled by default)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.35.0"
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false for K8s >= 1.34 and < 1.36 if feature gate is not set", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.34.0"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"SomeOtherGate": true},
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false for K8s >= 1.34 and < 1.36 if feature gate is explicitly disabled", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.34.0"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": false},
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false for K8s >= 1.34 and < 1.36 if feature gate is enabled but RuntimeConfig is nil", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.34.0"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return false for K8s >= 1.34 and < 1.36 if feature gate is enabled but v1beta1 is not in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.35.0"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
				RuntimeConfig: map[string]bool{"some.other/v1": true},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeFalse())
		})

		It("should return true for K8s >= 1.34 and < 1.36 if feature gate is enabled and v1beta1 is in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.34.0"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
				RuntimeConfig: map[string]bool{"admissionregistration.k8s.io/v1beta1": true},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeTrue())
		})

		It("should return true for K8s 1.35 if feature gate is enabled and v1beta1 is in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.35.0"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": true},
				},
				RuntimeConfig: map[string]bool{"admissionregistration.k8s.io/v1beta1": true},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeTrue())
		})

		It("should return true for K8s >= 1.36 even if feature gate is explicitly disabled (GA, locked on)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.36.0"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				KubernetesConfig: gardencorev1beta1.KubernetesConfig{
					FeatureGates: map[string]bool{"MutatingAdmissionPolicy": false},
				},
			}
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeTrue())
		})

		It("should return true for K8s >= 1.36 without any feature gate or RuntimeConfig (GA)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.36.0"
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeTrue())
		})

		It("should return true for K8s >= 1.36 even without KubeAPIServer config (GA)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.37.1"
			Expect(isMutatingAdmissionPolicyEnabled(testCluster)).To(BeTrue())
		})
	})

	Describe("#mutatingAdmissionPolicyAPIVersion", func() {
		var testCluster *extensionscontroller.Cluster

		BeforeEach(func() {
			testCluster = &extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.33.0",
						},
					},
				},
			}
		})

		It("should return v1alpha1 if no RuntimeConfig is set (< 1.34)", func() {
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1alpha1"))
		})

		It("should return v1alpha1 if only v1alpha1 is in RuntimeConfig (< 1.34)", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				RuntimeConfig: map[string]bool{"admissionregistration.k8s.io/v1alpha1": true},
			}
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1alpha1"))
		})

		It("should return v1alpha1 if v1beta1 is in RuntimeConfig (< 1.34, v1beta1 not supported)", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				RuntimeConfig: map[string]bool{"admissionregistration.k8s.io/v1beta1": true},
			}
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1alpha1"))
		})

		It("should return v1alpha1 if both v1alpha1 and v1beta1 are in RuntimeConfig (< 1.34)", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				RuntimeConfig: map[string]bool{
					"admissionregistration.k8s.io/v1alpha1": true,
					"admissionregistration.k8s.io/v1beta1":  true,
				},
			}
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1alpha1"))
		})

		It("should return v1alpha1 if v1beta1 is explicitly disabled in RuntimeConfig (< 1.34)", func() {
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				RuntimeConfig: map[string]bool{
					"admissionregistration.k8s.io/v1alpha1": true,
					"admissionregistration.k8s.io/v1beta1":  false,
				},
			}
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1alpha1"))
		})

		It("should return v1beta1 for K8s >= 1.34 (beta)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.34.0"
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1beta1"))
		})

		It("should return v1beta1 for K8s 1.35 (beta)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.35.0"
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1beta1"))
		})

		It("should return v1 for K8s >= 1.36 (GA)", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.36.0"
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1"))
		})

		It("should return v1 for K8s 1.36 even if v1beta1 is in RuntimeConfig", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.36.2"
			testCluster.Shoot.Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
				RuntimeConfig: map[string]bool{"admissionregistration.k8s.io/v1beta1": true},
			}
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1"))
		})

		It("should return v1 for K8s versions higher than 1.36", func() {
			testCluster.Shoot.Spec.Kubernetes.Version = "1.38.0"
			Expect(mutatingAdmissionPolicyAPIVersion(testCluster)).To(Equal("v1"))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
