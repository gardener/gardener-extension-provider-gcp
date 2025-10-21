// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/worker"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("#ValidateWorkers", func() {
	var (
		nilPath *field.Path
		workers []core.Worker
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
		errorList := ValidateWorkers(workers, nilPath)
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

	It("should forbid because volume type SCRATCH must not be main volume", func() {
		workers[0].Volume.Type = ptr.To(worker.VolumeTypeScratch)
		errorList := ValidateWorkers(workers, field.NewPath("workers"))

		Expect(errorList).To(ConsistOf(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("workers[0].volume.type"),
				"Detail": Equal(fmt.Sprintf("type %s is not allowed as boot disk", worker.VolumeTypeScratch)),
			})),
		))
	})

	Describe("#ValidateWorkersUpdate", func() {
		It("should pass because workers are unchanged", func() {
			newWorkers := copyWorkers(workers)
			errorList := ValidateWorkersUpdate(workers, newWorkers, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow adding workers", func() {
			newWorkers := copyWorkers(workers)
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
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[1].zones"),
					"Detail": Equal("field is immutable"),
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
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[0].zones"),
					"Detail": Equal("field is immutable"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[1].zones"),
					"Detail": Equal("field is immutable"),
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
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("[1].zones"),
					"Detail": Equal("field is immutable"),
				})),
			))
		})
	})
})

