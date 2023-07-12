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

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-gcp/charts"
	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Machines", func() {
	var (
		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		chartApplier *mockkubernetes.MockChartApplier
		statusWriter *mockclient.MockStatusWriter
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("workerDelegate", func() {
		workerDelegate, _ := NewWorkerDelegate(nil, nil, nil, nil, "", nil, nil)

		Describe("#MachineClassKind", func() {
			It("should return the correct kind of the machine class", func() {
				Expect(workerDelegate.MachineClassKind()).To(Equal("MachineClass"))
			})
		})

		Describe("#MachineClass", func() {
			It("should return the correct type for the machine class", func() {
				Expect(workerDelegate.MachineClass()).To(Equal(&machinev1alpha1.MachineClass{}))
			})
		})

		Describe("#MachineClassList", func() {
			It("should return the correct type for the machine class list", func() {
				Expect(workerDelegate.MachineClassList()).To(Equal(&machinev1alpha1.MachineClassList{}))
			})
		})

		Describe("#GenerateMachineDeployments, #DeployMachineClasses", func() {
			var (
				name             string
				namespace        string
				cloudProfileName string

				region string

				machineImageName    string
				machineImageVersion string
				machineImage        string

				serviceAccountEmail string
				machineType         string
				userData            = []byte("some-user-data")
				subnetName          string

				volumeType string
				volumeSize int

				localVolumeType      string
				localVolumeInterface string

				namePool1           string
				minPool1            int32
				maxPool1            int32
				maxSurgePool1       intstr.IntOrString
				maxUnavailablePool1 intstr.IntOrString

				namePool2           string
				minPool2            int32
				maxPool2            int32
				maxSurgePool2       intstr.IntOrString
				maxUnavailablePool2 intstr.IntOrString

				zone1 string
				zone2 string

				archAMD string
				archARM string

				labels map[string]string

				nodeCapacity         corev1.ResourceList
				nodeTemplateZone1    machinev1alpha1.NodeTemplate
				nodeTemplateZone2    machinev1alpha1.NodeTemplate
				machineConfiguration *machinev1alpha1.MachineConfiguration

				workerPoolHash1 string
				workerPoolHash2 string

				shootVersionMajorMinor string
				shootVersion           string
				scheme                 *runtime.Scheme
				decoder                runtime.Decoder
				clusterWithoutImages   *extensionscontroller.Cluster
				cluster                *extensionscontroller.Cluster
				w                      *extensionsv1alpha1.Worker
			)

			BeforeEach(func() {
				name = "my-shoot"
				namespace = "shoot--foobar--gcp"
				cloudProfileName = "gcp"

				region = "eu-west-1"

				machineImageName = "my-os"
				machineImageVersion = "123"
				machineImage = "path/to/project/machine/image"

				serviceAccountEmail = "service@account.com"
				machineType = "large"
				userData = []byte("some-user-data")
				subnetName = "subnet-nodes"

				volumeType = "normal"
				volumeSize = 20

				localVolumeType = "SCRATCH"
				localVolumeInterface = "SCSI"

				namePool1 = "pool-1"
				minPool1 = 5
				maxPool1 = 10
				maxSurgePool1 = intstr.FromInt(3)
				maxUnavailablePool1 = intstr.FromInt(2)

				namePool2 = "pool-2"
				minPool2 = 30
				maxPool2 = 45
				maxSurgePool2 = intstr.FromInt(10)
				maxUnavailablePool2 = intstr.FromInt(15)

				zone1 = region + "a"
				zone2 = region + "b"

				archAMD = "amd64"
				archARM = "arm64"

				labels = map[string]string{"component": "TiDB"}

				nodeCapacity = corev1.ResourceList{
					"cpu":    resource.MustParse("8"),
					"gpu":    resource.MustParse("0"),
					"memory": resource.MustParse("128Gi"),
				}
				nodeTemplateZone1 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         zone1,
				}

				nodeTemplateZone2 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         zone2,
				}

				machineConfiguration = &machinev1alpha1.MachineConfiguration{}

				shootVersionMajorMinor = "1.24"
				shootVersion = shootVersionMajorMinor + ".3"

				clusterWithoutImages = &extensionscontroller.Cluster{
					Shoot: &gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Kubernetes: gardencorev1beta1.Kubernetes{
								Version: shootVersion,
							},
						},
					},
				}

				cloudProfileConfig := &apiv1alpha1.CloudProfileConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
						Kind:       "CloudProfileConfig",
					},
					MachineImages: []apiv1alpha1.MachineImages{
						{
							Name: machineImageName,
							Versions: []apiv1alpha1.MachineImageVersion{
								{
									Version:      machineImageVersion,
									Image:        machineImage,
									Architecture: pointer.String(archAMD),
								},
							},
						},
					},
				}
				cloudProfileConfigJSON, _ := json.Marshal(cloudProfileConfig)
				cluster = &extensionscontroller.Cluster{
					CloudProfile: &gardencorev1beta1.CloudProfile{
						ObjectMeta: metav1.ObjectMeta{
							Name: cloudProfileName,
						},
						Spec: gardencorev1beta1.CloudProfileSpec{
							ProviderConfig: &runtime.RawExtension{
								Raw: cloudProfileConfigJSON,
							},
						},
					},
					Shoot: clusterWithoutImages.Shoot,
				}

				w = &extensionsv1alpha1.Worker{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
					},
					Spec: extensionsv1alpha1.WorkerSpec{
						SecretRef: corev1.SecretReference{
							Name:      "secret",
							Namespace: namespace,
						},
						Region: region,
						InfrastructureProviderStatus: &runtime.RawExtension{
							Raw: encode(&api.InfrastructureStatus{
								ServiceAccountEmail: serviceAccountEmail,
								Networks: api.NetworkStatus{
									Subnets: []api.Subnet{
										{
											Name:    subnetName,
											Purpose: api.PurposeNodes,
										},
									},
								},
							}),
						},
						Pools: []extensionsv1alpha1.WorkerPool{
							{
								Name:           namePool1,
								Minimum:        minPool1,
								Maximum:        maxPool1,
								MaxSurge:       maxSurgePool1,
								MaxUnavailable: maxUnavailablePool1,
								MachineType:    machineType,
								Architecture:   pointer.String(archAMD),
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserData: userData,
								Volume: &extensionsv1alpha1.Volume{
									Type: &volumeType,
									Size: fmt.Sprintf("%dGi", volumeSize),
								},
								DataVolumes: []extensionsv1alpha1.DataVolume{
									{
										Type: &localVolumeType,
										Size: fmt.Sprintf("%dGi", volumeSize),
									},
								},
								ProviderConfig: &runtime.RawExtension{
									Raw: encode(&api.WorkerConfig{
										Volume: &api.Volume{
											LocalSSDInterface: &localVolumeInterface,
										},
									}),
								},
								Zones: []string{
									zone1,
									zone2,
								},
								Labels: labels,
							},
							{
								Name:           namePool2,
								Minimum:        minPool2,
								Maximum:        maxPool2,
								MaxSurge:       maxSurgePool2,
								MaxUnavailable: maxUnavailablePool2,
								MachineType:    machineType,
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								ProviderConfig: &runtime.RawExtension{
									Raw: encode(&api.WorkerConfig{
										Volume: &api.Volume{
											LocalSSDInterface: &localVolumeInterface,
										},
										ServiceAccount: &api.ServiceAccount{
											Email:  "foo",
											Scopes: []string{"bar"},
										},
									}),
								},
								UserData: userData,
								Volume: &extensionsv1alpha1.Volume{
									Type: &volumeType,
									Size: fmt.Sprintf("%dGi", volumeSize),
								},
								DataVolumes: []extensionsv1alpha1.DataVolume{
									{
										Type: &localVolumeType,
										Size: fmt.Sprintf("%dGi", volumeSize),
									},
								},
								Zones: []string{
									zone1,
									zone2,
								},
								Labels: labels,
							},
						},
					},
				}

				scheme = runtime.NewScheme()
				_ = api.AddToScheme(scheme)
				_ = apiv1alpha1.AddToScheme(scheme)
				decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()

				workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster)
				workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster)

				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, clusterWithoutImages)
			})

			Describe("machine images", func() {
				var (
					defaultMachineClass map[string]interface{}
					machineDeployments  worker.MachineDeployments
					machineClasses      map[string]interface{}
				)

				setup := func(disableExternalIP bool) {
					gceInstanceLabels := map[string]interface{}{
						"name": name,
					}
					for k, v := range labels {
						gceInstanceLabels[SanitizeGcpLabel(k)] = SanitizeGcpLabelValue(v)
					}
					defaultMachineClass = map[string]interface{}{
						"region":             region,
						"canIpForward":       true,
						"deletionProtection": false,
						"description":        fmt.Sprintf("Machine of Shoot %s created by machine-controller-manager.", name),
						"disks": []map[string]interface{}{
							{
								"autoDelete": true,
								"boot":       true,
								"sizeGb":     volumeSize,
								"type":       volumeType,
								"image":      machineImage,
								"labels": map[string]interface{}{
									"name": name,
								},
							},
							{
								"autoDelete": true,
								"boot":       false,
								"sizeGb":     volumeSize,
								"type":       localVolumeType,
								"interface":  localVolumeInterface,
								"labels": map[string]interface{}{
									"name": name,
								},
							},
						},
						"labels": gceInstanceLabels,
						"metadata": []map[string]string{
							{
								"key":   "block-project-ssh-keys",
								"value": "TRUE",
							},
						},
						"machineType": machineType,
						"networkInterfaces": []map[string]interface{}{
							{
								"subnetwork":        subnetName,
								"disableExternalIP": disableExternalIP,
							},
						},
						"scheduling": map[string]interface{}{
							"automaticRestart":  true,
							"onHostMaintenance": "MIGRATE",
							"preemptible":       false,
						},
						"secret": map[string]interface{}{
							"cloudConfig": string(userData),
						},
						"serviceAccounts": []map[string]interface{}{
							{
								"email": serviceAccountEmail,
								"scopes": []string{
									"https://www.googleapis.com/auth/compute",
								},
							},
						},
						"tags": []string{
							namespace,
							fmt.Sprintf("kubernetes-io-cluster-%s", namespace),
							"kubernetes-io-role-node",
						},
					}

					var (
						machineClassPool2 = useDefaultMachineClass(
							defaultMachineClass,
							"serviceAccounts",
							[]map[string]interface{}{{"email": "foo", "scopes": []string{"bar"}}},
						)

						machineClassPool1Zone1 = useDefaultMachineClass(defaultMachineClass, "zone", zone1)
						machineClassPool1Zone2 = useDefaultMachineClass(defaultMachineClass, "zone", zone2)
						machineClassPool2Zone1 = useDefaultMachineClass(machineClassPool2, "zone", zone1)
						machineClassPool2Zone2 = useDefaultMachineClass(machineClassPool2, "zone", zone2)

						machineClassNamePool1Zone1 = fmt.Sprintf("%s-%s-z1", namespace, namePool1)
						machineClassNamePool1Zone2 = fmt.Sprintf("%s-%s-z2", namespace, namePool1)
						machineClassNamePool2Zone1 = fmt.Sprintf("%s-%s-z1", namespace, namePool2)
						machineClassNamePool2Zone2 = fmt.Sprintf("%s-%s-z2", namespace, namePool2)

						machineClassWithHashPool1Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone1, workerPoolHash1)
						machineClassWithHashPool1Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone2, workerPoolHash1)
						machineClassWithHashPool2Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone1, workerPoolHash2)
						machineClassWithHashPool2Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone2, workerPoolHash2)
					)

					addNameAndSecretToMachineClass(machineClassPool1Zone1, machineClassWithHashPool1Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool1Zone2, machineClassWithHashPool1Zone2, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone1, machineClassWithHashPool2Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone2, machineClassWithHashPool2Zone2, w.Spec.SecretRef)

					addNodeTemplateToMachineClass(machineClassPool1Zone1, nodeTemplateZone1)
					addNodeTemplateToMachineClass(machineClassPool1Zone2, nodeTemplateZone2)
					addNodeTemplateToMachineClass(machineClassPool2Zone1, nodeTemplateZone1)
					addNodeTemplateToMachineClass(machineClassPool2Zone2, nodeTemplateZone2)

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1Zone1,
						machineClassPool1Zone2,
						machineClassPool2Zone1,
						machineClassPool2Zone2,
					}}

					labelsZone1 := utils.MergeStringMaps(labels, map[string]string{gcp.CSIDiskDriverTopologyKey: zone1})
					labelsZone2 := utils.MergeStringMaps(labels, map[string]string{gcp.CSIDiskDriverTopologyKey: zone2})
					machineDeployments = worker.MachineDeployments{
						{
							Name:                 machineClassNamePool1Zone1,
							ClassName:            machineClassWithHashPool1Zone1,
							SecretName:           machineClassWithHashPool1Zone1,
							Minimum:              worker.DistributeOverZones(0, minPool1, 2),
							Maximum:              worker.DistributeOverZones(0, maxPool1, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(0, maxSurgePool1, 2, maxPool1),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(0, maxUnavailablePool1, 2, minPool1),
							Labels:               labelsZone1,
							MachineConfiguration: machineConfiguration,
						},
						{
							Name:                 machineClassNamePool1Zone2,
							ClassName:            machineClassWithHashPool1Zone2,
							SecretName:           machineClassWithHashPool1Zone2,
							Minimum:              worker.DistributeOverZones(1, minPool1, 2),
							Maximum:              worker.DistributeOverZones(1, maxPool1, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(1, maxSurgePool1, 2, maxPool1),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(1, maxUnavailablePool1, 2, minPool1),
							Labels:               labelsZone2,
							MachineConfiguration: machineConfiguration,
						},
						{
							Name:                 machineClassNamePool2Zone1,
							ClassName:            machineClassWithHashPool2Zone1,
							SecretName:           machineClassWithHashPool2Zone1,
							Minimum:              worker.DistributeOverZones(0, minPool2, 2),
							Maximum:              worker.DistributeOverZones(0, maxPool2, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(0, maxSurgePool2, 2, maxPool2),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(0, maxUnavailablePool2, 2, minPool2),
							Labels:               labelsZone1,
							MachineConfiguration: machineConfiguration,
						},
						{
							Name:                 machineClassNamePool2Zone2,
							ClassName:            machineClassWithHashPool2Zone2,
							SecretName:           machineClassWithHashPool2Zone2,
							Minimum:              worker.DistributeOverZones(1, minPool2, 2),
							Maximum:              worker.DistributeOverZones(1, maxPool2, 2),
							MaxSurge:             worker.DistributePositiveIntOrPercent(1, maxSurgePool2, 2, maxPool2),
							MaxUnavailable:       worker.DistributePositiveIntOrPercent(1, maxUnavailablePool2, 2, minPool2),
							Labels:               labelsZone2,
							MachineConfiguration: machineConfiguration,
						},
					}
				}
				It("should return the expected machine deployments when disableExternal IP is true with profile image types", func() {
					setup(true)
					workerCloudRouter := w
					workerCloudRouter.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
						Raw: encode(&api.InfrastructureStatus{
							ServiceAccountEmail: serviceAccountEmail,
							Networks: api.NetworkStatus{
								VPC: api.VPC{
									CloudRouter: &api.CloudRouter{
										Name: "my-cloudrouter",
									},
								},
								Subnets: []api.Subnet{
									{
										Name:    subnetName,
										Purpose: api.PurposeNodes,
									},
								},
							},
						}),
					}

					workerDelegateCloudRouter, _ := NewWorkerDelegate(c, decoder, scheme, chartApplier, "", workerCloudRouter, cluster)
					// Test workerDelegate.DeployMachineClasses()
					chartApplier.EXPECT().ApplyFromEmbeddedFS(
						context.TODO(),
						charts.InternalChart,
						filepath.Join("internal", "machineclass"),
						namespace,
						"machineclass",
						kubernetes.Values(machineClasses),
					)

					err := workerDelegateCloudRouter.DeployMachineClasses(context.TODO())
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.UpdateMachineDeployments()
					expectedImages := &apiv1alpha1.WorkerStatus{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
							Kind:       "WorkerStatus",
						},
						MachineImages: []apiv1alpha1.MachineImage{
							{
								Name:         machineImageName,
								Version:      machineImageVersion,
								Image:        machineImage,
								Architecture: pointer.String(archAMD),
							},
						},
					}

					workerWithExpectedImages := w.DeepCopy()
					workerWithExpectedImages.Status.ProviderStatus = &runtime.RawExtension{
						Object: expectedImages,
					}
					ctx := context.TODO()
					c.EXPECT().Status().Return(statusWriter)
					statusWriter.EXPECT().Patch(ctx, workerWithExpectedImages, gomock.Any()).Return(nil)

					err = workerDelegateCloudRouter.UpdateMachineImagesStatus(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test workerDelegate.GenerateMachineDeployments()
					result, err := workerDelegateCloudRouter.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments))
				})
			})

			It("should fail because the version is invalid", func() {
				clusterWithoutImages.Shoot.Spec.Kubernetes.Version = "invalid"
				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the infrastructure status cannot be decoded", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{}

				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the nodes subnet cannot be found", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&api.InfrastructureStatus{}),
				}

				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image for given architecture cannot be found", func() {
				w.Spec.Pools[0].Architecture = pointer.String(archARM)

				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image cannot be found", func() {
				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, clusterWithoutImages)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the volume size cannot be decoded", func() {
				w.Spec.Pools[0].Volume.Size = "not-decodeable"

				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should set expected machineControllerManager settings on machine deployment", func() {
				testDrainTimeout := metav1.Duration{Duration: 10 * time.Minute}
				testHealthTimeout := metav1.Duration{Duration: 20 * time.Minute}
				testCreationTimeout := metav1.Duration{Duration: 30 * time.Minute}
				testMaxEvictRetries := int32(30)
				testNodeConditions := []string{"ReadonlyFilesystem", "KernelDeadlock", "DiskPressure"}
				w.Spec.Pools[0].MachineControllerManagerSettings = &gardencorev1beta1.MachineControllerManagerSettings{
					MachineDrainTimeout:    &testDrainTimeout,
					MachineCreationTimeout: &testCreationTimeout,
					MachineHealthTimeout:   &testHealthTimeout,
					MaxEvictRetries:        &testMaxEvictRetries,
					NodeConditions:         testNodeConditions,
				}

				workerDelegate, _ = NewWorkerDelegate(c, decoder, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(context.TODO())
				resultSettings := result[0].MachineConfiguration
				resultNodeConditions := strings.Join(testNodeConditions, ",")

				Expect(err).NotTo(HaveOccurred())
				Expect(resultSettings.MachineDrainTimeout).To(Equal(&testDrainTimeout))
				Expect(resultSettings.MachineCreationTimeout).To(Equal(&testCreationTimeout))
				Expect(resultSettings.MachineHealthTimeout).To(Equal(&testHealthTimeout))
				Expect(resultSettings.MaxEvictRetries).To(Equal(&testMaxEvictRetries))
				Expect(resultSettings.NodeConditions).To(Equal(&resultNodeConditions))
			})
		})
	})

	Describe("sanitize gcp label/value ", func() {
		It("gcp label must start with lowercase character", func() {
			Expect(SanitizeGcpLabel("////Abcd-efg")).To(Equal("abcd-efg"))
			Expect(SanitizeGcpLabel("1Abcd-efg")).To(Equal("abcd-efg"))
		})
		It("gcp label value can  start with '-' ", func() {
			Expect(SanitizeGcpLabelValue("////Abcd-efg")).To(Equal("____abcd-efg"))
			Expect(SanitizeGcpLabelValue("1Abcd-efg")).To(Equal("1abcd-efg"))
		})
		It("label can be at most 63 characters long", func() {
			Expect(SanitizeGcpLabel("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz0123456789abcd")).To(Equal("abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz0123456789a"))
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func useDefaultMachineClass(def map[string]interface{}, key string, value interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(def)+1)

	for k, v := range def {
		out[k] = v
	}

	out[key] = value
	return out
}

func addNodeTemplateToMachineClass(class map[string]interface{}, nodeTemplate machinev1alpha1.NodeTemplate) {
	class["nodeTemplate"] = nodeTemplate
}

func addNameAndSecretToMachineClass(class map[string]interface{}, name string, credentialsSecretRef corev1.SecretReference) {
	class["name"] = name
	class["resourceLabels"] = map[string]string{
		v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
	}
	class["credentialsSecretRef"] = map[string]interface{}{
		"name":      credentialsSecretRef.Name,
		"namespace": credentialsSecretRef.Namespace,
	}
}
