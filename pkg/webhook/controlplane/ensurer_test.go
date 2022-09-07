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
	"testing"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"

	"github.com/Masterminds/semver"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils/version"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

const namespace = "test"

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctrl *gomock.Controller
		ctx  = context.TODO()

		dummyContext   = gcontext.NewGardenContext(nil, nil)
		eContextK8s117 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.17.0",
						},
					},
				},
			},
		)
		eContextK8s118 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.18.0",
						},
					},
				},
			},
		)
		eContextK8s118WithCSIAnnotation = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						csimigration.AnnotationKeyNeedsComplete: "true",
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.18.0",
						},
					},
				},
			},
		)
		eContextK8s120WithCSIAnnotation = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						csimigration.AnnotationKeyNeedsComplete: "true",
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.20.0",
						},
					},
				},
			},
		)
		eContextK8s121 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.21.0",
						},
					},
				},
			},
		)
		eContextK8s121WithCSIAnnotation = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						csimigration.AnnotationKeyNeedsComplete: "true",
					},
				},
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.21.0",
						},
					},
				},
			},
		)

		secretKey = client.ObjectKey{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider}
		secret    = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.SecretNameCloudProvider},
			Data:       map[string][]byte{"foo": []byte("bar")},
		}

		cmKey = client.ObjectKey{Namespace: namespace, Name: internal.CloudProviderConfigName}
		cm    = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: internal.CloudProviderConfigName},
			Data:       map[string]string{"abc": "xyz"},
		}

		annotations = map[string]string{
			"checksum/secret-" + v1beta1constants.SecretNameCloudProvider: "8bafb35ff1ac60275d62e1cbd495aceb511fb354f74a20f7d06ecb48b3a68432",
			"checksum/configmap-" + internal.CloudProviderConfigName:      "08a7bc7fe8f59b055f173145e211760a83f02cf89635cef26ebb351378635606",
		}

		kubeControllerManagerLabels = map[string]string{
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureKubeAPIServerDeployment", func() {
		var (
			client  *mockclient.MockClient
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-apiserver",
								},
							},
						},
					},
				},
			}
			client = mockclient.NewMockClient(ctrl)

			ensurer = NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should add missing elements to kube-apiserver deployment (k8s = 1.17)", func() {
			client.EXPECT().Get(ctx, secretKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))
			client.EXPECT().Get(ctx, cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.17.0", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.18)", func() {
			client.EXPECT().Get(ctx, secretKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))
			client.EXPECT().Get(ctx, cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s118, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, annotations, "1.18.0", false)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.18 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s118WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, nil, "1.18.0", true)
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.21 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, nil, "1.21.0", true)
		})

		It("should modify existing elements of kube-apiserver deployment", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeAPIServer},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-apiserver",
										Command: []string{
											"--cloud-provider=?",
											"--cloud-config=?",
											"--enable-admission-plugins=Priority,NamespaceLifecycle",
											"--disable-admission-plugins=PersistentVolumeLabel",
										},
										Env: []corev1.EnvVar{
											{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "?"},
										},
										VolumeMounts: []corev1.VolumeMount{
											{Name: internal.CloudProviderConfigName, MountPath: "?"},
											{Name: v1beta1constants.SecretNameCloudProvider, MountPath: "?"},
										},
									},
								},
								Volumes: []corev1.Volume{
									{Name: internal.CloudProviderConfigName},
									{Name: v1beta1constants.SecretNameCloudProvider},
								},
							},
						},
					},
				}
			)

			client.EXPECT().Get(ctx, secretKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))
			client.EXPECT().Get(ctx, cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, annotations, "1.17.0", false)
		})
	})

	Describe("#EnsureKubeControllerManagerDeployment", func() {
		var (
			client  *mockclient.MockClient
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								v1beta1constants.LabelNetworkPolicyToBlockedCIDRs: v1beta1constants.LabelNetworkPolicyAllowed,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-controller-manager",
								},
							},
						},
					},
				},
			}
			client = mockclient.NewMockClient(ctrl)

			ensurer = NewEnsurer(logger)
			err := ensurer.(inject.Client).InjectClient(client)
			Expect(err).To(Not(HaveOccurred()))
		})

		It("should add missing elements to kube-controller-manager deployment (k8s = 1.17)", func() {
			client.EXPECT().Get(ctx, secretKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))
			client.EXPECT().Get(ctx, cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.17.0", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.18 w/o CSI annotation)", func() {
			client.EXPECT().Get(ctx, secretKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))
			client.EXPECT().Get(ctx, cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s118, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.18.0", false)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.18 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s118WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, nil, nil, "1.18.0", true)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.21 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, nil, nil, "1.21.0", true)
		})

		It("should modify existing elements of kube-controller-manager deployment", func() {
			var (
				dep = &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeControllerManager},
					Spec: appsv1.DeploymentSpec{
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									v1beta1constants.LabelNetworkPolicyToBlockedCIDRs: v1beta1constants.LabelNetworkPolicyAllowed,
								},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "kube-controller-manager",
										Command: []string{
											"--cloud-provider=?",
											"--cloud-config=?",
											"--external-cloud-volume-plugin=?",
										},
										Env: []corev1.EnvVar{
											{Name: "GOOGLE_APPLICATION_CREDENTIALS", Value: "?"},
										},
										VolumeMounts: []corev1.VolumeMount{
											{Name: internal.CloudProviderConfigName, MountPath: "?"},
											{Name: v1beta1constants.SecretNameCloudProvider, MountPath: "?"},
										},
									},
								},
								Volumes: []corev1.Volume{
									{Name: internal.CloudProviderConfigName},
									{Name: v1beta1constants.SecretNameCloudProvider},
								},
							},
						},
					},
				}
			)

			client.EXPECT().Get(ctx, secretKey, &corev1.Secret{}).DoAndReturn(clientGet(secret))
			client.EXPECT().Get(ctx, cmKey, &corev1.ConfigMap{}).DoAndReturn(clientGet(cm))

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, annotations, kubeControllerManagerLabels, "1.17.0", false)
		})
	})

	Describe("#EnsureKubeSchedulerDeployment", func() {
		var (
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameKubeScheduler},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "kube-scheduler",
								},
							},
						},
					},
				},
			}

			ensurer = NewEnsurer(logger)
		})

		It("should not add anything to kube-scheduler deployment (k8s < 1.18)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s117, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.17.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.18 w/o CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s118, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.18.0", false)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.18 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s118WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.18.0", true)
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.21 w/ CSI annotation)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.21.0", true)
		})
	})

	Describe("#EnsureClusterAutoscalerDeployment", func() {
		var (
			dep     *appsv1.Deployment
			ensurer genericmutator.Ensurer
		)

		BeforeEach(func() {
			dep = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: v1beta1constants.DeploymentNameClusterAutoscaler},
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "cluster-autoscaler",
								},
							},
						},
					},
				},
			}

			ensurer = NewEnsurer(logger)
		})

		It("should not add anything to cluster-autoscaler deployment (k8s < 1.20)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s118, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.18.0")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s 1.20)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s120WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.20.0")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.21)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s121WithCSIAnnotation, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.21.0")
		})
	})

	Describe("#EnsureKubeletServiceUnitOptions", func() {
		var (
			ensurer               genericmutator.Ensurer
			oldUnitOptions        []*unit.UnitOption
			hostnamectlUnitOption *unit.UnitOption
		)

		BeforeEach(func() {
			ensurer = NewEnsurer(logger)
			oldUnitOptions = []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet`,
				},
			}
			hostnamectlUnitOption = &unit.UnitOption{
				Section: "Service",
				Name:    "ExecStartPre",
				Value:   `/bin/sh -c 'hostnamectl set-hostname $(wget -q -O- --header "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/hostname | cut -d '.' -f 1)'`,
			}
		})

		DescribeTable("should modify existing elements of kubelet.service unit options",
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, cloudProvider string, withControllerAttachDetachFlag bool) {
				newUnitOptions := []*unit.UnitOption{
					{
						Section: "Service",
						Name:    "ExecStart",
						Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet`,
					},
					hostnamectlUnitOption,
				}

				if cloudProvider != "" {
					newUnitOptions[0].Value += ` \
    --cloud-provider=` + cloudProvider
				}

				if withControllerAttachDetachFlag {
					newUnitOptions[0].Value += ` \
    --enable-controller-attach-detach=true`
				}

				opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, gctx, kubeletVersion, oldUnitOptions, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(opts).To(Equal(newUnitOptions))
			},

			Entry("1.17 <= kubelet version < 1.18", eContextK8s117, semver.MustParse("1.17.0"), "gce", false),
			Entry("1.18 <= kubelet version < 1.23", eContextK8s118, semver.MustParse("1.18.0"), "external", true),
			Entry("kubelet version >= 1.23", eContextK8s118, semver.MustParse("1.23.0"), "external", false),
		)
	})

	Describe("#EnsureKubeletConfiguration", func() {
		var (
			ensurer          genericmutator.Ensurer
			oldKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration
		)

		BeforeEach(func() {
			ensurer = NewEnsurer(logger)
			oldKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo": true,
				},
			}
		})

		DescribeTable("should modify existing elements of kubelet configuration",
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, unregisterFeatureGateName string, enableControllerAttachDetach *bool) {
				newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
					FeatureGates: map[string]bool{
						"Foo": true,
					},
					EnableControllerAttachDetach: enableControllerAttachDetach,
				}

				if unregisterFeatureGateName != "" {
					newKubeletConfig.FeatureGates["CSIMigration"] = true
					newKubeletConfig.FeatureGates["CSIMigrationGCE"] = true
					newKubeletConfig.FeatureGates[unregisterFeatureGateName] = true
				}

				kubeletConfig := *oldKubeletConfig

				err := ensurer.EnsureKubeletConfiguration(ctx, gctx, kubeletVersion, &kubeletConfig, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(&kubeletConfig).To(Equal(newKubeletConfig))
			},

			Entry("control plane, kubelet < 1.18", eContextK8s117, semver.MustParse("1.17.0"), "", nil),
			Entry("1.18 <= control plane, kubelet <= 1.21", eContextK8s118, semver.MustParse("1.18.0"), "CSIMigrationGCEComplete", nil),
			Entry("controlplane >= 1.21, kubelet < 1.21", eContextK8s121, semver.MustParse("1.20.0"), "CSIMigrationGCEComplete", nil),
			Entry("1.23 <= control plane, kubelet < 1.23", eContextK8s121, semver.MustParse("1.21.0"), "InTreePluginGCEUnregister", nil),
			Entry("kubelet >= 1.23", eContextK8s121, semver.MustParse("1.23.0"), "InTreePluginGCEUnregister", pointer.Bool(true)),
		)
	})

	Describe("#EnsureKubernetesGeneralConfiguration", func() {
		var ensurer genericmutator.Ensurer

		BeforeEach(func() {
			ensurer = NewEnsurer(logger)
		})

		It("should modify existing elements of kubernetes general configuration", func() {
			var (
				modifiedData = pointer.StringPtr("# Default Socket Send Buffer\n" +
					"net.core.wmem_max = 16777216\n" +
					"# GCE specific settings\n" +
					"net.ipv4.ip_forward = 5\n" +
					"# For persistent HTTP connections\n" +
					"net.ipv4.tcp_slow_start_after_idle = 0")
				result = "# Default Socket Send Buffer\n" +
					"net.core.wmem_max = 16777216\n" +
					"# GCE specific settings\n" +
					"net.ipv4.ip_forward = 1\n" +
					"# For persistent HTTP connections\n" +
					"net.ipv4.tcp_slow_start_after_idle = 0"
			)

			err := ensurer.EnsureKubernetesGeneralConfiguration(ctx, dummyContext, modifiedData, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*modifiedData).To(Equal(result))
		})
		It("should add needed elements of kubernetes general configuration", func() {
			var (
				data   = pointer.StringPtr("# Default Socket Send Buffer\nnet.core.wmem_max = 16777216")
				result = "# Default Socket Send Buffer\n" +
					"net.core.wmem_max = 16777216\n" +
					"# GCE specific settings\n" +
					"net.ipv4.ip_forward = 1"
			)

			err := ensurer.EnsureKubernetesGeneralConfiguration(ctx, dummyContext, data, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(*data).To(Equal(result))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, annotations map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionAtLeast118, _ := version.CompareVersions(k8sVersion, ">=", "1.18")
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-provider=gce"))
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(Not(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ",")))
		Expect(c.Env).To(ContainElement(credentialsEnvVar))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderSecretVolumeMount))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderSecretVolume))
		if k8sVersionAtLeast118 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true"))
		}
	} else {
		if k8sVersionAtLeast121 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,InTreePluginGCEUnregister=true"))
		} else {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,CSIMigrationGCEComplete=true"))
		}
		Expect(c.Command).NotTo(ContainElement("--cloud-provider=gce"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Env).NotTo(ContainElement(credentialsEnvVar))
		Expect(dep.Spec.Template.Annotations).To(BeNil())
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(dep.Spec.Template.Annotations).To(BeNil())
	}

}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, annotations, labels map[string]string, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	k8sVersionAtLeast118, _ := version.CompareVersions(k8sVersion, ">=", "1.18")
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")

	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--cloud-provider=external"))
		Expect(c.Command).To(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).To(ContainElement("--external-cloud-volume-plugin=gce"))
		Expect(c.Env).To(ContainElement(credentialsEnvVar))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderConfigVolumeMount))
		Expect(c.VolumeMounts).To(ContainElement(cloudProviderSecretVolumeMount))
		Expect(dep.Spec.Template.Annotations).To(Equal(annotations))
		Expect(dep.Spec.Template.Labels).To(Equal(labels))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderConfigVolume))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(cloudProviderSecretVolume))
		Expect(c.VolumeMounts).To(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(etcSSLVolume))
		Expect(c.VolumeMounts).To(ContainElement(usrShareCaCertsVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).To(ContainElement(usrShareCaCertsVolume))
		if k8sVersionAtLeast118 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true"))
		}
	} else {
		if k8sVersionAtLeast121 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,InTreePluginGCEUnregister=true"))
		} else {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,CSIMigrationGCEComplete=true"))
		}
		Expect(c.Command).To(ContainElement("--cloud-provider=external"))
		Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
		Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=gce"))
		Expect(c.Env).NotTo(ContainElement(credentialsEnvVar))
		Expect(dep.Spec.Template.Labels).To(BeEmpty())
		Expect(dep.Spec.Template.Spec.Volumes).To(BeNil())
		Expect(c.VolumeMounts).NotTo(ContainElement(cloudProviderConfigVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(cloudProviderConfigVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
		Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCaCertsVolumeMount))
		Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCaCertsVolume))
	}
}

