// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
)

var _ = Describe("#ValidateWorkers", func() {
	var (
		workers []core.Worker
		nilPath *field.Path
	)

	BeforeEach(func() {
		workers = []core.Worker{
			{
				Volume: &core.Volume{
					Type:       ptr.To("Volume"),
					VolumeSize: "30G",
				},

				Zones: []string{
					"zone1",
					"zone2",
				},
				DataVolumes: []core.DataVolume{
					{
						Name:       "foo",
						Type:       ptr.To("Volume"),
						VolumeSize: "30G",
					},
				},
			},
			{
				Volume: &core.Volume{
					Type:       ptr.To("Volume"),
					VolumeSize: "20G",
				},

				Zones: []string{
					"zone1",
					"zone2",
				},
			},
			{
				Volume: &core.Volume{
					Type:       ptr.To("Volume"),
					VolumeSize: "20G",
				},
				Minimum: 2,
				Zones: []string{
					"zone1",
					"zone2",
				},
			},
		}
	})
	It("should pass because workers are configured correctly", func() {
		errorList := ValidateWorkers(workers, field.NewPath(""))

		Expect(errorList).To(BeEmpty())
	})

	It("should forbid because volume is not configured", func() {
		workers[1].Volume = nil

		errorList := ValidateWorkers(workers, field.NewPath("workers"))

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("workers[1].volume"),
			})),
		))
	})

	It("should forbid because volume type and size are not configured", func() {
		workers[0].Volume.Type = nil
		workers[0].Volume.VolumeSize = ""

		errorList := ValidateWorkers(workers, field.NewPath("workers"))

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("workers[0].volume.type"),
			})),
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("workers[0].volume.size"),
			})),
		))
	})

	It("should forbid because data volume type is empty", func() {
		workers[0].DataVolumes[0].Type = nil
		errorList := validateWorkerConfig(workers, nil)
		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("dataVolumes[0].type"),
			})),
		))
	})

	It("should forbid because worker does not specify a zone", func() {
		workers[0].Zones = nil

		errorList := ValidateWorkers(workers, field.NewPath("workers"))

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("workers[0].zones"),
			})),
		))
	})

	It("should forbid because service account's email is empty", func() {
		errorList := ValidateWorkerConfig(&gcp.WorkerConfig{
			ServiceAccount: &gcp.ServiceAccount{
				Email:  "",
				Scopes: []string{"scope-1"},
			},
		}, nil)

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("providerConfig.serviceAccount.email"),
			})),
		))
	})

	It("should forbid because volume.encryption.kmsKeyName should be specified", func() {
		errorList := ValidateWorkerConfig(&gcp.WorkerConfig{
			Volume: &gcp.Volume{
				Encryption: &gcp.DiskEncryption{
					KmsKeyName: ptr.To("  "),
				},
			},
		}, nil)

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("providerConfig.volume.encryption.kmsKeyName"),
			})),
		))
	})

	It("should forbid because service account scope is empty", func() {
		errorList := ValidateWorkerConfig(&gcp.WorkerConfig{
			ServiceAccount: &gcp.ServiceAccount{
				Email:  "foo",
				Scopes: []string{},
			},
		}, nil)

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("providerConfig.serviceAccount.scopes"),
			})),
		))
	})

	It("should forbid because service account scopes has an empty scope", func() {
		errorList := ValidateWorkerConfig(&gcp.WorkerConfig{
			ServiceAccount: &gcp.ServiceAccount{
				Email:  "foo",
				Scopes: []string{"baz", ""},
			},
		}, nil)

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("providerConfig.serviceAccount.scopes[1]"),
			})),
		))
	})

	It("should forbid because service account scopes are duplicated", func() {
		errorList := ValidateWorkerConfig(&gcp.WorkerConfig{
			ServiceAccount: &gcp.ServiceAccount{
				Email:  "foo",
				Scopes: []string{"baz", "bar", "baz"},
			},
		}, nil)

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("providerConfig.serviceAccount.scopes[2]"),
			})),
		))
	})

	It("should allow valid service account", func() {
		errorList := ValidateWorkerConfig(&gcp.WorkerConfig{
			ServiceAccount: &gcp.ServiceAccount{
				Email:  "foo",
				Scopes: []string{"baz"},
			},
		}, nil)

		Expect(errorList).To(BeEmpty())
	})

	It("should forbid because gpu accelerator type is empty", func() {
		errorList := ValidateWorkerConfig(
			&gcp.WorkerConfig{
				GPU: &gcp.GPU{
					AcceleratorType: "",
					Count:           1},
				ServiceAccount: &gcp.ServiceAccount{
					Email:  "foo",
					Scopes: []string{"baz"},
				},
			},
			nil,
		)

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("providerConfig.gpu.acceleratorType"),
			})),
		))
	})

	It("should forbid because gpu count is zero", func() {
		errorList := ValidateWorkerConfig(
			&gcp.WorkerConfig{
				GPU: &gcp.GPU{
					AcceleratorType: "foo",
					Count:           0},
				ServiceAccount: &gcp.ServiceAccount{
					Email:  "foo",
					Scopes: []string{"baz"},
				},
			},
			nil,
		)

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("providerConfig.gpu.count"),
			})),
		))
	})

	It("should allow valid gpu configurations", func() {
		errorList := ValidateWorkerConfig(
			&gcp.WorkerConfig{
				GPU: &gcp.GPU{
					AcceleratorType: "foo",
					Count:           1},
				ServiceAccount: &gcp.ServiceAccount{
					Email:  "foo",
					Scopes: []string{"baz"},
				},
			},
			nil,
		)

		Expect(errorList).To(BeEmpty())
	})

	It("should allow valid dataVolume name", func() {
		errorList := validateWorkerConfig([]core.Worker{workers[0]}, &gcp.WorkerConfig{
			DataVolumes: []gcp.DataVolume{{
				Name: "foo",
			}},
		})
		Expect(errorList).To(BeEmpty())
	})

	It("should forbid invalid dataVolume name", func() {
		errorList := validateWorkerConfig([]core.Worker{workers[0]}, &gcp.WorkerConfig{
			DataVolumes: []gcp.DataVolume{{
				Name: "foo-invalid",
			}},
		})
		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("providerConfig.dataVolume"),
			})),
		))
	})

	Describe("#Volume type SCRATCH", func() {
		It("should pass because worker config is configured correctly", func() {
			workers[0].DataVolumes[0].Type = ptr.To(worker.VolumeTypeScratch)
			errorList := validateWorkerConfig(workers, &gcp.WorkerConfig{
				Volume: &gcp.Volume{
					LocalSSDInterface: ptr.To("NVME"),
				},
			})
			Expect(errorList).To(BeEmpty())
		})
		It("should forbid because volume type SCRATCH must not be main volume", func() {
			workers[0].Volume.Type = ptr.To(worker.VolumeTypeScratch)

			errorList := ValidateWorkers(workers, field.NewPath("workers"))

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("workers[0].volume.type"),
				})),
			))
		})

		It("should forbid because interface of worker config is misconfiguration", func() {
			workers[0].DataVolumes[0].Type = ptr.To(worker.VolumeTypeScratch)
			errorList := validateWorkerConfig(workers, &gcp.WorkerConfig{
				Volume: &gcp.Volume{
					LocalSSDInterface: ptr.To("Interface"),
				},
			})
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("providerConfig.volume.interface"),
				})),
			))
		})

		It("should forbid because encryption is not allowed for volume type SCRATCH", func() {
			workers[0].DataVolumes[0].Type = ptr.To(worker.VolumeTypeScratch)
			errorList := validateWorkerConfig(workers, &gcp.WorkerConfig{
				Volume: &gcp.Volume{
					LocalSSDInterface: ptr.To("NVME"),
					Encryption: &gcp.DiskEncryption{
						KmsKeyName: ptr.To("KmsKey"),
					},
				},
			})
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("providerConfig.volume.encryption"),
				})),
			))
		})

		It("should forbid because interface of worker config is not configured", func() {
			workers[0].DataVolumes[0].Type = ptr.To(worker.VolumeTypeScratch)
			errorList := validateWorkerConfig(workers, nil)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("providerConfig.volume.interface"),
				})),
			))
		})
	})

	Describe("#ValidateWorkersUpdate", func() {
		It("should pass because workers are unchanged", func() {
			newWorkers := copyWorkers(workers)
			errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow adding workers", func() {
			newWorkers := append(workers[:0:0], workers...)
			workers = workers[:1]
			errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow adding a zone to a worker", func() {
			newWorkers := copyWorkers(workers)
			newWorkers[0].Zones = append(newWorkers[0].Zones, "another-zone")
			errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid removing a zone from a worker", func() {
			newWorkers := copyWorkers(workers)
			newWorkers[1].Zones = newWorkers[1].Zones[1:]
			errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("[1].zones"),
				})),
			))
		})

		It("should forbid changing the zone order", func() {
			newWorkers := copyWorkers(workers)
			newWorkers[0].Zones[0] = workers[0].Zones[1]
			newWorkers[0].Zones[1] = workers[0].Zones[0]
			newWorkers[1].Zones[0] = workers[1].Zones[1]
			newWorkers[1].Zones[1] = workers[1].Zones[0]
			errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("[0].zones"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("[1].zones"),
				})),
			))
		})

		It("should forbid adding a zone while changing an existing one", func() {
			newWorkers := copyWorkers(workers)
			newWorkers = append(newWorkers, core.Worker{Name: "worker3", Zones: []string{"zone1"}})
			newWorkers[1].Zones[0] = workers[1].Zones[1]
			errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("[1].zones"),
				})),
			))
		})
	})
})

func copyWorkers(workers []core.Worker) []core.Worker {
	cp := append(workers[:0:0], workers...)
	for i := range cp {
		cp[i].Zones = append(workers[i].Zones[:0:0], workers[i].Zones...)
	}
	return cp
}

func validateWorkerConfig(workers []core.Worker, workerConfig *gcp.WorkerConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	for _, worker := range workers {
		allErrs = append(allErrs, ValidateWorkerConfig(workerConfig, worker.DataVolumes)...)
	}

	return allErrs
}
