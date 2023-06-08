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

package worker

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	computev1 "google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/charts"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpapihelper "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var labelRegex = regexp.MustCompile(`[^a-z0-9_-]`)

const (
	maxGcpLabelCharactersSize = 63
	// ResourceGPU is the GPU resource . It should be a non-negative integer.
	ResourceGPU v1.ResourceName = "gpu"
)

// MachineClassKind yields the name of the machine class kind used by GCP provider.
func (w *workerDelegate) MachineClassKind() string {
	return "MachineClass"
}

// MachineClass yields a newly initialized machine class object.
func (w *workerDelegate) MachineClass() client.Object {
	return &machinev1alpha1.MachineClass{}
}

// MachineClassList yields a newly initialized MachineClassList object.
func (w *workerDelegate) MachineClassList() client.ObjectList {
	return &machinev1alpha1.MachineClassList{}
}

// DeployMachineClasses generates and creates the GCP specific machine classes.
func (w *workerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}

	return w.seedChartApplier.ApplyFromEmbeddedFS(ctx, charts.InternalChart, filepath.Join("internal", "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
}

// GenerateMachineDeployments generates the configuration for the desired machine deployments.
func (w *workerDelegate) GenerateMachineDeployments(ctx context.Context) (worker.MachineDeployments, error) {
	if w.machineDeployments == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return nil, err
		}
	}
	return w.machineDeployments, nil
}

