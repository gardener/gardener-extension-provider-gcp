// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation

import (
	"fmt"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"

	"github.com/Masterminds/semver"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	validationutils "github.com/gardener/gardener/pkg/utils/validation"
	"k8s.io/apimachinery/pkg/api/equality"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var k8sV121 *semver.Version

func init() {
	k8sV121 = semver.MustParse("v1.21.0")
}

// ValidateNetworking validates the network settings of a Shoot.
func ValidateNetworking(networking core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking.Nodes == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("nodes"), "a nodes CIDR must be provided for GCP shoots"))
	}

	return allErrs
}

// ValidateWorkers validates the workers of a Shoot.
func ValidateWorkers(workers []core.Worker, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	csiMigrationVersion, err := semver.NewVersion(gcp.CSIMigrationKubernetesVersion)
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	for i, worker := range workers {
		workerFldPath := fldPath.Index(i)

		// Ensure the kubelet version is not lower than the version in which the extension performs CSI migration.
		if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
			versionPath := workerFldPath.Child("kubernetes", "version")

			v, err := semver.NewVersion(*worker.Kubernetes.Version)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(versionPath, *worker.Kubernetes.Version, err.Error()))
				return allErrs
			}

			if v.LessThan(csiMigrationVersion) {
				allErrs = append(allErrs, field.Forbidden(versionPath, fmt.Sprintf("cannot use kubelet version (%s) lower than CSI migration version (%s)", v.String(), csiMigrationVersion.String())))
			}
		}

		if worker.Volume == nil {
			allErrs = append(allErrs, field.Required(workerFldPath.Child("volume"), "must not be nil"))
		} else {
			allErrs = append(allErrs, validateVolume(worker.Volume, workerFldPath.Child("volume"))...)
		}

		if len(worker.Zones) == 0 {
			allErrs = append(allErrs, field.Required(workerFldPath.Child("zones"), "at least one zone must be configured"))
			continue
		}

		if worker.Maximum != 0 && worker.Minimum == 0 {
			allErrs = append(allErrs, field.Forbidden(workerFldPath.Child("minimum"), "minimum value must be > 0 if maximum value > 0 (auto scaling to 0 is not supported)"))
		}
	}

	return allErrs
}

// ValidateWorkerAutoScaling checks if the worker.minimum value is greater or equal to the number of worker.zones[]
// when the worker.maximum value is greater than zero. This check is necessary because autoscaling from 0 is not supported on gcp.
func ValidateWorkerAutoScaling(worker core.Worker, path string) error {
	if worker.Maximum > 0 && worker.Minimum < int32(len(worker.Zones)) {
		return fmt.Errorf("%s value must be >= %d (number of zones) if maximum value > 0 (auto scaling to 0 & from 0 is not supported)", path, len(worker.Zones))
	}
	return nil
}

func validateVolume(vol *core.Volume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if vol.Type == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must not be empty"))
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

		// TODO: This check won't be needed after generic support to scale from zero is introduced in CA
		// Ongoing issue - https://github.com/gardener/autoscaler/issues/27
		if !equality.Semantic.DeepEqual(&newWorker, oldWorker) {
			if err := ValidateWorkerAutoScaling(newWorker, workerFldPath.Child("minimum").String()); err != nil {
				allErrs = append(allErrs, field.Forbidden(workerFldPath.Child("minimum"), err.Error()))
			}
		}
	}
	return allErrs
}

// ValidateUpgradeV120ToV121 prevents that Shoots get updated from k8s v1.20 to v1.21.
// TODO(dkistner) remove this once the kcm detaches all disks during upgrade from v1.20 to v1.21 is resolved.
func ValidateUpgradeV120ToV121(newShoot, oldShoot *core.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	oldShootVersion, err := semver.NewVersion(oldShoot.Spec.Kubernetes.Version)
	if err != nil {
		return append(allErrs, field.Invalid(fldPath, oldShoot.Spec.Kubernetes.Version, "could not parse kubernetes version"))
	}

	newShootVersion, err := semver.NewVersion(newShoot.Spec.Kubernetes.Version)
	if err != nil {
		return append(allErrs, field.Invalid(fldPath, newShoot.Spec.Kubernetes.Version, "could not parse kubernetes version"))
	}

	if oldShootVersion.LessThan(k8sV121) && (newShootVersion.Equal(k8sV121) || newShootVersion.GreaterThan(k8sV121)) {
		if value, ok := newShoot.Annotations[gcp.ForceK8sVersionUpgrade]; ok && value == "true" {
			return allErrs
		}
		allErrs = append(allErrs, field.Forbidden(fldPath, "upgrade Kubernetes version from < v1.21 to >= v1.21.0 are disabled"))
	}

	return allErrs
}
