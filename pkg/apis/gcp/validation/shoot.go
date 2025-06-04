// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"

	"github.com/Masterminds/semver/v3"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
)

const (
	overlayKey = "overlay"
	enabledKey = "enabled"
)

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking *core.Networking, fldPath *field.Path, k8sVersion *semver.Version) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for GCP shoots"))
	}

	if core.IsIPv6SingleStack(networking.IPFamilies) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ipFamilies"), networking.IPFamilies, "IPv6 single-stack networking is not supported"))
	}

	if len(networking.IPFamilies) > 1 && versionutils.ConstraintK8sLess131.Check(k8sVersion) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ipFamilies"), networking.IPFamilies, "dual-stack is not supported for Kubernetes versions < 1.31"))
	}

	if networking.IPFamilies != nil && slices.Contains(networking.IPFamilies, core.IPFamilyIPv6) {
		networkConfig, err := decodeNetworkConfig(networking.ProviderConfig)
		if err != nil {
			return append(allErrs, field.Invalid(fldPath.Child("providerConfig"), networking.ProviderConfig, fmt.Sprintf("failed to decode networking provider config: %v", err)))
		}

		if _, ok := networkConfig[overlayKey]; ok {
			if overlay, ok := networkConfig[overlayKey].(map[string]interface{}); ok {
				if enabled, ok := overlay[enabledKey].(bool); ok && enabled {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("providerConfig").Child(overlayKey).Child(enabledKey), enabled, "overlay is not supported in conjunction with IPv6"))
				}
			}
		}
	}

	return allErrs
}

// ValidateWorkers validates the workers of a Shoot.
func ValidateWorkers(workers []core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, worker := range workers {
		workerFldPath := fldPath.Index(i)

		if worker.Volume == nil {
			allErrs = append(allErrs, field.Required(workerFldPath.Child("volume"), "must not be nil"))
		} else {
			allErrs = append(allErrs, validateVolume(worker.Volume, workerFldPath.Child("volume"))...)
		}

		if len(worker.Zones) == 0 {
			allErrs = append(allErrs, field.Required(workerFldPath.Child("zones"), "at least one zone must be configured"))
			continue
		}

		if len(worker.Zones) > math.MaxInt32 {
			allErrs = append(allErrs, field.Invalid(workerFldPath.Child("zones"), len(worker.Zones), "too many zones"))
		}
	}

	return allErrs
}

func validateVolume(vol *core.Volume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if vol.Type == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must not be empty"))
	}
	if vol.Type != nil && *vol.Type == worker.VolumeTypeScratch {
		allErrs = append(allErrs, field.Invalid(
			fldPath.Child("type"), worker.VolumeTypeScratch, fmt.Sprintf("type %s is not allowed as boot disk", worker.VolumeTypeScratch),
		))
	}
	if vol.VolumeSize == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("size"), "must not be empty"))
	}
	return allErrs
}

// ValidateWorkersUpdate validates updates on Workers.
func ValidateWorkersUpdate(oldWorkers, newWorkers []core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, newWorker := range newWorkers {
		workerFldPath := fldPath.Index(i)
		oldWorker := helper.FindWorkerByName(oldWorkers, newWorker.Name)

		if oldWorker != nil && validationutils.ShouldEnforceImmutability(newWorker.Zones, oldWorker.Zones) {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newWorker.Zones, oldWorker.Zones, workerFldPath.Child("zones"))...)
		}

		// Changing these fields will cause a change in the calculation of the WorkerPool hash. Currently machine-controller-manager does not provide an UpdateMachine call to update the
		// data volumes or provider config in-place. gardener-node-agent cannot update the provider config in-place either. So we disallow changing these fields if the update strategy is in-place.
		if helper.IsUpdateStrategyInPlace(newWorker.UpdateStrategy) {
			if !apiequality.Semantic.DeepEqual(newWorker.ProviderConfig, oldWorker.ProviderConfig) {
				allErrs = append(allErrs, field.Invalid(workerFldPath.Child("providerConfig"), newWorker.ProviderConfig, "providerConfig is immutable when update strategy is in-place"))
			}

			if !apiequality.Semantic.DeepEqual(newWorker.DataVolumes, oldWorker.DataVolumes) {
				allErrs = append(allErrs, field.Invalid(workerFldPath.Child("dataVolumes"), newWorker.DataVolumes, "dataVolumes are immutable when update strategy is in-place"))
			}
		}
	}
	return allErrs
}

func decodeNetworkConfig(network *runtime.RawExtension) (map[string]interface{}, error) {
	var networkConfig map[string]interface{}
	if network == nil || network.Raw == nil {
		return map[string]interface{}{}, nil
	}
	if err := json.Unmarshal(network.Raw, &networkConfig); err != nil {
		return nil, err
	}
	return networkConfig, nil
}
