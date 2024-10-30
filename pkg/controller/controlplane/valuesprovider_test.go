// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		ctrl *gomock.Controller
		ctx  = context.TODO()

		scheme = runtime.NewScheme()
		_      = apisgcp.AddToScheme(scheme)

		vp  genericactuator.ValuesProvider
		c   *mockclient.MockClient
		mgr *mockmanager.MockManager

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
							},
						},
					}),
				},
			},
		}

		cidr = "10.250.0.0/19"

		projectID = "abc"
		zone      = "europe-west1a"

		cpSecretKey = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		cpSecret    = &corev1.Secret{
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

		enabledTrue = map[string]interface{}{"enabled": true}

		fakeClient         client.Client
		fakeSecretsManager secretsmanager.Interface

		cluster *extensionscontroller.Cluster
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c)
		mgr.EXPECT().GetScheme().Return(scheme)
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
						Pods: &cidr,
					},
					Kubernetes: gardencorev1beta1.Kubernetes{
						Version: "1.28.2",
					},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetConfigChartValues", func() {
		It("should return correct config chart values", func() {
			c.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			values, err := vp.GetConfigChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"projectID":      projectID,
				"networkName":    "vpc-1234",
				"subNetworkName": "subnet-acbd1234",
				"zone":           zone,
				"nodeTags":       namespace,
			}))
		})
	})

	Describe("#GetControlPlaneChartValues", func() {
		ccmChartValues := utils.MergeMaps(enabledTrue, map[string]interface{}{
			"replicas":    1,
			"clusterName": namespace,
			"podNetwork":  cidr,
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
			c.EXPECT().Get(context.TODO(), cpSecretKey, &corev1.Secret{}).DoAndReturn(clientGet(cpSecret))

			By("creating secrets managed outside of this package for whose secretsmanager.Get() will be called")
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-provider-gcp-controlplane", Namespace: namespace}})).To(Succeed())
			Expect(fakeClient.Create(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "cloud-controller-manager-server", Namespace: namespace}})).To(Succeed())

			c.EXPECT().Delete(context.TODO(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "csi-driver-controller-observability-config", Namespace: namespace}})
			c.EXPECT().Get(context.TODO(), client.ObjectKey{Name: "prometheus-shoot", Namespace: cp.Namespace}, gomock.AssignableToTypeOf(&appsv1.StatefulSet{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))
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
					"gep19Monitoring":     false,
					"useWorkloadIdentity": false,
				}),
				gcp.CSIControllerName: utils.MergeMaps(enabledTrue, map[string]interface{}{
					"replicas":  1,
					"projectID": projectID,
					"zone":      zone,
					"podAnnotations": map[string]interface{}{
						"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: checksums[v1beta1constants.SecretNameCloudProvider],
					},
					"csiSnapshotController": map[string]interface{}{
						"replicas": 1,
					},
					"useWorkloadIdentity": false,
				}),
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
				"gep19Monitoring":      false,
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
				"gep19Monitoring":      false,
			})))
		})

		DescribeTable("topologyAwareRoutingEnabled value",
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

			Entry("seed setting is nil, shoot control plane is not HA",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is disabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is enabled, shoot control plane is not HA",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}},
				&gardencorev1beta1.ControlPlane{HighAvailability: nil},
			),
			Entry("seed setting is nil, shoot control plane is HA with failure tolerance type 'zone'",
				nil,
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry("seed setting is disabled, shoot control plane is HA with failure tolerance type 'zone'",
				&gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}},
				&gardencorev1beta1.ControlPlane{HighAvailability: &gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{Type: gardencorev1beta1.FailureToleranceTypeZone}}},
			),
			Entry("seed setting is enabled, shoot control plane is HA with failure tolerance type 'zone'",
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
					"kubernetesVersion": "1.28.2",
				}),
			}))
		})
	})
	Describe("#GetStorageClassesChartValues()", func() {
		It("should return correct storage class chart values when using managed classes", func() {
			values, err := vp.GetStorageClassesChartValues(ctx, cp, cluster)
			Expect(err).NotTo(HaveOccurred())
			Expect(values).To(Equal(map[string]interface{}{
				"managedDefaultStorageClass":        true,
				"managedDefaultVolumeSnapshotClass": true,
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
				"managedDefaultStorageClass":        false,
				"managedDefaultVolumeSnapshotClass": false,
			}))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func clientGet(result runtime.Object) interface{} {
	return func(_ context.Context, _ client.ObjectKey, obj runtime.Object, _ ...client.GetOption) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		}
		return nil
	}
}