var _ = Describe("#ValidateWorkerConfig", func() {
	var (
		nilPath      *field.Path
		workerConfig apisgcp.WorkerConfig
	)

	BeforeEach(func() {
		workerConfig = apisgcp.WorkerConfig{
			GPU: &apisgcp.GPU{
				AcceleratorType: "nvidia-tesla-p100",
				Count:           1,
			},
			MinCpuPlatform: ptr.To("Intel Haswell"),
			Volume:         &apisgcp.Volume{},
			ServiceAccount: &apisgcp.ServiceAccount{
				Email:  "user@projectid.iam.gserviceaccount.com",
				Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			},
		}
	})

	It("should forbid because data volume type is nil", func() {
		dataVolumes := []core.DataVolume{{Type: nil}}
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{}, dataVolumes, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeRequired),
			"Field": Equal("dataVolumes[0].type"),
		}))))
	})

	It("should allow valid source image", func() {
		workerConfig.DataVolumes = []apisgcp.DataVolume{{
			Name:        "foo",
			SourceImage: ptr.To("/debian-cloud/global/images/family/debian-9"),
		}}
		errorList := ValidateWorkerConfig(workerConfig, []core.DataVolume{{
			Name:       "foo",
			Type:       ptr.To("Volume"),
			VolumeSize: "30G",
		}}, nil, nilPath)

		Expect(errorList).To(BeEmpty())
	})

	It("should forbid invalid data volume source image", func() {
		workerConfig.DataVolumes = []apisgcp.DataVolume{{
			Name:        "foo",
			SourceImage: ptr.To("projects/my-project/NO_UPPER_CASE"),
		}}
		errorList := ValidateWorkerConfig(workerConfig, []core.DataVolume{{
			Name:       "foo",
			Type:       ptr.To("Volume"),
			VolumeSize: "30G",
		}}, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("dataVolumes[0].sourceImage"),
			"Detail": Equal(fmt.Sprintf("does not match expected regex %s", VolumeSourceImageRegex)),
		}))))
	})

	It("should forbid because service account's email is empty", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			ServiceAccount: &apisgcp.ServiceAccount{
				Email:  "",
				Scopes: []string{"https://www.googleapis.com/auth"},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("serviceAccount.email"),
			"Detail": Equal("must not be fewer than 1 characters, got 0"),
		}))))
	})

	It("should forbid invalid service account email", func() {
		workerConfig.ServiceAccount.Email = "user@projectid.iam.WRONG_SUFFIX.com"
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("serviceAccount.email"),
			"Detail": Equal(fmt.Sprintf("does not match expected regex %s", ServiceAccountRegex)),
		}))))
	})

	It("should forbid invalid service account scope", func() {
		workerConfig.ServiceAccount.Scopes[0] = "https://www.wrong-host.com/auth/cloud-platform"
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("serviceAccount.scopes[0]"),
			"Detail": Equal(fmt.Sprintf("does not match expected regex %s", ServiceAccountScopeRegex)),
		}))))
	})

	It("should pass for valid volume interface", func() {
		workerConfig.Volume.LocalSSDInterface = ptr.To("NVME")
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(BeEmpty())
	})

	It("should forbid invalid volume interface", func() {
		workerConfig.Volume.LocalSSDInterface = ptr.To("not in set")
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeNotSupported),
			"Field": Equal("volume.interface"),
		}))))
	})

	It("should forbid invalid volume encryption key name", func() {
		workerConfig.Volume.Encryption = &apisgcp.DiskEncryption{
			KmsKeyName: ptr.To("projects/my-project/NO_SPECIAL_CHARS#"),
		}
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("volume.encryption.kmsKeyName"),
			"Detail": Equal(fmt.Sprintf("does not match expected regex %s", VolumeKmsKeyNameRegex)),
		}))))
	})

	It("should pass for valid volume encryption", func() {
		workerConfig.Volume.Encryption = &apisgcp.DiskEncryption{
			KmsKeyName:           ptr.To("projects/my-project/locations/global/keyRings/my-keyring"),
			KmsKeyServiceAccount: ptr.To("user@projectid.iam.gserviceaccount.com"),
		}
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(BeEmpty())
	})

	It("should forbid invalid volume encryption service account", func() {
		workerConfig.Volume.Encryption = &apisgcp.DiskEncryption{
			KmsKeyName:           ptr.To("projects/my-project/locations/global/keyRings/my-keyring"),
			KmsKeyServiceAccount: ptr.To("userMISSING_ATprojectid.iam.gserviceaccount.com"),
		}
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("volume.encryption.kmsKeyServiceAccount"),
			"Detail": Equal(fmt.Sprintf("does not match expected regex %s", ServiceAccountRegex)),
		}))))
	})

	It("should forbid because volume.encryption.kmsKeyName should be specified", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			Volume: &apisgcp.Volume{
				Encryption: &apisgcp.DiskEncryption{
					KmsKeyName: ptr.To("  "),
				},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeRequired),
			"Field": Equal("volume.encryption.kmsKeyName"),
		}))))
	})

	It("should forbid because service account scope is empty", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			ServiceAccount: &apisgcp.ServiceAccount{
				Email:  "user@projectid.iam.gserviceaccount.com",
				Scopes: []string{},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeRequired),
			"Field": Equal("serviceAccount.scopes"),
		}))))
	})

	It("should forbid because service account scopes has an empty scope", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			ServiceAccount: &apisgcp.ServiceAccount{
				Email:  "user@projectid.iam.gserviceaccount.com",
				Scopes: []string{"https://www.googleapis.com/auth/test", ""},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("serviceAccount.scopes[1]"),
			"Detail": Equal("must not be fewer than 1 characters, got 0"),
		}))))
	})

	It("should forbid because service account scopes are duplicated", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			ServiceAccount: &apisgcp.ServiceAccount{
				Email:  "user@projectid.iam.gserviceaccount.com",
				Scopes: []string{"https://www.googleapis.com/auth/test", "https://www.googleapis.com/auth/test"},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeDuplicate),
			"Field": Equal("serviceAccount.scopes[1]"),
		}))))
	})

	It("should allow valid service account", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			ServiceAccount: &apisgcp.ServiceAccount{
				Email:  "user@projectid.iam.gserviceaccount.com",
				Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(BeEmpty())
	})

	It("should forbid because gpu accelerator type is empty", func() {
		workerConfig.GPU.AcceleratorType = ""
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("gpu.acceleratorType"),
			"Detail": Equal("must not be fewer than 1 characters, got 0"),
		}))))
	})

	It("should forbid invalid gpu accelerator type", func() {
		workerConfig.GPU.AcceleratorType = "CAPITAL-NOT-ALLOWED"
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("gpu.acceleratorType"),
			"Detail": Equal(fmt.Sprintf("does not match expected regex %s", GpuAcceleratorTypeRegex)),
		}))))
	})

	It("should forbid because gpu count is zero", func() {
		workerConfig.GPU.Count = 0
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeForbidden),
			"Field": Equal("gpu.count"),
		}))))
	})

	It("should forbid invalid min CPU platform", func() {
		workerConfig.MinCpuPlatform = ptr.To("No Special Chars#")
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("minCpuPlatform"),
			"Detail": Equal(fmt.Sprintf("does not match expected regex %s", MinCPUsPlatformRegex)),
		}))))
	})

	It("should fail because WorkerConfig NodeTemplate is specified with empty capacity", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			NodeTemplate: &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeRequired),
			"Field":  Equal("nodeTemplate.capacity"),
			"Detail": Equal("capacity must not be empty"),
		}))))
	})

	It("should not fail for WorkerConfig NodeTemplate not populated with all fields", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			NodeTemplate: &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					"cpu": resource.MustParse("80m"),
				},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(BeEmpty())
	})

	It("should fail for WorkerConfig NodeTemplate resource value less than zero", func() {
		errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
			NodeTemplate: &extensionsv1alpha1.NodeTemplate{
				Capacity: corev1.ResourceList{
					"cpu": resource.MustParse("-80m"),
				},
			},
		}, nil, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("nodeTemplate.capacity.cpu"),
			"Detail": Equal("cpu value must not be negative"),
		}))))
	})

	It("should allow valid gpu configurations", func() {
		workerConfig.GPU = &apisgcp.GPU{
			AcceleratorType: "nvidia-tesla-p100",
			Count:           1,
		}
		errorList := ValidateWorkerConfig(workerConfig, nil, nil, nilPath)

		Expect(errorList).To(BeEmpty())
	})

	It("should allow valid dataVolume name", func() {
		workerConfig := apisgcp.WorkerConfig{
			DataVolumes: []apisgcp.DataVolume{{
				Name: "foo",
			}},
		}
		dataVolumes := []core.DataVolume{{
			Type: ptr.To("Volume"),
			Name: "foo",
		}}
		errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

		Expect(errorList).To(BeEmpty())
	})

	It("should detect duplicate dataVolume names", func() {
		workerConfig := apisgcp.WorkerConfig{
			DataVolumes: []apisgcp.DataVolume{
				{
					Name: "foo",
				}, {
					Name: "foo",
				},
			},
		}
		dataVolumes := []core.DataVolume{{
			Type: ptr.To("Volume"),
			Name: "foo",
		}}
		errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":  Equal(field.ErrorTypeDuplicate),
			"Field": Equal("dataVolumes[1]"),
		}))))
	})

	It("should forbid invalid dataVolume name", func() {
		workerConfig := apisgcp.WorkerConfig{
			DataVolumes: []apisgcp.DataVolume{{
				Name: "foo-invalid",
			}},
		}
		dataVolumes := []core.DataVolume{{
			Type: ptr.To("Volume"),
			Name: "foo",
		}}
		errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

		Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
			"Type":   Equal(field.ErrorTypeInvalid),
			"Field":  Equal("dataVolumes"),
			"Detail": Equal("could not find dataVolume with name foo-invalid"),
		}))))
	})

	Describe("#Volume type hyper-disk", func() {
		It("should pass because setting ProvisionedIops is allowed for hyperdisk-extreme", func() {
			workerConfig := apisgcp.WorkerConfig{
				DataVolumes: []apisgcp.DataVolume{{
					Name: "foo",
					DiskSettings: apisgcp.DiskSettings{
						ProvisionedIops: ptr.To[int64](3000),
					},
				}},
			}
			dataVolumes := []core.DataVolume{{
				Type: ptr.To("hyperdisk-extreme"),
				Name: "foo",
			}}
			errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail because setting ProvisionedIops is not allowed for hyperdisk-throughput", func() {
			workerConfig := apisgcp.WorkerConfig{
				DataVolumes: []apisgcp.DataVolume{{
					Name: "foo",
					DiskSettings: apisgcp.DiskSettings{
						ProvisionedIops: ptr.To[int64](3000),
					},
				}},
			}
			dataVolumes := []core.DataVolume{{
				Type: ptr.To("hyperdisk-throughput"),
				Name: "foo",
			}}
			errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("dataVolumes[0].provisionedIops"),
			}))))
		})

		It("should pass because setting ProvisionedThroughput is allowed for hyperdisk-throughput", func() {
			workerConfig := apisgcp.WorkerConfig{
				DataVolumes: []apisgcp.DataVolume{{
					Name: "foo",
					DiskSettings: apisgcp.DiskSettings{
						ProvisionedThroughput: ptr.To[int64](150),
					},
				}},
			}
			dataVolumes := []core.DataVolume{{
				Type: ptr.To("hyperdisk-throughput"),
				Name: "foo",
			}}
			errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should fail because setting ProvisionedThroughput is not allowed for hyperdisk-extreme", func() {
			workerConfig := apisgcp.WorkerConfig{
				DataVolumes: []apisgcp.DataVolume{{
					Name: "foo",
					DiskSettings: apisgcp.DiskSettings{
						ProvisionedThroughput: ptr.To[int64](150),
					},
				}},
			}
			dataVolumes := []core.DataVolume{{
				Type: ptr.To("hyperdisk-extreme"),
				Name: "foo",
			}}
			errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("dataVolumes[0].provisionedThroughput"),
			}))))
		})

		Context("BootVolume", func() {
			var bootVolume *core.Volume

			BeforeEach(func() {
				bootVolume = &core.Volume{}
			})

			It("should fail because ProvisionedThroughput is not allowed for hyperdisk-extreme", func() {
				bootVolume.Type = ptr.To(gcp.HyperDiskExtreme)
				errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
					BootVolume: &apisgcp.BootVolume{
						DiskSettings: apisgcp.DiskSettings{
							ProvisionedThroughput: ptr.To[int64](150),
						},
					},
				}, nil, bootVolume, nilPath)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("bootVolume.provisionedThroughput"),
					})),
				))
			})

			It("should pass because ProvisionedThroughput is allowed for hyperdisk-throughput", func() {
				bootVolume.Type = ptr.To(gcp.HyperDiskThroughput)
				errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
					BootVolume: &apisgcp.BootVolume{
						DiskSettings: apisgcp.DiskSettings{
							ProvisionedThroughput: ptr.To[int64](150),
						},
					},
				}, nil, bootVolume, nilPath)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail because provisionedIops is not allowed for hyperdisk-throughput", func() {
				bootVolume.Type = ptr.To(gcp.HyperDiskThroughput)
				errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
					BootVolume: &apisgcp.BootVolume{
						DiskSettings: apisgcp.DiskSettings{
							ProvisionedIops: ptr.To[int64](3000),
						},
					},
				}, nil, bootVolume, nilPath)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("bootVolume.provisionedIops"),
					})),
				))
			})

			It("should pass because provisionedIops is allowed for hyperdisk-balanced", func() {
				bootVolume.Type = ptr.To(gcp.HyperDiskBalanced)
				errorList := ValidateWorkerConfig(apisgcp.WorkerConfig{
					BootVolume: &apisgcp.BootVolume{
						DiskSettings: apisgcp.DiskSettings{
							ProvisionedIops: ptr.To[int64](3000),
						},
					},
				}, nil, bootVolume, nilPath)
				Expect(errorList).To(BeEmpty())
			})
		})
	})

	Describe("#Volume type SCRATCH", func() {
		It("should pass because worker config is configured correctly", func() {
			workerConfig := apisgcp.WorkerConfig{
				Volume: &apisgcp.Volume{
					LocalSSDInterface: ptr.To("NVME"),
				},
				DataVolumes: []apisgcp.DataVolume{{
					Name: "foo",
				}},
			}
			dataVolumes := []core.DataVolume{{
				Type: ptr.To(worker.VolumeTypeScratch),
				Name: "foo",
			}}
			errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid because encryption is not allowed for volume type SCRATCH", func() {
			workerConfig := apisgcp.WorkerConfig{
				Volume: &apisgcp.Volume{
					LocalSSDInterface: ptr.To("NVME"),
					Encryption: &apisgcp.DiskEncryption{
						KmsKeyName: ptr.To("KmsKey"),
					},
				},
				DataVolumes: []apisgcp.DataVolume{{
					Name: "foo",
				}},
			}
			dataVolumes := []core.DataVolume{{
				Type: ptr.To(worker.VolumeTypeScratch),
				Name: "foo",
			}}
			errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("volume.encryption"),
				"Detail": Equal(fmt.Sprintf("must not be set in combination with %s volumes", worker.VolumeTypeScratch)),
			}))))
		})

		It("should forbid because interface of worker config is not configured", func() {
			workerConfig := apisgcp.WorkerConfig{}
			dataVolumes := []core.DataVolume{{
				Type: ptr.To(worker.VolumeTypeScratch),
				Name: "foo",
			}}
			errorList := ValidateWorkerConfig(workerConfig, dataVolumes, nil, nilPath)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("volume.interface"),
			}))))
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
