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
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

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
	}
	return allErrs
}