func checkKubeSchedulerDeployment(dep *appsv1.Deployment, k8sVersion string, needsCSIMigrationCompletedFeatureGates bool) {
	if k8sVersionAtLeast118, _ := version.CompareVersions(k8sVersion, ">=", "1.18"); !k8sVersionAtLeast118 {
		return
	}
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")

	// Check that the kube-scheduler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-scheduler")
	Expect(c).To(Not(BeNil()))

	if !needsCSIMigrationCompletedFeatureGates {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true"))
	} else {
		if k8sVersionAtLeast121 {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,InTreePluginGCEUnregister=true"))
		} else {
			Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,CSIMigrationGCEComplete=true"))
		}
	}
}

func checkClusterAutoscalerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	if k8sVersionAtLeast120, _ := version.CompareVersions(k8sVersion, ">=", "1.20"); !k8sVersionAtLeast120 {
		return
	}
	k8sVersionAtLeast121, _ := version.CompareVersions(k8sVersion, ">=", "1.21")

	// Check that the cluster-autoscaler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "cluster-autoscaler")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast121 {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,InTreePluginGCEUnregister=true"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=CSIMigration=true,CSIMigrationGCE=true,CSIMigrationGCEComplete=true"))
	}
}

func clientGet(result runtime.Object) interface{} {
	return func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
		switch obj.(type) {
		case *corev1.Secret:
			*obj.(*corev1.Secret) = *result.(*corev1.Secret)
		case *corev1.ConfigMap:
			*obj.(*corev1.ConfigMap) = *result.(*corev1.ConfigMap)
		}
		return nil
	}
}