func (w *workerDelegate) generateMachineConfig(_ context.Context) error {
	var (
		machineDeployments = worker.MachineDeployments{}
		machineClasses     []map[string]interface{}
		machineImages      []apisgcp.MachineImage
	)

	infrastructureStatus := &apisgcp.InfrastructureStatus{}
	if _, _, err := w.Decoder().Decode(w.worker.Spec.InfrastructureProviderStatus.Raw, nil, infrastructureStatus); err != nil {
		return err
	}

	nodesSubnet, err := gcpapihelper.FindSubnetByPurpose(infrastructureStatus.Networks.Subnets, apisgcp.PurposeNodes)
	if err != nil {
		return err
	}

	for _, pool := range w.worker.Spec.Pools {
		zoneLen := int32(len(pool.Zones))

		workerPoolHash, err := worker.WorkerPoolHash(pool, w.cluster)
		if err != nil {
			return err
		}

		arch := pointer.StringDeref(pool.Architecture, v1beta1constants.ArchitectureAMD64)

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

		workerConfig := &apisgcp.WorkerConfig{}
		if pool.ProviderConfig != nil && pool.ProviderConfig.Raw != nil {
			if _, _, err := w.Decoder().Decode(pool.ProviderConfig.Raw, nil, workerConfig); err != nil {
				return fmt.Errorf("could not decode provider config: %+v", err)
			}
		}

		disks := make([]map[string]interface{}, 0)
		// root volume
		if pool.Volume != nil {
			disk, err := createDiskSpecForVolume(*pool.Volume, w.worker.Name, machineImage, true)
			if err != nil {
				return err
			}
			if workerConfig.Volume != nil {
				addDiskEncryptionDetails(disk, workerConfig.Volume.Encryption)
			}
			disks = append(disks, disk)
		}

		// additional volumes
		for _, volume := range pool.DataVolumes {
			disk, err := createDiskSpecForDataVolume(volume, w.worker.Name, false)
			if err != nil {
				return err
			}

			if volume.Type != nil && *volume.Type == "SCRATCH" && workerConfig.Volume != nil && workerConfig.Volume.LocalSSDInterface != nil {
				disk["interface"] = *workerConfig.Volume.LocalSSDInterface
			}

			if workerConfig.Volume != nil && volume.Type != nil && *volume.Type != "SCRATCH" {
				// Only add encryption details for non-scratch disks.
				// See https://cloud.google.com/compute/docs/disks/customer-supplied-encryption#technical_restrictions
				addDiskEncryptionDetails(disk, workerConfig.Volume.Encryption)
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

		gceInstanceLabels := getGceInstanceLabels(w.worker.Name, pool)
		isLiveMigrationAllowed := true

		for zoneIndex, zone := range pool.Zones {
			zoneIdx := int32(zoneIndex)
			machineClassSpec := map[string]interface{}{
				"region":             w.worker.Spec.Region,
				"zone":               zone,
				"canIpForward":       true,
				"deletionProtection": false,
				"description":        fmt.Sprintf("Machine of Shoot %s created by machine-controller-manager.", w.worker.Name),
				"disks":              disks,
				"labels":             gceInstanceLabels,
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
					"cloudConfig": string(pool.UserData),
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

			var (
				deploymentName = fmt.Sprintf("%s-%s-z%d", w.worker.Namespace, pool.Name, zoneIndex+1)
				className      = fmt.Sprintf("%s-%s", deploymentName, workerPoolHash)
				gpuCount       int32
			)

			machineDeployments = append(machineDeployments, worker.MachineDeployment{
				Name:                 deploymentName,
				ClassName:            className,
				SecretName:           className,
				Minimum:              worker.DistributeOverZones(zoneIdx, pool.Minimum, zoneLen),
				Maximum:              worker.DistributeOverZones(zoneIdx, pool.Maximum, zoneLen),
				MaxSurge:             worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxSurge, zoneLen, pool.Maximum),
				MaxUnavailable:       worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxUnavailable, zoneLen, pool.Minimum),
				Labels:               addTopologyLabel(pool.Labels, zone),
				Annotations:          pool.Annotations,
				Taints:               pool.Taints,
				MachineConfiguration: genericworkeractuator.ReadMachineConfiguration(pool),
			})

			machineClassSpec["name"] = className
			machineClassSpec["resourceLabels"] = map[string]string{
				v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
			}

			if workerConfig.GPU != nil {
				machineClassSpec["gpu"] = map[string]interface{}{
					"acceleratorType": workerConfig.GPU.AcceleratorType,
					"count":           workerConfig.GPU.Count,
				}
				// using this gpu count for scale-from-zero cases
				gpuCount = workerConfig.GPU.Count
				isLiveMigrationAllowed = false
			}

			if pool.NodeTemplate != nil {
				machineClassSpec["nodeTemplate"] = machinev1alpha1.NodeTemplate{
					Capacity:     initializeCapacity(pool.NodeTemplate.Capacity, gpuCount),
					InstanceType: pool.MachineType,
					Region:       w.worker.Spec.Region,
					Zone:         zone,
				}

				numGpus := pool.NodeTemplate.Capacity[ResourceGPU]
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

func createDiskSpecForVolume(volume v1alpha1.Volume, workerName string, machineImage string, boot bool) (map[string]interface{}, error) {
	return createDiskSpec(volume.Size, workerName, boot, &machineImage, volume.Type)
}

func createDiskSpecForDataVolume(volume v1alpha1.DataVolume, workerName string, boot bool) (map[string]interface{}, error) {
	// Don't set machine image for data volumes. Any pre-existing data on the disk can interfere with the boot disk.
	// See https://github.com/gardener/gardener-extension-provider-gcp/issues/323
	return createDiskSpec(volume.Size, workerName, boot, nil, volume.Type)
}

func createDiskSpec(size, workerName string, boot bool, machineImage, volumeType *string) (map[string]interface{}, error) {
	volumeSize, err := worker.DiskSize(size)
	if err != nil {
		return nil, err
	}

	disk := map[string]interface{}{
		"autoDelete": true,
		"boot":       boot,
		"sizeGb":     volumeSize,
		"labels": map[string]interface{}{
			"name": workerName,
		},
	}

	if machineImage != nil {
		disk["image"] = *machineImage
	}

	if volumeType != nil {
		disk["type"] = *volumeType
	}

	return disk, nil
}

func addDiskEncryptionDetails(disk map[string]interface{}, encryption *apisgcp.DiskEncryption) {
	if encryption == nil {
		return
	}
	var encryptionMap = make(map[string]interface{})
	if encryption.KmsKeyName != nil {
		encryptionMap["kmsKeyName"] = *encryption.KmsKeyName
	}
	if encryption.KmsKeyServiceAccount != nil {
		encryptionMap["kmsKeyServiceAccount"] = *encryption.KmsKeyServiceAccount
	}
	disk["encryption"] = encryptionMap
}

func getGceInstanceLabels(name string, pool v1alpha1.WorkerPool) map[string]interface{} {
	gceInstanceLabels := map[string]interface{}{
		"name": SanitizeGcpLabelValue(name),
	}
	for k, v := range pool.Labels {
		if label := SanitizeGcpLabel(k); label != "" {
			gceInstanceLabels[label] = SanitizeGcpLabelValue(v)
		}
	}
	return gceInstanceLabels
}

func initializeCapacity(capacityList v1.ResourceList, gpuCount int32) v1.ResourceList {
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
