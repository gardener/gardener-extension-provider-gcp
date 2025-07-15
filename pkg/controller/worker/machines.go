// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	computev1 "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/charts"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpapihelper "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var labelRegex = regexp.MustCompile(`[^a-z0-9_-]`)

// InitializeCapacity is a handle to make the function accessible to the tests.
var InitializeCapacity = initializeCapacity

const (
	persistentDiskExtreme     = "pd-extreme"
	hyperDiskBalanced         = "hyperdisk-balanced"
	hyperDiskExtreme          = "hyperdisk-extreme"
	hyperDiskThroughput       = "hyperdisk-throughput"
	maxGcpLabelCharactersSize = 63
	// ResourceGPU is the GPU resource. It should be a non-negative integer.
	ResourceGPU corev1.ResourceName = "gpu"
	// VolumeTypeScratch is the gcp SCRATCH volume type
	VolumeTypeScratch = "SCRATCH"
)

var (
	// AllowedTypesIops are the volume types for which iops can be configured
	AllowedTypesIops = []string{persistentDiskExtreme, hyperDiskExtreme, hyperDiskBalanced}
	// AllowedTypesThroughput are the volume types for which throughput can be configured
	AllowedTypesThroughput = []string{hyperDiskThroughput, hyperDiskBalanced}
)

// MachineClassKind yields the name of the machine class kind used by GCP provider.
func (w *WorkerDelegate) MachineClassKind() string {
	return "MachineClass"
}

// MachineClass yields a newly initialized machine class object.
func (w *WorkerDelegate) MachineClass() client.Object {
	return &machinev1alpha1.MachineClass{}
}

// MachineClassList yields a newly initialized MachineClassList object.
func (w *WorkerDelegate) MachineClassList() client.ObjectList {
	return &machinev1alpha1.MachineClassList{}
}

// DeployMachineClasses generates and creates the GCP specific machine classes.
func (w *WorkerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}

	return w.seedChartApplier.ApplyFromEmbeddedFS(ctx, charts.InternalChart, filepath.Join(charts.InternalChartsPath, "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
}

// GenerateMachineDeployments generates the configuration for the desired machine deployments.
func (w *WorkerDelegate) GenerateMachineDeployments(ctx context.Context) (worker.MachineDeployments, error) {
	if w.machineDeployments == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return nil, err
		}
	}
	return w.machineDeployments, nil
}

func formatNodeCIDRMask(val *int32, defaultVal int) string {
	return fmt.Sprintf("/%d", ptr.Deref(val, int32(defaultVal))) // #nosec: G115 -- netmask is limited in size
}

