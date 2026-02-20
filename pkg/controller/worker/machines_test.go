// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
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
	"github.com/gardener/gardener/pkg/utils"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/charts"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Machines", func() {
	var (
		ctx = context.Background()

		ctrl         *gomock.Controller
		c            *mockclient.MockClient
		chartApplier *mockkubernetes.MockChartApplier
		statusWriter *mockclient.MockStatusWriter

		workerDelegate genericworkeractuator.WorkerDelegate
		scheme         *runtime.Scheme
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		c = mockclient.NewMockClient(ctrl)
		chartApplier = mockkubernetes.NewMockChartApplier(ctrl)
		statusWriter = mockclient.NewMockStatusWriter(ctrl)

		scheme = runtime.NewScheme()
		_ = apisgcp.AddToScheme(scheme)
		_ = apiv1alpha1.AddToScheme(scheme)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Context("WorkerDelegate", func() {
		BeforeEach(func() {
			workerDelegate, _ = NewWorkerDelegate(nil, scheme, nil, "", nil, nil)
		})

		Describe("#GenerateMachineDeployments, #DeployMachineClasses", func() {
			var (
				name             string
				namespace        string
				technicalID      string
				cloudProfileName string

				region string

				machineImageName    string
				machineImageVersion string
				machineImage        string

				serviceAccountEmail   string
				machineType           string
				userData              []byte
				userDataSecretName    string
				userDataSecretDataKey string
				subnetName            string

				volumeType string
				volumeSize int

				minCpuPlatform string

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
				priorityPool2       int32
				maxSurgePool2       intstr.IntOrString
				maxUnavailablePool2 intstr.IntOrString

				namePool3           string
				minPool3            int32
				maxPool3            int32
				maxSurgePool3       intstr.IntOrString
				maxUnavailablePool3 intstr.IntOrString

				zone1 string
				zone2 string

				archAMD  string
				archARM  string
				archFAKE string

				acceleratorTypeName string
				acceleratorCount    int32

				poolLabels map[string]string

				nodeCapacity           corev1.ResourceList
				nodeTemplatePool1Zone1 machinev1alpha1.NodeTemplate
				nodeTemplatePool2Zone1 machinev1alpha1.NodeTemplate
				nodeTemplatePool1Zone2 machinev1alpha1.NodeTemplate
				nodeTemplatePool2Zone2 machinev1alpha1.NodeTemplate
				nodeTemplatePool3Zone1 machinev1alpha1.NodeTemplate
				nodeTemplatePool3Zone2 machinev1alpha1.NodeTemplate
				machineConfiguration   *machinev1alpha1.MachineConfiguration

				workerPoolHash1 string
				workerPoolHash2 string
				workerPoolHash3 string

				shootVersionMajorMinor string
				shootVersion           string
				clusterWithoutImages   *extensionscontroller.Cluster
				cluster                *extensionscontroller.Cluster
				w                      *extensionsv1alpha1.Worker
			)

			BeforeEach(func() {
				name = "my-shoot"
				namespace = "control-plane-namespace"
				technicalID = "shoot--foobar--gcp"
				cloudProfileName = "gcp"

				region = "eu-west-1"

				machineImageName = "my-os"
				machineImageVersion = "123.4.5-foo+bar123"
				machineImage = "path/to/project/machine/image"

				serviceAccountEmail = "service@account.com"
				machineType = "large"
				userData = []byte("some-user-data")
				userDataSecretName = "userdata-secret-name"
				userDataSecretDataKey = "userdata-secret-key"
				subnetName = "subnet-nodes"

				volumeType = "normal"
				volumeSize = 20

				minCpuPlatform = "Foo"

				localVolumeType = VolumeTypeScratch
				localVolumeInterface = "SCSI"

				namePool1 = "pool-1"
				minPool1 = 5
				maxPool1 = 10
				maxSurgePool1 = intstr.FromInt(3)
				maxUnavailablePool1 = intstr.FromInt(2)

				namePool2 = "pool-2"
				minPool2 = 30
				maxPool2 = 45
				priorityPool2 = 100
				maxSurgePool2 = intstr.FromInt(10)
				maxUnavailablePool2 = intstr.FromInt(15)

				namePool3 = "pool-3"
				minPool3 = 5
				maxPool3 = 10
				maxSurgePool3 = intstr.FromInt(3)
				maxUnavailablePool3 = intstr.FromInt(2)

				zone1 = region + "a"
				zone2 = region + "b"

				archAMD = "amd64"
				archARM = "arm64"
				archFAKE = "fake"

				acceleratorTypeName = "FooAccelerator"
				acceleratorCount = 1

				poolLabels = map[string]string{"component": "TiDB"}

				nodeCapacity = corev1.ResourceList{
					"cpu":    resource.MustParse("8"),
					"gpu":    resource.MustParse("0"),
					"memory": resource.MustParse("128Gi"),
				}
				nodeTemplatePool1Zone1 = machinev1alpha1.NodeTemplate{
					Capacity:     InitializeCapacity(nodeCapacity, acceleratorCount),
					InstanceType: machineType,
					Region:       region,
					Zone:         zone1,
					Architecture: &archAMD,
				}

				nodeTemplatePool1Zone2 = machinev1alpha1.NodeTemplate{
					Capacity:     InitializeCapacity(nodeCapacity, acceleratorCount),
					InstanceType: machineType,
					Region:       region,
					Zone:         zone2,
					Architecture: &archAMD,
				}

				nodeTemplatePool2Zone1 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         zone1,
					Architecture: &archARM,
				}

				nodeTemplatePool2Zone2 = machinev1alpha1.NodeTemplate{
					Capacity:     nodeCapacity,
					InstanceType: machineType,
					Region:       region,
					Zone:         zone2,
					Architecture: &archARM,
				}

				nodeTemplatePool3Zone1 = machinev1alpha1.NodeTemplate{
					Capacity:     InitializeCapacity(nodeCapacity, acceleratorCount),
					InstanceType: machineType,
					Region:       region,
					Zone:         zone1,
					Architecture: ptr.To(archAMD),
				}

				nodeTemplatePool3Zone2 = machinev1alpha1.NodeTemplate{
					Capacity:     InitializeCapacity(nodeCapacity, acceleratorCount),
					InstanceType: machineType,
					Region:       region,
					Zone:         zone2,
					Architecture: ptr.To(archAMD),
				}

				machineConfiguration = &machinev1alpha1.MachineConfiguration{}

				shootVersionMajorMinor = "1.30"
				shootVersion = shootVersionMajorMinor + ".14"

				clusterWithoutImages = &extensionscontroller.Cluster{
					Shoot: &gardencorev1beta1.Shoot{
						Spec: gardencorev1beta1.ShootSpec{
							Kubernetes: gardencorev1beta1.Kubernetes{
								Version: shootVersion,
							},
							Networking: &gardencorev1beta1.Networking{IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}},
						},
						Status: gardencorev1beta1.ShootStatus{
							TechnicalID: technicalID,
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
									Architecture: ptr.To(archAMD),
								},
							},
						},
						{
							Name: machineImageName,
							Versions: []apiv1alpha1.MachineImageVersion{
								{
									Version:      machineImageVersion,
									Image:        machineImage,
									Architecture: ptr.To(archARM),
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
							Raw: encode(&apisgcp.InfrastructureStatus{
								ServiceAccountEmail: serviceAccountEmail,
								Networks: apisgcp.NetworkStatus{
									Subnets: []apisgcp.Subnet{
										{
											Name:    subnetName,
											Purpose: apisgcp.PurposeNodes,
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
								Architecture:   ptr.To(archAMD),
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserDataSecretRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
									Key:                  userDataSecretDataKey,
								},
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
									Raw: encode(&apisgcp.WorkerConfig{
										Volume: &apisgcp.Volume{
											LocalSSDInterface: &localVolumeInterface,
										},
										MinCpuPlatform: &minCpuPlatform,
										GPU: &apisgcp.GPU{
											AcceleratorType: acceleratorTypeName,
											Count:           acceleratorCount,
										},
									}),
								},
								Zones: []string{
									zone1,
									zone2,
								},
								Labels: poolLabels,
							},
							{
								Name:           namePool2,
								Minimum:        minPool2,
								Architecture:   ptr.To(archARM),
								Maximum:        maxPool2,
								Priority:       ptr.To(priorityPool2),
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
									Raw: encode(&apisgcp.WorkerConfig{
										Volume: &apisgcp.Volume{
											LocalSSDInterface: &localVolumeInterface,
										},
										ServiceAccount: &apisgcp.ServiceAccount{
											Email:  "foo",
											Scopes: []string{"bar"},
										},
										MinCpuPlatform: &minCpuPlatform,
									}),
								},
								UserDataSecretRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
									Key:                  userDataSecretDataKey,
								},
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
								Labels: poolLabels,
							},
							{
								Name:              namePool3,
								Minimum:           minPool3,
								Maximum:           maxPool3,
								MaxSurge:          maxSurgePool3,
								MaxUnavailable:    maxUnavailablePool3,
								MachineType:       machineType,
								Architecture:      ptr.To(archAMD),
								UpdateStrategy:    ptr.To(gardencorev1beta1.AutoInPlaceUpdate),
								KubernetesVersion: ptr.To(shootVersion),
								MachineImage: extensionsv1alpha1.MachineImage{
									Name:    machineImageName,
									Version: machineImageVersion,
								},
								NodeTemplate: &extensionsv1alpha1.NodeTemplate{
									Capacity: nodeCapacity,
								},
								UserDataSecretRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
									Key:                  userDataSecretDataKey,
								},
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
									Raw: encode(&apisgcp.WorkerConfig{
										Volume: &apisgcp.Volume{
											LocalSSDInterface: &localVolumeInterface,
										},
										MinCpuPlatform: &minCpuPlatform,
										GPU: &apisgcp.GPU{
											AcceleratorType: acceleratorTypeName,
											Count:           acceleratorCount,
										},
									}),
								},
								Zones: []string{
									zone1,
									zone2,
								},
								Labels: poolLabels,
							},
						},
					},
				}

				additionalData1 := []string{fmt.Sprintf("%dGi", volumeSize), minCpuPlatform, acceleratorTypeName, strconv.Itoa(int(acceleratorCount)), localVolumeInterface}
				additionalData2 := []string{fmt.Sprintf("%dGi", volumeSize), minCpuPlatform, "foo", "bar", localVolumeInterface}
				workerPoolHash1, _ = worker.WorkerPoolHash(w.Spec.Pools[0], cluster, []string{}, additionalData1, nil)
				workerPoolHash2, _ = worker.WorkerPoolHash(w.Spec.Pools[1], cluster, []string{}, additionalData2, nil)
				workerPoolHash3, _ = worker.WorkerPoolHash(w.Spec.Pools[2], cluster, nil, nil, nil)

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, clusterWithoutImages)
			})

			expectedUserDataSecretRefRead := func() {
				c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: userDataSecretName}, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, secret *corev1.Secret, _ ...client.GetOption) error {
						secret.Data = map[string][]byte{userDataSecretDataKey: userData}
						return nil
					},
				).AnyTimes()
			}

			Describe("machine images", func() {
				var (
					defaultMachineClass map[string]interface{}
					machineDeployments  worker.MachineDeployments
					machineClasses      map[string]interface{}
				)

				setup := func(disableExternalIP bool) {
					instanceLabels := map[string]interface{}{
						"name":             name,
						"k8s-cluster-name": technicalID,
					}
					for k, v := range poolLabels {
						instanceLabels[SanitizeGcpLabel(k)] = SanitizeGcpLabelValue(v)
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
								"labels":     instanceLabels,
							},
							{
								"autoDelete": true,
								"boot":       false,
								"sizeGb":     volumeSize,
								"type":       localVolumeType,
								"interface":  localVolumeInterface,
								"labels":     instanceLabels,
							},
						},
						"labels": instanceLabels,
						"metadata": []map[string]string{
							{
								"key":   "block-project-ssh-keys",
								"value": "TRUE",
							},
						},
						"machineType":    machineType,
						"minCpuPlatform": minCpuPlatform,
						"networkInterfaces": []map[string]interface{}{
							{
								"subnetwork":        subnetName,
								"disableExternalIP": disableExternalIP,
							},
						},
						"scheduling": map[string]interface{}{
							"automaticRestart":  true,
							"onHostMaintenance": "TERMINATE",
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
							technicalID,
							fmt.Sprintf("kubernetes-io-cluster-%s", technicalID),
							"kubernetes-io-role-node",
						},
						"operatingSystem": map[string]interface{}{
							"operatingSystemName":    machineImageName,
							"operatingSystemVersion": strings.ReplaceAll(machineImageVersion, "+", "_"),
						},
						"gpu": map[string]interface{}{
							"acceleratorType": acceleratorTypeName,
							"count":           acceleratorCount,
						},
					}

					// Copy default case and prepare the copy with the differences to the defaults above
					machineClassPool2 := useDefaultMachineClass(
						defaultMachineClass,
						"serviceAccounts",
						[]map[string]interface{}{{"email": "foo", "scopes": []string{"bar"}}},
					)
					machineClassPool2["scheduling"] = map[string]interface{}{"automaticRestart": true, "onHostMaintenance": "MIGRATE", "preemptible": false}
					delete(machineClassPool2, "gpu")

					var (
						machineClassPool1Zone1 = useDefaultMachineClass(defaultMachineClass, "zone", zone1)
						machineClassPool1Zone2 = useDefaultMachineClass(defaultMachineClass, "zone", zone2)
						machineClassPool2Zone1 = useDefaultMachineClass(machineClassPool2, "zone", zone1)
						machineClassPool2Zone2 = useDefaultMachineClass(machineClassPool2, "zone", zone2)
						machineClassPool3Zone1 = useDefaultMachineClass(defaultMachineClass, "zone", zone1)
						machineClassPool3Zone2 = useDefaultMachineClass(defaultMachineClass, "zone", zone2)

						machineClassNamePool1Zone1 = fmt.Sprintf("%s-%s-z1", technicalID, namePool1)
						machineClassNamePool1Zone2 = fmt.Sprintf("%s-%s-z2", technicalID, namePool1)
						machineClassNamePool2Zone1 = fmt.Sprintf("%s-%s-z1", technicalID, namePool2)
						machineClassNamePool2Zone2 = fmt.Sprintf("%s-%s-z2", technicalID, namePool2)
						machineClassNamePool3Zone1 = fmt.Sprintf("%s-%s-z1", technicalID, namePool3)
						machineClassNamePool3Zone2 = fmt.Sprintf("%s-%s-z2", technicalID, namePool3)

						machineClassWithHashPool1Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone1, workerPoolHash1)
						machineClassWithHashPool1Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool1Zone2, workerPoolHash1)
						machineClassWithHashPool2Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone1, workerPoolHash2)
						machineClassWithHashPool2Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool2Zone2, workerPoolHash2)
						machineClassWithHashPool3Zone1 = fmt.Sprintf("%s-%s", machineClassNamePool3Zone1, workerPoolHash3)
						machineClassWithHashPool3Zone2 = fmt.Sprintf("%s-%s", machineClassNamePool3Zone2, workerPoolHash3)
					)

					addNameAndSecretToMachineClass(machineClassPool1Zone1, machineClassWithHashPool1Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool1Zone2, machineClassWithHashPool1Zone2, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone1, machineClassWithHashPool2Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool2Zone2, machineClassWithHashPool2Zone2, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool3Zone1, machineClassWithHashPool3Zone1, w.Spec.SecretRef)
					addNameAndSecretToMachineClass(machineClassPool3Zone2, machineClassWithHashPool3Zone2, w.Spec.SecretRef)

					addNodeTemplateToMachineClass(machineClassPool1Zone1, nodeTemplatePool1Zone1)
					addNodeTemplateToMachineClass(machineClassPool1Zone2, nodeTemplatePool1Zone2)
					addNodeTemplateToMachineClass(machineClassPool2Zone1, nodeTemplatePool2Zone1)
					addNodeTemplateToMachineClass(machineClassPool2Zone2, nodeTemplatePool2Zone2)
					addNodeTemplateToMachineClass(machineClassPool3Zone1, nodeTemplatePool3Zone1)
					addNodeTemplateToMachineClass(machineClassPool3Zone2, nodeTemplatePool3Zone2)

					machineClasses = map[string]interface{}{"machineClasses": []map[string]interface{}{
						machineClassPool1Zone1,
						machineClassPool1Zone2,
						machineClassPool2Zone1,
						machineClassPool2Zone2,
						machineClassPool3Zone1,
						machineClassPool3Zone2,
					}}

					emptyClusterAutoscalerAnnotations := map[string]string{
						"autoscaler.gardener.cloud/max-node-provision-time":              "",
						"autoscaler.gardener.cloud/scale-down-gpu-utilization-threshold": "",
						"autoscaler.gardener.cloud/scale-down-unneeded-time":             "",
						"autoscaler.gardener.cloud/scale-down-unready-time":              "",
						"autoscaler.gardener.cloud/scale-down-utilization-threshold":     "",
					}

					labelsZone1 := utils.MergeStringMaps(poolLabels, map[string]string{gcp.CSIDiskDriverTopologyKey: zone1})
					labelsZone2 := utils.MergeStringMaps(poolLabels, map[string]string{gcp.CSIDiskDriverTopologyKey: zone2})
					machineDeployments = worker.MachineDeployments{
						{
							Name:       machineClassNamePool1Zone1,
							PoolName:   namePool1,
							ClassName:  machineClassWithHashPool1Zone1,
							SecretName: machineClassWithHashPool1Zone1,
							Minimum:    worker.DistributeOverZones(0, minPool1, 2),
							Maximum:    worker.DistributeOverZones(0, maxPool1, 2),
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(0, maxSurgePool1, 2, maxPool1)),
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(0, maxUnavailablePool1, 2, minPool1)),
									},
								},
							},
							Labels:                       labelsZone1,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool1Zone2,
							PoolName:   namePool1,
							ClassName:  machineClassWithHashPool1Zone2,
							SecretName: machineClassWithHashPool1Zone2,
							Minimum:    worker.DistributeOverZones(1, minPool1, 2),
							Maximum:    worker.DistributeOverZones(1, maxPool1, 2),
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(1, maxSurgePool1, 2, maxPool1)),
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(1, maxUnavailablePool1, 2, minPool1)),
									},
								},
							},
							Labels:                       labelsZone2,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool2Zone1,
							PoolName:   namePool2,
							ClassName:  machineClassWithHashPool2Zone1,
							SecretName: machineClassWithHashPool2Zone1,
							Minimum:    worker.DistributeOverZones(0, minPool2, 2),
							Maximum:    worker.DistributeOverZones(0, maxPool2, 2),
							Priority:   ptr.To(priorityPool2),
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(0, maxSurgePool2, 2, maxPool2)),
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(0, maxUnavailablePool2, 2, minPool2)),
									},
								},
							},
							Labels:                       labelsZone1,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool2Zone2,
							PoolName:   namePool2,
							ClassName:  machineClassWithHashPool2Zone2,
							SecretName: machineClassWithHashPool2Zone2,
							Minimum:    worker.DistributeOverZones(1, minPool2, 2),
							Maximum:    worker.DistributeOverZones(1, maxPool2, 2),
							Priority:   ptr.To(priorityPool2),
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
								RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(1, maxSurgePool2, 2, maxPool2)),
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(1, maxUnavailablePool2, 2, minPool2)),
									},
								},
							},
							Labels:                       labelsZone2,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool3Zone1,
							PoolName:   namePool3,
							ClassName:  machineClassWithHashPool3Zone1,
							SecretName: machineClassWithHashPool3Zone1,
							Minimum:    worker.DistributeOverZones(0, minPool3, 2),
							Maximum:    worker.DistributeOverZones(0, maxPool3, 2),
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
								InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(0, maxSurgePool3, 2, maxPool3)),
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(0, maxUnavailablePool3, 2, minPool3)),
									},
									OrchestrationType: machinev1alpha1.OrchestrationTypeAuto,
								},
							},
							Labels:                       labelsZone1,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
						{
							Name:       machineClassNamePool3Zone2,
							PoolName:   namePool3,
							ClassName:  machineClassWithHashPool3Zone2,
							SecretName: machineClassWithHashPool3Zone2,
							Minimum:    worker.DistributeOverZones(1, minPool3, 2),
							Maximum:    worker.DistributeOverZones(1, maxPool3, 2),
							Strategy: machinev1alpha1.MachineDeploymentStrategy{
								Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
								InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
									UpdateConfiguration: machinev1alpha1.UpdateConfiguration{
										MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(1, maxSurgePool3, 2, maxPool3)),
										MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(1, maxUnavailablePool3, 2, minPool3)),
									},
									OrchestrationType: machinev1alpha1.OrchestrationTypeAuto,
								},
							},
							Labels:                       labelsZone2,
							MachineConfiguration:         machineConfiguration,
							ClusterAutoscalerAnnotations: emptyClusterAutoscalerAnnotations,
						},
					}
				}

				It("should return the expected machine deployments when disableExternal IP is true with profile image types", func() {
					setup(true)
					workerCloudRouter := w
					workerCloudRouter.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
						Raw: encode(&apisgcp.InfrastructureStatus{
							ServiceAccountEmail: serviceAccountEmail,
							Networks: apisgcp.NetworkStatus{
								VPC: apisgcp.VPC{
									CloudRouter: &apisgcp.CloudRouter{
										Name: "my-cloudrouter",
									},
								},
								Subnets: []apisgcp.Subnet{
									{
										Name:    subnetName,
										Purpose: apisgcp.PurposeNodes,
									},
								},
							},
						}),
					}
					workerDelegateCloudRouter, _ := NewWorkerDelegate(c, scheme, chartApplier, "", workerCloudRouter, cluster)

					expectedUserDataSecretRefRead()

					// Test WorkerDelegate.DeployMachineClasses()
					chartApplier.EXPECT().ApplyFromEmbeddedFS(
						ctx,
						charts.InternalChart,
						filepath.Join("internal", "machineclass"),
						namespace,
						"machineclass",
						kubernetes.Values(machineClasses),
					)
					err := workerDelegateCloudRouter.DeployMachineClasses(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test WorkerDelegate.UpdateMachineDeployments()
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
								Architecture: ptr.To(archAMD),
							},
							{
								Name:         machineImageName,
								Version:      machineImageVersion,
								Image:        machineImage,
								Architecture: ptr.To(archARM),
							},
						},
					}
					workerWithExpectedImages := w.DeepCopy()
					workerWithExpectedImages.Status.ProviderStatus = &runtime.RawExtension{
						Object: expectedImages,
					}
					c.EXPECT().Status().Return(statusWriter)
					statusWriter.EXPECT().Patch(ctx, workerWithExpectedImages, gomock.Any()).Return(nil)

					err = workerDelegateCloudRouter.UpdateMachineImagesStatus(ctx)
					Expect(err).NotTo(HaveOccurred())

					// Test WorkerDelegate.GenerateMachineDeployments()
					result, err := workerDelegateCloudRouter.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(machineDeployments), "diff: %s", cmp.Diff(machineDeployments, result))
				})
			})

			It("should succeed with ipv4 cluster", func() {
				cluster.Shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}
				wd, err := NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)
				Expect(err).NotTo(HaveOccurred())
				expectedUserDataSecretRefRead()
				_, err = wd.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				workerDelegate := wd.(*WorkerDelegate)
				mClasses := workerDelegate.GetMachineClasses()
				for _, mClz := range mClasses {
					Expect(mClz["networkInterfaces"]).To(Equal([]map[string]interface{}{{
						"subnetwork":        subnetName,
						"disableExternalIP": true,
					}}))
				}
			})

			It("should succeed with dual-stack cluster", func() {
				cluster.Shoot.Spec.Networking.IPFamilies = []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6}
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&apisgcp.InfrastructureStatus{
						ServiceAccountEmail: serviceAccountEmail,
						Networks: apisgcp.NetworkStatus{
							Subnets: []apisgcp.Subnet{
								{
									Name:    subnetName,
									Purpose: apisgcp.PurposeNodes,
								},
							},
							IPFamilies: []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4, gardencorev1beta1.IPFamilyIPv6},
						},
					}),
				}
				wd, err := NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)
				Expect(err).NotTo(HaveOccurred())
				expectedUserDataSecretRefRead()
				_, err = wd.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				workerDelegate := wd.(*WorkerDelegate)
				mClasses := workerDelegate.GetMachineClasses()
				for _, mClz := range mClasses {
					Expect(mClz["networkInterfaces"]).To(Equal([]map[string]interface{}{{
						"subnetwork":          subnetName,
						"disableExternalIP":   true,
						"stackType":           "IPV4_IPV6",
						"ipv6accessType":      "EXTERNAL",
						"ipCidrRange":         "/24",
						"subnetworkRangeName": "ipv4-pod-cidr",
						"useAliasIPs":         true,
					}}))
				}
			})

			It("should fail because the version is invalid", func() {
				clusterWithoutImages.Shoot.Spec.Kubernetes.Version = "invalid"
				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the infrastructure status cannot be decoded", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{}

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the nodes subnet cannot be found", func() {
				w.Spec.InfrastructureProviderStatus = &runtime.RawExtension{
					Raw: encode(&apisgcp.InfrastructureStatus{}),
				}

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image for given architecture cannot be found", func() {
				w.Spec.Pools[0].Architecture = ptr.To(archFAKE)

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the machine image cannot be found", func() {
				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, clusterWithoutImages)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should fail because the volume size cannot be decoded", func() {
				w.Spec.Pools[0].Volume.Size = "not-decodeable"

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
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

				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)

				expectedUserDataSecretRefRead()

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				resultSettings := result[0].MachineConfiguration
				resultNodeConditions := strings.Join(testNodeConditions, ",")

				Expect(err).NotTo(HaveOccurred())
				Expect(resultSettings.MachineDrainTimeout).To(Equal(&testDrainTimeout))
				Expect(resultSettings.MachineCreationTimeout).To(Equal(&testCreationTimeout))
				Expect(resultSettings.MachineHealthTimeout).To(Equal(&testHealthTimeout))
				Expect(resultSettings.MaxEvictRetries).To(Equal(&testMaxEvictRetries))
				Expect(resultSettings.NodeConditions).To(Equal(&resultNodeConditions))
			})

			It("should return generate machine classes with core and extended resources in the nodeTemplate", func() {
				ephemeralStorageQuant := resource.MustParse("30Gi")
				dongleName := corev1.ResourceName("resources.com/dongle")
				dongleQuant := resource.MustParse("4")
				customResources := corev1.ResourceList{
					corev1.ResourceEphemeralStorage: ephemeralStorageQuant,
					dongleName:                      dongleQuant,
				}
				w.Spec.Pools[0].ProviderConfig = &runtime.RawExtension{
					Raw: encode(&apisgcp.WorkerConfig{
						NodeTemplate: &extensionsv1alpha1.NodeTemplate{
							Capacity: customResources,
						},
					}),
				}

				expectedCapacity := w.Spec.Pools[0].NodeTemplate.Capacity.DeepCopy()
				maps.Copy(expectedCapacity, customResources)

				wd, err := NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)
				Expect(err).NotTo(HaveOccurred())
				expectedUserDataSecretRefRead()
				_, err = wd.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				workerDelegate := wd.(*WorkerDelegate)
				mClasses := workerDelegate.GetMachineClasses()
				for _, mClz := range mClasses {
					className := mClz["name"].(string)
					if strings.Contains(className, namePool1) {
						nt := mClz["nodeTemplate"].(machinev1alpha1.NodeTemplate)
						Expect(nt.Capacity).To(Equal(expectedCapacity))
					}
				}
			})

			It("should set expected cluster-autoscaler annotations on the machine deployment", func() {
				w.Spec.Pools[0].ClusterAutoscaler = &extensionsv1alpha1.ClusterAutoscalerOptions{
					MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: time.Minute}),
					ScaleDownGpuUtilizationThreshold: ptr.To("0.4"),
					ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: 2 * time.Minute}),
					ScaleDownUnreadyTime:             ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
					ScaleDownUtilizationThreshold:    ptr.To("0.5"),
				}
				w.Spec.Pools[1].ClusterAutoscaler = nil
				workerDelegate, _ = NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)

				expectedUserDataSecretRefRead()

				result, err := workerDelegate.GenerateMachineDeployments(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).NotTo(BeNil())

				Expect(result[0].ClusterAutoscalerAnnotations).NotTo(BeNil())
				Expect(result[1].ClusterAutoscalerAnnotations).NotTo(BeNil())

				for k, v := range result[2].ClusterAutoscalerAnnotations {
					Expect(v).To(BeEmpty(), "entry for key %v is not empty", k)
				}
				for k, v := range result[3].ClusterAutoscalerAnnotations {
					Expect(v).To(BeEmpty(), "entry for key %v is not empty", k)
				}

				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation]).To(Equal("1m0s"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation]).To(Equal("0.4"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation]).To(Equal("2m0s"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation]).To(Equal("3m0s"))
				Expect(result[0].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation]).To(Equal("0.5"))

				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation]).To(Equal("1m0s"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation]).To(Equal("0.4"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation]).To(Equal("2m0s"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation]).To(Equal("3m0s"))
				Expect(result[1].ClusterAutoscalerAnnotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation]).To(Equal("0.5"))
			})

			DescribeTable("Set correct onHostMaintenance option for specific machine types",
				func(machineType string, expectedOnHostMaintenance string) {
					w.Spec.Pools = []extensionsv1alpha1.WorkerPool{
						{
							Name:           namePool1,
							Minimum:        minPool1,
							Maximum:        maxPool1,
							MaxSurge:       maxSurgePool1,
							MaxUnavailable: maxUnavailablePool1,
							MachineType:    machineType,
							Architecture:   ptr.To(archAMD),
							MachineImage: extensionsv1alpha1.MachineImage{
								Name:    machineImageName,
								Version: machineImageVersion,
							},
							NodeTemplate: &extensionsv1alpha1.NodeTemplate{
								Capacity: nodeCapacity,
							},
							UserDataSecretRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: userDataSecretName},
								Key:                  userDataSecretDataKey,
							},
							Volume: &extensionsv1alpha1.Volume{
								Type: &volumeType,
								Size: fmt.Sprintf("%dGi", volumeSize),
							},
							Zones: []string{
								zone1,
							},
							Labels: poolLabels,
						},
					}

					wd, err := NewWorkerDelegate(c, scheme, chartApplier, "", w, cluster)
					Expect(err).NotTo(HaveOccurred())
					expectedUserDataSecretRefRead()
					_, err = wd.GenerateMachineDeployments(ctx)
					Expect(err).NotTo(HaveOccurred())
					workerDelegate := wd.(*WorkerDelegate)
					mClasses := workerDelegate.GetMachineClasses()
					Expect(mClasses).To(HaveLen(1))
					scheduling, ok := mClasses[0]["scheduling"].(map[string]interface{})
					Expect(ok).To(BeTrue())
					Expect(scheduling["onHostMaintenance"]).To(Equal(expectedOnHostMaintenance))
					Expect(scheduling["automaticRestart"]).To(BeTrue())
					Expect(scheduling["preemptible"]).To(BeFalse())
				},
				Entry("MIGRATE for general purpose machine types", "n4-standard-8", "MIGRATE"),
				Entry("TERMINATE for bare metal machine types", "c3-highcpu-192-metal", "TERMINATE"),
				Entry("TERMINATE for H4D machine types", "h4d-standard-192", "TERMINATE"),
				Entry("MIGRATE for Z3 machines with low storage", "z3-highmem-8-highlssd", "MIGRATE"),
				Entry("TERMINATE for Z3 machines with high storage", "z3-highmem-88-highlssd", "TERMINATE"),
				Entry("TERMINATE for machines with TPUs", "ct6e-standard-4t", "TERMINATE"),
			)
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
		v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass,
	}
	class["credentialsSecretRef"] = map[string]interface{}{
		"name":      credentialsSecretRef.Name,
		"namespace": credentialsSecretRef.Namespace,
	}
}
