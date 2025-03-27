// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"math"

	"github.com/Masterminds/semver/v3"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
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
	}
	return allErrs
}

// ValidateNetworkingConfigUpdate validates updates on Networking.
func ValidateNetworkingConfigUpdate(oldNetworkConfig, newNetworkConfig *core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateIPFamiliesUpdate(oldNetworkConfig.IPFamilies, newNetworkConfig.IPFamilies, fldPath.Child("ipFamilies"))...)
	return allErrs
}

// ValidateIPFamiliesUpdate validates updates on IPFamilies.
func ValidateIPFamiliesUpdate(oldIPFamilies, newIPFamilies []core.IPFamily, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if (len(oldIPFamilies) == 0 && len(newIPFamilies) == 0) ||
		(len(oldIPFamilies) == 0 && len(newIPFamilies) == 1 && newIPFamilies[0] == core.IPFamilyIPv4) ||
		(len(oldIPFamilies) == 1 && len(newIPFamilies) == 1 && oldIPFamilies[0] == newIPFamilies[0]) ||
		(len(oldIPFamilies) == 2 && len(newIPFamilies) == 2) ||
		(len(oldIPFamilies) == 1 && oldIPFamilies[0] == core.IPFamilyIPv4 && len(newIPFamilies) == 2 && newIPFamilies[0] == core.IPFamilyIPv4 && newIPFamilies[1] == core.IPFamilyIPv6) {
		return allErrs
	}

	allErrs = append(allErrs, field.Invalid(fldPath, newIPFamilies, "unsupported IPFamilies update"))
	return allErrs
}
