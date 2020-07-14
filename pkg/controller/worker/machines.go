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

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpapi "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpapihelper "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	genericworkeractuator "github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	computev1 "google.golang.org/api/compute/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	labelRegex = regexp.MustCompile(`[^a-z0-9_-]`)
)

const maxGcpLabelCharactersSize = 63

// MachineClassKind yields the name of the GCP machine class.
func (w *workerDelegate) MachineClassKind() string {
	return "GCPMachineClass"
}

// MachineClassList yields a newly initialized GCPMachineClassList object.
func (w *workerDelegate) MachineClassList() runtime.Object {
	return &machinev1alpha1.GCPMachineClassList{}
}

// DeployMachineClasses generates and creates the GCP specific machine classes.
func (w *workerDelegate) DeployMachineClasses(ctx context.Context) error {
	if w.machineClasses == nil {
		if err := w.generateMachineConfig(ctx); err != nil {
			return err
		}
	}
	return w.seedChartApplier.Apply(ctx, filepath.Join(gcp.InternalChartsPath, "machineclass"), w.worker.Namespace, "machineclass", kubernetes.Values(map[string]interface{}{"machineClasses": w.machineClasses}))
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

func (w *workerDelegate) generateMachineClassSecretData(ctx context.Context) (map[string][]byte, error) {
	serviceAccountJSON, err := gcp.GetServiceAccountData(ctx, w.Client(), w.worker.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	return map[string][]byte{
		machinev1alpha1.GCPServiceAccountJSON: serviceAccountJSON,
	}, nil
}

func (w *workerDelegate) generateMachineConfig(ctx context.Context) error {
	var (
		machineDeployments = worker.MachineDeployments{}
		machineClasses     []map[string]interface{}
		machineImages      []apisgcp.MachineImage
	)

	machineClassSecretData, err := w.generateMachineClassSecretData(ctx)
	if err != nil {
		return err
	}

	infrastructureStatus := &gcpapi.InfrastructureStatus{}
	if _, _, err := w.Decoder().Decode(w.worker.Spec.InfrastructureProviderStatus.Raw, nil, infrastructureStatus); err != nil {
		return err
	}

	nodesSubnet, err := gcpapihelper.FindSubnetByPurpose(infrastructureStatus.Networks.Subnets, gcpapi.PurposeNodes)
	if err != nil {
		return err
	}

	for _, pool := range w.worker.Spec.Pools {
		zoneLen := int32(len(pool.Zones))

		workerPoolHash, err := worker.WorkerPoolHash(pool, w.cluster)
		if err != nil {
			return err
		}

		machineImage, err := w.findMachineImage(pool.MachineImage.Name, pool.MachineImage.Version)
		if err != nil {
			return err
		}
		machineImages = appendMachineImage(machineImages, apisgcp.MachineImage{
			Name:    pool.MachineImage.Name,
			Version: pool.MachineImage.Version,
			Image:   machineImage,
		})

		disks := make([]map[string]interface{}, 0)
		// root volume
		if pool.Volume != nil {
			disk, err := createDiskSpecForVolume(*pool.Volume, w.worker.Name, machineImage, true)
			if err != nil {
				return err
			}

			disks = append(disks, disk)
		}

		// additional volumes
		workerConfig := &gcpapi.WorkerConfig{}
		if pool.ProviderConfig != nil && pool.ProviderConfig.Raw != nil {
			if _, _, err := w.Decoder().Decode(pool.ProviderConfig.Raw, nil, workerConfig); err != nil {
				return fmt.Errorf("could not decode provider config: %+v", err)
			}
		}
		for _, volume := range pool.DataVolumes {
			disk, err := createDiskSpecForDataVolume(volume, w.worker.Name, machineImage, false)
			if err != nil {
				return err
			}

			if volume.Type != nil && *volume.Type == "SCRATCH" && workerConfig.Volume != nil && workerConfig.Volume.LocalSSDInterface != nil {
				disk["interface"] = *workerConfig.Volume.LocalSSDInterface
			}

			disks = append(disks, disk)
		}

		serviceAccounts := make([]map[string]interface{}, 0)

		if workerConfig.ServiceAccount != nil {
			serviceAccounts = append(serviceAccounts, map[string]interface{}{
				"email":  workerConfig.ServiceAccount.Email,
				"scopes": workerConfig.ServiceAccount.Scopes,
			})
		} else {
			serviceAccounts = append(serviceAccounts, map[string]interface{}{
				"email":  infrastructureStatus.ServiceAccountEmail,
				"scopes": []string{computev1.ComputeScope},
			})
		}

		gceInstanceLabels := getGceInstanceLabels(w.worker.Name, pool)
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
				"machineType":        pool.MachineType,
				"networkInterfaces": []map[string]interface{}{
					{
						"subnetwork":        nodesSubnet.Name,
						"disableExternalIP": true,
					},
				},
				"scheduling": map[string]interface{}{
					"automaticRestart":  true,
					"onHostMaintenance": "MIGRATE",
					"preemptible":       false,
				},
				"secret": map[string]interface{}{
					"cloudConfig": string(pool.UserData),
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
			)

			machineDeployments = append(machineDeployments, worker.MachineDeployment{
				Name:           deploymentName,
				ClassName:      className,
				SecretName:     className,
				Minimum:        worker.DistributeOverZones(zoneIdx, pool.Minimum, zoneLen),
				Maximum:        worker.DistributeOverZones(zoneIdx, pool.Maximum, zoneLen),
				MaxSurge:       worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxSurge, zoneLen, pool.Maximum),
				MaxUnavailable: worker.DistributePositiveIntOrPercent(zoneIdx, pool.MaxUnavailable, zoneLen, pool.Minimum),
				Labels:         pool.Labels,
				Annotations:    pool.Annotations,
				Taints:         pool.Taints,
			})

			machineClassSpec["name"] = className
			machineClassSpec["resourceLabels"] = map[string]string{
				v1beta1constants.GardenerPurpose: genericworkeractuator.GardenPurposeMachineClass,
			}
			machineClassSpec["secret"].(map[string]interface{})[gcp.ServiceAccountJSONMCM] = string(machineClassSecretData[machinev1alpha1.GCPServiceAccountJSON])

			machineClasses = append(machineClasses, machineClassSpec)
		}
	}

	w.machineDeployments = machineDeployments
	w.machineClasses = machineClasses
	w.machineImages = machineImages

	return nil
}

func createDiskSpecForVolume(volume v1alpha1.Volume, workerName string, machineImage string, boot bool) (map[string]interface{}, error) {
	return createDiskSpec(volume.Size, workerName, machineImage, boot, volume.Type)
}

func createDiskSpecForDataVolume(volume v1alpha1.DataVolume, workerName string, machineImage string, boot bool) (map[string]interface{}, error) {
	return createDiskSpec(volume.Size, workerName, machineImage, boot, volume.Type)
}

func createDiskSpec(size, workerName, machineImage string, boot bool, volumeType *string) (map[string]interface{}, error) {
	volumeSize, err := worker.DiskSize(size)
	if err != nil {
		return nil, err
	}

	disk := map[string]interface{}{
		"autoDelete": true,
		"boot":       boot,
		"sizeGb":     volumeSize,
		"image":      machineImage,
		"labels": map[string]interface{}{
			"name": workerName,
		},
	}

	if volumeType != nil {
		disk["type"] = *volumeType
	}

	return disk, nil
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