func (w *WorkerDelegate) generateMachineConfig(ctx context.Context) error {
	var (
		machineDeployments = worker.MachineDeployments{}
		machineClasses     []map[string]interface{}
		machineImages      []apisgcp.MachineImage
	)

	infrastructureStatus := &apisgcp.InfrastructureStatus{}
	if _, _, err := w.decoder.Decode(w.worker.Spec.InfrastructureProviderStatus.Raw, nil, infrastructureStatus); err != nil {
		return err
	}

	nodesSubnet, err := gcpapihelper.FindSubnetByPurpose(infrastructureStatus.Networks.Subnets, apisgcp.PurposeNodes)
	if err != nil {
		return err
	}

	for _, pool := range w.worker.Spec.Pools {
		zoneLen := int32(len(pool.Zones)) // #nosec: G115 - We check if pool zones exceeds max_int32.

		workerConfig := &apisgcp.WorkerConfig{}
		if pool.ProviderConfig != nil && pool.ProviderConfig.Raw != nil {
			if _, _, err := w.decoder.Decode(pool.ProviderConfig.Raw, nil, workerConfig); err != nil {
				return fmt.Errorf("could not decode provider config: %+v", err)
			}
		}

		workerPoolHash, err := w.generateWorkerPoolHash(pool, *workerConfig)
		if err != nil {
			return err
		}

		poolLabels := getGcePoolLabels(w.worker, pool)

		arch := ptr.Deref(pool.Architecture, v1beta1constants.ArchitectureAMD64)
		machineImage, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version, &arch)
		if err != nil {
			return err
		}

		machineImages = appendMachineImage(machineImages, apisgcp.MachineImage{
			Name:         pool.MachineImage.Name,
			Version:      pool.MachineImage.Version,
			Image:        machineImage,
			Architecture: &arch,
		})

		disks := make([]map[string]interface{}, 0)
		// root volume
		if pool.Volume != nil {
			disk, err := createDiskSpecForVolume(pool.Volume, machineImage, workerConfig, poolLabels)
			if err != nil {
				return err
			}
			disks = append(disks, disk)
		}

		// additional volumes
		for _, volume := range pool.DataVolumes {
			disk, err := createDiskSpecForDataVolume(volume, workerConfig, poolLabels)
			if err != nil {
				return err
			}
			disks = append(disks, disk)
		}

		serviceAccounts := make([]map[string]interface{}, 0)
		if workerConfig.ServiceAccount != nil {
			serviceAccounts = append(serviceAccounts, map[string]interface{}{
				"email":  workerConfig.ServiceAccount.Email,
				"scopes": workerConfig.ServiceAccount.Scopes,
			})
		} else if len(infrastructureStatus.ServiceAccountEmail) != 0 {
			serviceAccounts = append(serviceAccounts, map[string]interface{}{
				"email":  infrastructureStatus.ServiceAccountEmail,
				"scopes": []string{computev1.ComputeScope},
			})
		}

		isLiveMigrationAllowed := true

		userData, err := worker.FetchUserData(ctx, w.client, w.worker.Namespace, pool)
		if err != nil {
			return err
		}

		for zoneIndex, zone := range pool.Zones {
			zoneIdx := int32(zoneIndex) // #nosec: G115 - We check if pool zones exceeds max_int32.
			ipCidrRange := formatNodeCIDRMask(nil, 24)

			if kcc := w.cluster.Shoot.Spec.Kubernetes.KubeControllerManager; kcc != nil {
				ipCidrRange = formatNodeCIDRMask(kcc.NodeCIDRMaskSize, 24)
			}

			machineClassSpec := map[string]interface{}{
				"region":             w.worker.Spec.Region,
				"zone":               zone,
				"canIpForward":       true,
				"deletionProtection": false,
				"description":        fmt.Sprintf("Machine of Shoot %s created by machine-controller-manager.", w.worker.Name),
				"disks":              disks,
				"labels":             poolLabels,
				// TODO: make this configurable for the user
				"metadata": []map[string]string{
					{
						"key":   "block-project-ssh-keys",
						"value": "TRUE",
					},
				},
				"machineType": pool.MachineType,
				"networkInterfaces": []map[string]interface{}{
					{
						"subnetwork":        nodesSubnet.Name,
						"disableExternalIP": true,
					},
				},
				"secret": map[string]interface{}{
					"cloudConfig": string(userData),
				},
				"credentialsSecretRef": map[string]interface{}{
					"name":      w.worker.Spec.SecretRef.Name,
					"namespace": w.worker.Spec.SecretRef.Namespace,
				},
				"serviceAccounts": serviceAccounts,
				"tags": []string{
					w.worker.Namespace,
					fmt.Sprintf("kubernetes-io-cluster-%s", w.worker.Namespace),
					"kubernetes-io-role-node",
				},
			}

			if !gardencorev1beta1.IsIPv4SingleStack(infrastructureStatus.Networks.IPFamilies) {
				machineClassSpec["networkInterfaces"] = []map[string]interface{}{
					{
						"subnetwork":          nodesSubnet.Name,
						"disableExternalIP":   true,
						"stackType":           w.getStackType(),
						"ipv6accessType":      "EXTERNAL",
						"ipCidrRange":         ipCidrRange,
						"subnetworkRangeName": infraflow.DefaultSecondarySubnetName,
					},
				}
			}

			var (
				deploymentName = fmt.Sprintf("%s-%s-z%d", w.worker.Namespace, pool.Name, zoneIndex+1)
				className      = fmt.Sprintf("%s-%s", deploymentName, workerPoolHash)
				gpuCount       int32
			)

			updateConfiguration := machinev1alpha1.UpdateConfiguration{
				MaxSurge:       ptr.To(worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxSurge, zoneLen, pool.Maximum)),
				MaxUnavailable: ptr.To(worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxUnavailable, zoneLen, pool.Minimum)),
			}

			machineDeploymentStrategy := machinev1alpha1.MachineDeploymentStrategy{
				Type: machinev1alpha1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: &machinev1alpha1.RollingUpdateMachineDeployment{
					UpdateConfiguration: updateConfiguration,
				},
			}

			if gardencorev1beta1helper.IsUpdateStrategyInPlace(pool.UpdateStrategy) {
				machineDeploymentStrategy = machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
						UpdateConfiguration: updateConfiguration,
						OrchestrationType:   machinev1alpha1.OrchestrationTypeAuto,
					},
				}

				if gardencorev1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
					machineDeploymentStrategy.InPlaceUpdate.OrchestrationType = machinev1alpha1.OrchestrationTypeManual
				}
			}

			machineDeployments = append(machineDeployments, worker.MachineDeployment{
				Name:                         deploymentName,
				PoolName:                     pool.Name,
				ClassName:                    className,
				SecretName:                   className,
				Minimum:                      worker.DistributeOverZones(zoneIdx, pool.Minimum, zoneLen),
				Maximum:                      worker.DistributeOverZones(zoneIdx, pool.Maximum, zoneLen),
				Strategy:                     machineDeploymentStrategy,
				Priority:                     pool.Priority,
				Labels:                       addTopologyLabel(pool.Labels, zone),
				Annotations:                  pool.Annotations,
				Taints:                       pool.Taints,
				MachineConfiguration:         genericworkeractuator.ReadMachineConfiguration(pool),
				ClusterAutoscalerAnnotations: extensionsv1alpha1helper.GetMachineDeploymentClusterAutoscalerAnnotations(pool.ClusterAutoscaler),
			})

			machineClassSpec["name"] = className
			machineClassSpec["resourceLabels"] = map[string]string{
				v1beta1constants.GardenerPurpose: v1beta1constants.GardenPurposeMachineClass,
			}

			if pool.MachineImage.Name != "" && pool.MachineImage.Version != "" {
				machineClassSpec["operatingSystem"] = map[string]interface{}{
					"operatingSystemName":    pool.MachineImage.Name,
					"operatingSystemVersion": strings.ReplaceAll(pool.MachineImage.Version, "+", "_"),
				}
			}

			if workerConfig.GPU != nil {
				machineClassSpec["gpu"] = map[string]interface{}{
					"acceleratorType": workerConfig.GPU.AcceleratorType,
					"count":           workerConfig.GPU.Count,
				}
				// using this gpu count for scale-from-zero cases
				gpuCount = workerConfig.GPU.Count
			}

			if workerConfig.MinCpuPlatform != nil {
				machineClassSpec["minCpuPlatform"] = *workerConfig.MinCpuPlatform
			}

			nodeTemplate := pool.NodeTemplate.DeepCopy()
			if workerConfig.NodeTemplate != nil {
				// Support extended resources by copying into nodeTemplate.Capacity overriding if needed
				maps.Copy(nodeTemplate.Capacity, workerConfig.NodeTemplate.Capacity)
			}
			if nodeTemplate != nil {
				template := machinev1alpha1.NodeTemplate{
					// always overwrite the GPU count if it was provided in the WorkerConfig.
					Capacity:     initializeCapacity(nodeTemplate.Capacity, gpuCount),
					InstanceType: pool.MachineType,
					Region:       w.worker.Spec.Region,
					Zone:         zone,
					Architecture: ptr.To(arch),
				}
				machineClassSpec["nodeTemplate"] = template
				numGpus := template.Capacity[ResourceGPU]
				if !numGpus.IsZero() {
					isLiveMigrationAllowed = false
				}
			}

			setSchedulingPolicy(machineClassSpec, isLiveMigrationAllowed)
			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	w.machineDeployments = machineDeployments
	w.machineClasses = machineClasses
	w.machineImages = machineImages

	return nil
}

