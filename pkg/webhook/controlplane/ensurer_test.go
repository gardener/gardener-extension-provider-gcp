// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/coreos/go-systemd/v22/unit"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/test"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	testutils "github.com/gardener/gardener/pkg/utils/test"
	"github.com/gardener/gardener/pkg/utils/version"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const namespace = "test"

var serviceRange = []string{"10.0.0.0/16", "2001:0db8::/32"}

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		ctrl       *gomock.Controller
		ctx        = context.TODO()
		fakeClient client.Client

		dummyContext = gcontext.NewGardenContext(nil, nil)

		eContextK8s130 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.30.1",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						Networking: &gardencorev1beta1.NetworkingStatus{
							Services: serviceRange,
						},
					},
				},
			},
		)
		eContextK8s131 = gcontext.NewInternalGardenContext(
			&extensionscontroller.Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							Version: "1.31.1",
						},
					},
					Status: gardencorev1beta1.ShootStatus{
						Networking: &gardencorev1beta1.NetworkingStatus{
							Services: serviceRange,
						},
					},
				},
			},
		)
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		fakeClient = fakeclient.NewClientBuilder().Build()
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureKubeAPIServerDeployment", func() {
		var (
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

			ensurer = NewEnsurer(fakeClient, logger)
		})

		It("should add missing elements to kube-apiserver deployment (k8s < 1.31)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to kube-apiserver deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeAPIServerDeployment(dep, "1.31.1")
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
									},
								},
							},
						},
					},
				}
			)

			err := ensurer.EnsureKubeAPIServerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeAPIServerDeployment(dep, "1.30.1")
		})
	})

	Describe("#EnsureKubeControllerManagerDeployment", func() {
		var (
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
								v1beta1constants.LabelNetworkPolicyToBlockedCIDRs:    v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
								v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
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

			ensurer = NewEnsurer(fakeClient, logger)
		})

		It("should add missing elements to kube-controller-manager deployment (k8s < 1.31)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to kube-controller-manager deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeControllerManagerDeployment(dep, "1.31.1")
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
										VolumeMounts: []corev1.VolumeMount{
											{Name: usrShareCaCerts, MountPath: "?"},
											{Name: etcSSLName, MountPath: "?"},
										},
									},
								},
								Volumes: []corev1.Volume{
									{Name: usrShareCaCerts},
									{Name: etcSSLName},
								},
							},
						},
					},
				}
			)

			err := ensurer.EnsureKubeControllerManagerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))
			checkKubeControllerManagerDeployment(dep, "1.30.1")
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

			ensurer = NewEnsurer(fakeClient, logger)
		})

		It("should add missing elements to kube-scheduler deployment (k8s < 1.31)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to kube-scheduler deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureKubeSchedulerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkKubeSchedulerDeployment(dep, "1.31.1")
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

			ensurer = NewEnsurer(fakeClient, logger)
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s < 1.31)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s130, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.30.1")
		})

		It("should add missing elements to cluster-autoscaler deployment (k8s >= 1.31)", func() {
			err := ensurer.EnsureClusterAutoscalerDeployment(ctx, eContextK8s131, dep, nil)
			Expect(err).To(Not(HaveOccurred()))

			checkClusterAutoscalerDeployment(dep, "1.31.1")
		})
	})

	Describe("#EnsureKubeletServiceUnitOptions", func() {
		var (
			ensurer               genericmutator.Ensurer
			oldUnitOptions        []*unit.UnitOption
			hostnamectlUnitOption *unit.UnitOption
		)

		BeforeEach(func() {
			ensurer = NewEnsurer(fakeClient, logger)
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

		It("should modify existing elements of kubelet.service unit options", func() {
			newUnitOptions := []*unit.UnitOption{
				{
					Section: "Service",
					Name:    "ExecStart",
					Value: `/opt/bin/hyperkube kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=external`,
				},
				hostnamectlUnitOption,
			}

			opts, err := ensurer.EnsureKubeletServiceUnitOptions(ctx, nil, nil, oldUnitOptions, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(opts).To(Equal(newUnitOptions))
		})
	})

	Describe("#EnsureKubeletConfiguration", func() {
		var (
			ensurer          genericmutator.Ensurer
			oldKubeletConfig *kubeletconfigv1beta1.KubeletConfiguration
		)

		BeforeEach(func() {
			ensurer = NewEnsurer(fakeClient, logger)
			oldKubeletConfig = &kubeletconfigv1beta1.KubeletConfiguration{
				FeatureGates: map[string]bool{
					"Foo": true,
				},
			}
		})

		DescribeTable("should modify existing elements of kubelet configuration",
			func(gctx gcontext.GardenContext, kubeletVersion *semver.Version, expectedFeatureGates map[string]bool) {
				newKubeletConfig := &kubeletconfigv1beta1.KubeletConfiguration{
					FeatureGates: map[string]bool{
						"Foo": true,
					},
					EnableControllerAttachDetach: ptr.To(true),
				}

				for featureGate, value := range expectedFeatureGates {
					newKubeletConfig.FeatureGates[featureGate] = value
				}

				kubeletConfig := *oldKubeletConfig

				err := ensurer.EnsureKubeletConfiguration(ctx, gctx, kubeletVersion, &kubeletConfig, nil)
				Expect(err).To(Not(HaveOccurred()))
				Expect(&kubeletConfig).To(Equal(newKubeletConfig))
			},
			Entry("kubelet < 1.31", eContextK8s130, semver.MustParse("1.30.1"), map[string]bool{"InTreePluginGCEUnregister": true}),
			Entry("kubelet >= 1.31", eContextK8s131, semver.MustParse("1.31.1"), map[string]bool{}),
		)
	})

	Describe("#EnsureKubernetesGeneralConfiguration", func() {
		var ensurer genericmutator.Ensurer

		BeforeEach(func() {
			ensurer = NewEnsurer(fakeClient, logger)
		})

		It("should modify existing elements of kubernetes general configuration", func() {
			var (
				modifiedData = ptr.To("# Default Socket Send Buffer\n" +
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
				data   = ptr.To("# Default Socket Send Buffer\nnet.core.wmem_max = 16777216")
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

	Describe("#EnsureMachineControllerManagerDeployment", func() {
		var (
			deployment *appsv1.Deployment
			shoot      *gardencorev1beta1.Shoot
			ensurer    genericmutator.Ensurer
			secret     *corev1.Secret
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: "foo"}}
			ensurer = NewEnsurer(fakeClient, logger)
			DeferCleanup(testutils.WithVar(&ImageVector, imagevectorutils.ImageVector{{
				Name:       "machine-controller-manager-provider-gcp",
				Repository: ptr.To("foo"),
				Tag:        ptr.To("bar"),
			}}))
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cloudprovider",
					Namespace: "foo",
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Workers: []gardencorev1beta1.Worker{},
					},
				},
			}
		})

		It("should inject the sidecar container", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers).To(BeEmpty())
			Expect(ensurer.EnsureMachineControllerManagerDeployment(context.TODO(), eContextK8s130, deployment, nil)).To(Succeed())
			expectedContainer := machinecontrollermanager.ProviderSidecarContainer(shoot, deployment.Namespace, "provider-gcp", "foo:bar")
			Expect(deployment.Spec.Template.Spec.Containers).To(ConsistOf(expectedContainer))
		})

		It("should inject the sidecar container with workload identity mount", func() {
			secret.Labels = map[string]string{
				"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
				"workloadidentity.security.gardener.cloud/provider": "gcp",
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(deployment.Spec.Template.Spec.Containers).To(BeEmpty())
			Expect(ensurer.EnsureMachineControllerManagerDeployment(context.TODO(), eContextK8s130, deployment, nil)).To(Succeed())
			expectedContainer := machinecontrollermanager.ProviderSidecarContainer(shoot, deployment.Namespace, "provider-gcp", "foo:bar")
			expectedContainer.VolumeMounts = append(expectedContainer.VolumeMounts, corev1.VolumeMount{
				Name:      "workload-identity",
				MountPath: "/var/run/secrets/gardener.cloud/workload-identity",
			})
			Expect(deployment.Spec.Template.Spec.Containers).To(ConsistOf(expectedContainer))
			Expect(deployment.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
				Name: "workload-identity",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: v1beta1constants.SecretNameCloudProvider,
									},
									Items: []corev1.KeyToPath{
										{
											Key:  "token",
											Path: "token",
										},
									},
								},
							},
						},
					},
				},
			}))
		})
	})

	Describe("#EnsureMachineControllerManagerVPA", func() {
		var (
			ensurer genericmutator.Ensurer
			vpa     *vpaautoscalingv1.VerticalPodAutoscaler
		)

		BeforeEach(func() {
			ensurer = NewEnsurer(fakeClient, logger)
			vpa = &vpaautoscalingv1.VerticalPodAutoscaler{}
		})

		It("should inject the sidecar container policy", func() {
			Expect(vpa.Spec.ResourcePolicy).To(BeNil())
			Expect(ensurer.EnsureMachineControllerManagerVPA(context.TODO(), nil, vpa, nil)).To(Succeed())

			ccv := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
			Expect(vpa.Spec.ResourcePolicy.ContainerPolicies).To(ConsistOf(vpaautoscalingv1.ContainerResourcePolicy{
				ContainerName:    "machine-controller-manager-provider-gcp",
				ControlledValues: &ccv,
			}))
		})
	})

	Describe("#EnsureAdditionalUnits", func() {
		var ensurer genericmutator.Ensurer

		BeforeEach(func() {
			ensurer = NewEnsurer(fakeClient, logger)
		})

		It("should add google-guest-agent.service unit", func() {
			var units []extensionsv1alpha1.Unit

			err := ensurer.EnsureAdditionalUnits(ctx, nil, &units, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(units).To(HaveLen(1))

			unit := units[0]
			Expect(unit.Name).To(Equal("google-guest-agent.service"))
			Expect(unit.Enable).To(Equal(ptr.To(true)))
			Expect(unit.Command).To(Equal(ptr.To(extensionsv1alpha1.CommandStart)))
			Expect(unit.Content).ToNot(BeNil())
			Expect(*unit.Content).To(ContainSubstring("Description=Google Compute Engine Guest Agent"))
			Expect(*unit.Content).To(ContainSubstring("ExecStart=/usr/bin/google_guest_agent"))
		})
	})

	Describe("#EnsureAdditionalFiles", func() {
		var ensurer genericmutator.Ensurer

		BeforeEach(func() {
			ensurer = NewEnsurer(fakeClient, logger)
		})

		It("should add instance_configs.cfg file", func() {
			var files []extensionsv1alpha1.File

			err := ensurer.EnsureAdditionalFiles(ctx, nil, &files, nil)
			Expect(err).To(Not(HaveOccurred()))
			Expect(files).To(HaveLen(1))

			file := files[0]
			Expect(file.Path).To(Equal("/etc/default/instance_configs.cfg"))
			Expect(file.Permissions).ToNot(BeNil())
			Expect(*file.Permissions).To(Equal(uint32(0o644)))
			Expect(file.Content.Inline).ToNot(BeNil())
			Expect(file.Content.Inline.Data).To(ContainSubstring("[IpForwarding]"))
			Expect(file.Content.Inline.Data).To(ContainSubstring("ip_aliases = false"))
		})
	})
})

func checkKubeAPIServerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the kube-apiserver container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-apiserver")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast131 {
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginGCEUnregister=true"))
	}

	Expect(c.Command).NotTo(ContainElement("--cloud-provider=gce"))
	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	if !k8sVersionAtLeast131 {
		Expect(c.Command).NotTo(test.ContainElementWithPrefixContaining("--enable-admission-plugins=", "PersistentVolumeLabel", ","))
		Expect(c.Command).To(test.ContainElementWithPrefixContaining("--disable-admission-plugins=", "PersistentVolumeLabel", ","))
	}
}

func checkKubeControllerManagerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the kube-controller-manager container still exists and contains all needed command line args,
	// env vars, and volume mounts
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-controller-manager")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast131 {
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
		Expect(c.Command).To(ContainElement("--allocate-node-cidrs=false"))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginGCEUnregister=true"))
		Expect(c.Command).To(ContainElement("--allocate-node-cidrs=true"))
	}

	Expect(c.Command).To(ContainElement("--cloud-provider=external"))
	Expect(c.Command).NotTo(ContainElement("--cloud-config=/etc/kubernetes/cloudprovider/cloudprovider.conf"))
	Expect(c.Command).NotTo(ContainElement("--external-cloud-volume-plugin=gce"))
	Expect(c.VolumeMounts).NotTo(ContainElement(etcSSLVolumeMount))
	Expect(c.VolumeMounts).NotTo(ContainElement(usrShareCaCertsVolumeMount))
	Expect(dep.Spec.Template.Labels).To(BeEmpty())
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(etcSSLVolume))
	Expect(dep.Spec.Template.Spec.Volumes).NotTo(ContainElement(usrShareCaCertsVolume))
	Expect(dep.Spec.Template.Spec.Volumes).To(BeEmpty())
}

func checkKubeSchedulerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the kube-scheduler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "kube-scheduler")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast131 {
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginGCEUnregister=true"))
	}
}

func checkClusterAutoscalerDeployment(dep *appsv1.Deployment, k8sVersion string) {
	k8sVersionAtLeast131, _ := version.CompareVersions(k8sVersion, ">=", "1.31")

	// Check that the cluster-autoscaler container still exists and contains all needed command line args.
	c := extensionswebhook.ContainerWithName(dep.Spec.Template.Spec.Containers, "cluster-autoscaler")
	Expect(c).To(Not(BeNil()))

	if k8sVersionAtLeast131 {
		Expect(c.Command).NotTo(ContainElement(HavePrefix("--feature-gates")))
	} else {
		Expect(c.Command).To(ContainElement("--feature-gates=InTreePluginGCEUnregister=true"))
	}
}
