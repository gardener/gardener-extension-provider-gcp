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

package validation_test

import (
	"fmt"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

func copyWorkers(workers []core.Worker) []core.Worker {
	copy := append(workers[:0:0], workers...)
	for i := range copy {
		copy[i].Zones = append(workers[i].Zones[:0:0], workers[i].Zones...)
	}
	return copy
}

func makeStringPointer(s string) *string {
	ptr := s
	return &ptr
}

var _ = Describe("Shoot validation", func() {
	Describe("#ValidateNetworking", func() {
		var networkingPath = field.NewPath("spec", "networking")

		It("should return no error because nodes CIDR was provided", func() {
			networking := core.Networking{
				Nodes: makeStringPointer("1.2.3.4/5"),
			}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should return an error because no nodes CIDR was provided", func() {
			networking := core.Networking{}

			errorList := ValidateNetworking(networking, networkingPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.nodes"),
				})),
			))
		})
	})

	Describe("#ValidateWorkersUpdate", func() {
		var workerPath = field.NewPath("spec", "workers")

		It("should not return any error because new worker pool added honors having min workers >= number of zones", func() {
			newWorkers := []core.Worker{
				{
					Name: "worker-test",
					Machine: core.Machine{
						Type: "n1-standard-2",
						Image: &core.ShootMachineImage{
							Name:    "gardenlinux",
							Version: "318.8.0",
						},
					},
					Maximum: 6,
					Minimum: 3,
					Zones: []string{
						"europe-west1-b",
						"europe-west1-c",
						"europe-west1-d",
					},
					Volume: &core.Volume{
						Type:       pointer.StringPtr("pd-standard"),
						VolumeSize: "50Gi",
					},
				},
			}

			oldWorkers := []core.Worker{
				{
					Name: "worker-test",
					Machine: core.Machine{
						Type: "n1-standard-2",
						Image: &core.ShootMachineImage{
							Name:    "gardenlinux",
							Version: "318.8.0",
						},
					},
					Maximum: 6,
					Minimum: 1,
					Zones: []string{
						"europe-west1-b",
					},
					Volume: &core.Volume{
						Type:       pointer.StringPtr("pd-standard"),
						VolumeSize: "50Gi",
					},
				},
			}

			errorList := ValidateWorkersUpdate(oldWorkers, newWorkers, workerPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should return an error because new worker pool added does not honor having min workers >= number of zones", func() {
			newWorkers := []core.Worker{
				{
					Name: "worker-test",
					Machine: core.Machine{
						Type: "n1-standard-2",
						Image: &core.ShootMachineImage{
							Name:    "gardenlinux",
							Version: "318.8.0",
						},
					},
					Maximum: 6,
					Minimum: 2,
					Zones: []string{
						"europe-west1-b",
						"europe-west1-c",
						"europe-west1-d",
					},
					Volume: &core.Volume{
						Type:       pointer.StringPtr("pd-standard"),
						VolumeSize: "50Gi",
					},
				},
			}

			oldWorkers := []core.Worker{
				{
					Name: "worker-test",
					Machine: core.Machine{
						Type: "n1-standard-2",
						Image: &core.ShootMachineImage{
							Name:    "gardenlinux",
							Version: "318.8.0",
						},
					},
					Maximum: 6,
					Minimum: 1,
					Zones: []string{
						"europe-west1-b",
					},
					Volume: &core.Volume{
						Type:       pointer.StringPtr("pd-standard"),
						VolumeSize: "50Gi",
					},
				},
			}

			errorList := ValidateWorkersUpdate(oldWorkers, newWorkers, workerPath)

			Expect(errorList[0].Error()).To(Equal("spec.workers[0].minimum: Forbidden: minimum value must be >= " + fmt.Sprint(len(newWorkers[0].Zones)) + " if maximum value > 0 (auto scaling to 0 & from 0 is not supported"))
		})
	})
})

func validateWorkerConfig(workers []core.Worker, workerConfig *gcp.WorkerConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, worker := range workers {
		for _, volume := range worker.DataVolumes {
			allErrs = append(allErrs, ValidateWorkerConfig(workerConfig, volume.Type)...)

		}
	}

	return allErrs
}