func (w *WorkerDelegate) generateWorkerPoolHash(pool v1alpha1.WorkerPool, _ apisgcp.WorkerConfig) (string, error) {
	var additionalData []string

	volumes := slices.Clone(pool.DataVolumes)
	slices.SortFunc(volumes, func(i, j v1alpha1.DataVolume) int {
		return strings.Compare(i.Name, j.Name)
	})
	for _, volume := range volumes {
		additionalData = append(additionalData, volume.Name, volume.Size)
		if volume.Type != nil {
			additionalData = append(additionalData, *volume.Type)
		}
		// We exclude volume.Encrypted from the hash calculation because GCP disks are encrypted by default,
		// and the field does not influence disk encryption behavior.
	}

	additionalDataV2 := append(additionalData, workerPoolHashDataV2(pool)...)

	return worker.WorkerPoolHash(pool, w.cluster, []string{}, additionalDataV2, []string{})
}

func workerPoolHashDataV2(pool v1alpha1.WorkerPool) []string {
	// in the future, we may not calculate a hash for the whole ProviderConfig
	// for example volume field changes could be done in place, but MCM needs to support it
	// see https://cloud.google.com/compute/docs/instances/update-instance-properties?hl=de#updatable-properties
	// for a list of properties that requires a restart.
	if pool.ProviderConfig != nil && pool.ProviderConfig.Raw != nil {
		return []string{string(pool.ProviderConfig.Raw)}
	}

	return []string{}
}

