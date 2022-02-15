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

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
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

	Describe("#ValidateWorkerAutoscaling", func() {
		var worker core.Worker

		BeforeEach(func() {
			worker = core.Worker{
				Minimum: 2,
				Maximum: 4,
				Zones:   []string{"zone1", "zone2"},
			}
		})

		It("should not return an error if worker.minimum is equal to number of zones", func() {
			Expect(ValidateWorkerAutoScaling(worker, "")).To(Succeed())
		})

		It("should not return an error if worker.minimum is greater than number of zones", func() {
			worker.Minimum = 3
			Expect(ValidateWorkerAutoScaling(worker, "")).To(Succeed())
		})

		It("should not return an error if worker.maximum is 0 and worker.minimum is less than number of zones", func() {
			worker.Minimum = 0
			worker.Maximum = 0
			Expect(ValidateWorkerAutoScaling(worker, "")).To(Succeed())
		})

		It("should return an error if worker.minimum is less than number of zones", func() {
			worker.Minimum = 1
			Expect(ValidateWorkerAutoScaling(worker, "")).To(HaveOccurred())
		})
	})

	Describe("#ValidateWorkers", func() {
		var workers []core.Worker

		BeforeEach(func() {
			workers = []core.Worker{
				{
					Name: "foo",
					Volume: &core.Volume{
						Type:       pointer.String("some-type"),
						VolumeSize: "40Gi",
					},
					Zones: []string{"zone1"},
				},
				{
					Name: "bar",
					Volume: &core.Volume{
						Type:       pointer.String("some-type"),
						VolumeSize: "40Gi",
					},
					Zones: []string{"zone1"},
				},
			}
		})
		It("should pass when the kubernetes version is equal to the CSI migration version", func() {
			workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.18.0")}

			errorList := ValidateWorkers(workers, field.NewPath(""))

			Expect(errorList).To(BeEmpty())
		})

		It("should pass when the kubernetes version is higher to the CSI migration version", func() {
			workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.19.0")}

			errorList := ValidateWorkers(workers, field.NewPath(""))

			Expect(errorList).To(BeEmpty())
		})

		It("should not allow when the kubernetes version is lower than the CSI migration version", func() {
			workers[0].Kubernetes = &core.WorkerKubernetes{Version: pointer.String("1.17.0")}

			errorList := ValidateWorkers(workers, field.NewPath("workers"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("workers[0].kubernetes.version"),
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
					Name: "worker-test2",
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

		It("should return an error because updated worker pool does not honor having min workers >= number of zones", func() {
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

			Expect(errorList).ToNot(HaveLen(0))
			Expect(errorList[0].Error()).To(Equal("spec.workers[0].minimum: Forbidden: spec.workers[0].minimum value must be >= " + fmt.Sprint(len(newWorkers[0].Zones)) + " (number of zones) if maximum value > 0 (auto scaling to 0 & from 0 is not supported)"))
		})

		It("should return an error because newly added worker pool does not honor having min workers >= number of zones", func() {
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
					Minimum: 1,
					Zones: []string{
						"europe-west1-b",
						"europe-west1-c",
					},
					Volume: &core.Volume{
						Type:       pointer.StringPtr("pd-standard"),
						VolumeSize: "50Gi",
					},
				},
			}

			oldWorkers := []core.Worker{}

			errorList := ValidateWorkersUpdate(oldWorkers, newWorkers, workerPath)

			Expect(errorList).ToNot(HaveLen(0))
			Expect(errorList[0].Error()).To(Equal("spec.workers[0].minimum: Forbidden: spec.workers[0].minimum value must be >= " + fmt.Sprint(len(newWorkers[0].Zones)) + " (number of zones) if maximum value > 0 (auto scaling to 0 & from 0 is not supported)"))
		})

		It("should not return any error because existing worker pool does not honor having min workers >= number of zones", func() {
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

			errorList := ValidateWorkersUpdate(oldWorkers, newWorkers, workerPath)
			Expect(errorList).To(BeEmpty())
		})
	})
})

func validateWorkerConfig(workers []core.Worker, workerConfig *api.WorkerConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, worker := range workers {
		allErrs = append(allErrs, ValidateWorkerConfig(workerConfig, worker.DataVolumes)...)
	}

	return allErrs
}