func createDiskSpecForVolume(volume *v1alpha1.Volume, image string, workerConfig *apisgcp.WorkerConfig, labels map[string]interface{}) (map[string]interface{}, error) {
	return createDiskSpec(volume.Size, true, &image, volume.Type, workerConfig.Volume, nil, labels)
}

func createDiskSpecForDataVolume(volume v1alpha1.DataVolume, workerConfig *apisgcp.WorkerConfig, labels map[string]interface{}) (map[string]interface{}, error) {
	dataVolumeConf := getDataVolumeWorkerConf(volume.Name, workerConfig.DataVolumes)
	return createDiskSpec(volume.Size, false, dataVolumeConf.SourceImage, volume.Type, workerConfig.Volume, &dataVolumeConf, labels)
}

func createDiskSpec(size string, boot bool, image, volumeType *string, volumeConf *apisgcp.Volume, dataVolumeConf *apisgcp.DataVolume, labels map[string]interface{}) (map[string]interface{}, error) {
	volumeSize, err := worker.DiskSize(size)
	if err != nil {
		return nil, err
	}

	disk := map[string]interface{}{
		"autoDelete": true,
		"boot":       boot,
		"sizeGb":     volumeSize,
	}

	if len(labels) != 0 {
		disk["labels"] = labels
	}

	// Careful when setting machine image for data volumes. Any pre-existing data on the disk can interfere with the boot disk.
	// See https://github.com/gardener/gardener-extension-provider-gcp/issues/323
	if image != nil {
		disk["image"] = *image
	}

	if volumeType != nil {
		disk["type"] = *volumeType
	}

	if volumeConf != nil {
		// Only add encryption details for non-scratch disks, checked by worker validation
		addDiskEncryptionDetails(disk, volumeConf.Encryption)
		// only allowed if volume type is SCRATCH - checked by worker validation
		if volumeConf.LocalSSDInterface != nil && *volumeType == VolumeTypeScratch {
			disk["interface"] = *volumeConf.LocalSSDInterface
		}
	}

	if dataVolumeConf != nil {
		if dataVolumeConf.ProvisionedIops != nil && slices.Contains(AllowedTypesIops, *volumeType) {
			disk["provisionedIops"] = *dataVolumeConf.ProvisionedIops
		}
		if dataVolumeConf.ProvisionedThroughput != nil && slices.Contains(AllowedTypesThroughput, *volumeType) {
			disk["provisionedThroughput"] = *dataVolumeConf.ProvisionedThroughput
		}
	}

	return disk, nil
}

func addDiskEncryptionDetails(disk map[string]interface{}, encryption *apisgcp.DiskEncryption) {
	if encryption == nil {
		return
	}
	encryptionMap := make(map[string]interface{})
	if encryption.KmsKeyName != nil {
		encryptionMap["kmsKeyName"] = *encryption.KmsKeyName
	}
	if encryption.KmsKeyServiceAccount != nil {
		encryptionMap["kmsKeyServiceAccount"] = *encryption.KmsKeyServiceAccount
	}
	disk["encryption"] = encryptionMap
}

func getDataVolumeWorkerConf(volumeName string, dataVolumes []apisgcp.DataVolume) apisgcp.DataVolume {
	for _, dv := range dataVolumes {
		if dv.Name == volumeName {
			return dv
		}
	}
	return apisgcp.DataVolume{}
}

func getGcePoolLabels(worker *v1alpha1.Worker, pool v1alpha1.WorkerPool) map[string]interface{} {
	gceInstanceLabels := map[string]interface{}{
		"name": SanitizeGcpLabelValue(worker.Name),
		// Add shoot id to keep consistency with the label added to all disks by the csi-driver
		"k8s-cluster-name": SanitizeGcpLabelValue(worker.Namespace),
	}
	for k, v := range pool.Labels {
		if label := SanitizeGcpLabel(k); label != "" {
			gceInstanceLabels[label] = SanitizeGcpLabelValue(v)
		}
	}
	return gceInstanceLabels
}

func initializeCapacity(capacityList corev1.ResourceList, gpuCount int32) corev1.ResourceList {
	resultCapacity := capacityList.DeepCopy()
	if gpuCount != 0 {
		resultCapacity[ResourceGPU] = *resource.NewQuantity(int64(gpuCount), resource.DecimalSI)
	}

	return resultCapacity
}

func setSchedulingPolicy(machineClassSpec map[string]interface{}, isLiveMigrationAllowed bool) {
	if isLiveMigrationAllowed {
		machineClassSpec["scheduling"] = map[string]interface{}{
			"automaticRestart":  true,
			"onHostMaintenance": "MIGRATE",
			"preemptible":       false,
		}
	} else {
		machineClassSpec["scheduling"] = map[string]interface{}{
			"automaticRestart":  true,
			"onHostMaintenance": "TERMINATE",
			"preemptible":       false,
		}
	}
}

// SanitizeGcpLabel will sanitize the label base on the gcp label Restrictions
func SanitizeGcpLabel(label string) string {
	return sanitizeGcpLabelOrValue(label, true)
}

// SanitizeGcpLabelValue will sanitize the value base on the gcp label Restrictions
func SanitizeGcpLabelValue(value string) string {
	return sanitizeGcpLabelOrValue(value, false)
}

// sanitizeGcpLabelOrValue will sanitize the label/value base on the gcp label Restrictions
func sanitizeGcpLabelOrValue(label string, startWithCharacter bool) string {
	v := labelRegex.ReplaceAllString(strings.ToLower(label), "_")
	if startWithCharacter {
		v = strings.TrimLeftFunc(v, func(r rune) bool {
			if ('0' <= r && r <= '9') || r == '_' {
				return true
			}
			return false
		})
	}
	if len(v) > maxGcpLabelCharactersSize {
		return v[0:maxGcpLabelCharactersSize]
	}
	return v
}

func addTopologyLabel(labels map[string]string, zone string) map[string]string {
	return utils.MergeStringMaps(labels, map[string]string{gcp.CSIDiskDriverTopologyKey: zone})
}
