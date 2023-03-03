// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

var _ = Describe("InfrastructureConfig validation", func() {
	var (
		infrastructureConfig *apisgcp.InfrastructureConfig

		pods        = "100.96.0.0/11"
		services    = "100.64.0.0/13"
		nodes       = "10.250.0.0/16"
		internal    = "10.10.0.0/24"
		invalidCIDR = "invalid-cidr"

		fldPath *field.Path
	)

	BeforeEach(func() {
		infrastructureConfig = &apisgcp.InfrastructureConfig{
			Networks: apisgcp.NetworkConfig{
				VPC: &apisgcp.VPC{
					Name: "hugo",
					CloudRouter: &apisgcp.CloudRouter{
						Name: "hugo-cr",
					},
				},
				CloudNAT: &apisgcp.CloudNAT{
					MinPortsPerVM: pointer.Int32(20),
					NatIPNames: []apisgcp.NatIPName{{
						Name: "test",
					}},
				},
				FlowLogs: &apisgcp.FlowLogs{
					AggregationInterval: pointer.String("INTERVAL_5_SEC"),
					Metadata:            pointer.String("INCLUDE_ALL_METADATA"),
					FlowSampling:        pointer.Float32(0.4),
				},
				Internal: &internal,
				Workers:  "10.250.0.0/16",
				Worker:   "10.250.0.0/16",
			},
		}
	})

	Describe("#ValidateInfrastructureConfig", func() {
		Context("CIDR", func() {
			It("should forbid invalid worker CIDRs", func() {
				infrastructureConfig.Networks.Workers = invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid invalid internal CIDR", func() {
				invalidCIDR = "invalid-cidr"
				infrastructureConfig.Networks.Internal = &invalidCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.internal"),
					"Detail": Equal("invalid CIDR address: invalid-cidr"),
				}))
			})

			It("should forbid workers CIDR which are not in Nodes CIDR", func() {
				infrastructureConfig.Networks.Workers = "1.1.1.1/32"

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal(`must be a subset of "<nil>" ("10.250.0.0/16")`),
				}))
			})

			It("should forbid Internal CIDR to overlap with Node - and Worker CIDR", func() {
				overlappingCIDR := "10.250.1.0/30"
				infrastructureConfig.Networks.Internal = &overlappingCIDR
				infrastructureConfig.Networks.Workers = overlappingCIDR

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &overlappingCIDR, &pods, &services, fldPath)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.internal"),
					"Detail": Equal(`must not overlap with "<nil>" ("10.250.1.0/30")`),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.internal"),
					"Detail": Equal(`must not overlap with "networks.workers" ("10.250.1.0/30")`),
				}))
			})

			It("should forbid non canonical CIDRs", func() {
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"
				internal := "10.10.0.4/24"
				infrastructureConfig.Networks.Internal = &internal
				infrastructureConfig.Networks.Workers = "10.250.3.8/24"

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodeCIDR, &podCIDR, &serviceCIDR, fldPath)

				Expect(errorList).To(HaveLen(2))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.internal"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.workers"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should allow specifying valid config", func() {
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(BeEmpty())
			})

			It("should allow specifying valid config with podsCIDR=nil and servicesCIDR=nil", func() {
				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, nil, nil, fldPath)
				Expect(errorList).To(BeEmpty())
			})
		})
		Context("VPC", func() {
			var testInfrastructureConfig = &apisgcp.InfrastructureConfig{
				Networks: apisgcp.NetworkConfig{
					Internal: &internal,
					Workers:  "10.250.0.0/16",
				},
			}
			It("should forbid configuring CloudRouter if VPC name is not set", func() {
				testInfrastructureConfig.Networks.VPC = &apisgcp.VPC{}
				testInfrastructureConfig.Networks.VPC.CloudRouter = &apisgcp.CloudRouter{}

				errorList := ValidateInfrastructureConfig(testInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vpc.cloudRouter"),
					"Detail": Equal("cloud router can not be configured when the VPC name is not specified"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vpc.name"),
					"Detail": Equal("vpc name must not be empty when vpc key is provided"),
				}))
			})
			It("should forbid empty VPC flow log config", func() {
				infrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{}

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("networks.flowLogs"),
					"Detail": Equal("at least one VPC flow log parameter must be specified when VPC flow log section is provided"),
				}))
			})
			It("should forbid wrong VPC flow log config", func() {
				aggregationInterval := "foo"
				flowSampling := float32(1.2)
				metadata := "foo"
				infrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{AggregationInterval: &aggregationInterval, FlowSampling: &flowSampling, Metadata: &metadata}

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("networks.flowLogs.aggregationInterval"),
					"Detail": Equal("supported values: \"INTERVAL_5_SEC\", \"INTERVAL_30_SEC\", \"INTERVAL_1_MIN\", \"INTERVAL_5_MIN\", \"INTERVAL_15_MIN\""),
				}, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("networks.flowLogs.metadata"),
					"Detail": Equal("supported values: \"INCLUDE_ALL_METADATA\""),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.flowLogs.flowSampling"),
					"Detail": Equal("must contain a valid value"),
				}))
			})
			It("should forbid reusing a VPC without specifying a CloudRouter", func() {
				testInfrastructureConfig.Networks.VPC = &apisgcp.VPC{
					Name: "test-vpc",
				}

				errorList := ValidateInfrastructureConfig(testInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vpc.cloudRouter"),
					"Detail": Equal("cloud router must be defined when reusing a VPC"),
				}))
			})
			It("should forbid reusing a VPC without specifying a CloudRouter name", func() {
				testInfrastructureConfig.Networks.VPC = &apisgcp.VPC{
					Name:        "test-vpc",
					CloudRouter: &apisgcp.CloudRouter{},
				}

				errorList := ValidateInfrastructureConfig(testInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vpc.cloudRouter.name"),
					"Detail": Equal("cloud router name must be specified when reusing a VPC"),
				}))
			})
			It("should forbid reusing a VPC without specifying a CloudRouter", func() {
				testInfrastructureConfig.Networks.VPC = &apisgcp.VPC{
					Name: "test-vpc",
				}

				errorList := ValidateInfrastructureConfig(testInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vpc.cloudRouter"),
					"Detail": Equal("cloud router must be defined when reusing a VPC"),
				}))
			})
			It("should forbid reusing a VPC without specifying a CloudRouter name", func() {
				testInfrastructureConfig.Networks.VPC = &apisgcp.VPC{
					Name:        "test-vpc",
					CloudRouter: &apisgcp.CloudRouter{},
				}

				errorList := ValidateInfrastructureConfig(testInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.vpc.cloudRouter.name"),
					"Detail": Equal("cloud router name must be specified when reusing a VPC"),
				}))
			})

			It("should forbid empty VPC flow log config", func() {
				infrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{}

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("networks.flowLogs"),
					"Detail": Equal("at least one VPC flow log parameter must be specified when VPC flow log section is provided"),
				}))
			})
			It("should forbid wrong VPC flow log config", func() {
				aggregationInterval := "foo"
				flowSampling := float32(1.2)
				metadata := "foo"
				infrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{AggregationInterval: &aggregationInterval, FlowSampling: &flowSampling, Metadata: &metadata}

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("networks.flowLogs.aggregationInterval"),
					"Detail": Equal("supported values: \"INTERVAL_5_SEC\", \"INTERVAL_30_SEC\", \"INTERVAL_1_MIN\", \"INTERVAL_5_MIN\", \"INTERVAL_15_MIN\""),
				}, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("networks.flowLogs.metadata"),
					"Detail": Equal("supported values: \"INCLUDE_ALL_METADATA\""),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.flowLogs.flowSampling"),
					"Detail": Equal("must contain a valid value"),
				}))
			})
			It("should allow correct VPC flow log config", func() {
				aggregationInterval := "INTERVAL_1_MIN"
				flowSampling := float32(0.5)
				metadata := "INCLUDE_ALL_METADATA"
				infrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{AggregationInterval: &aggregationInterval, FlowSampling: &flowSampling, Metadata: &metadata}

				errorList := ValidateInfrastructureConfig(infrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(BeEmpty())
			})
		})
		Context("CloudNAT and Flowlogs", func() {
			It("should allow correct flowlogs config", func() {
				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{
					AggregationInterval: pointer.String("INTERVAL_1_MIN"),
					Metadata:            pointer.String("INCLUDE_ALL_METADATA"),
					FlowSampling:        pointer.Float32(0.5),
				}

				errorList := ValidateInfrastructureConfig(newInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(BeEmpty())
			})
			It("should allow CloudNAT config with valid NatIPNames", func() {
				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.CloudNAT = &apisgcp.CloudNAT{
					NatIPNames: []apisgcp.NatIPName{
						{Name: "test"},
					},
				}

				errorList := ValidateInfrastructureConfig(newInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(BeEmpty())
			})
			It("should allow CloudNAT config without NatIPNames present", func() {
				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.CloudNAT = &apisgcp.CloudNAT{}

				errorList := ValidateInfrastructureConfig(newInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(BeEmpty())
			})
			It("should forbid empty array for NAT IP names when CloudNAT is present", func() {
				newInfrastructureConfig := infrastructureConfig.DeepCopy()
				newInfrastructureConfig.Networks.CloudNAT = &apisgcp.CloudNAT{
					NatIPNames: []apisgcp.NatIPName{},
				}

				errorList := ValidateInfrastructureConfig(newInfrastructureConfig, &nodes, &pods, &services, fldPath)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("networks.cloudNAT.natIPNames"),
					"Detail": Equal("nat IP names cannot be empty"),
				}))
			})
		})
	})

	Describe("#ValidateInfrastructureConfigUpdate", func() {
		It("should return no errors for an unchanged config", func() {
			Expect(ValidateInfrastructureConfigUpdate(infrastructureConfig, infrastructureConfig, fldPath)).To(BeEmpty())
		})

		It("should allow changing cloudNAT AND Flow-logs details", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.CloudNAT = &apisgcp.CloudNAT{
				MinPortsPerVM: pointer.Int32(30),
				NatIPNames: []apisgcp.NatIPName{{
					Name: "not-test",
				}},
			}
			newInfrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{
				AggregationInterval: pointer.String("INTERVAL_30_SEC"),
			}

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid changing infrastructure network details", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.VPC = &apisgcp.VPC{
				Name: "not-hugo",
				CloudRouter: &apisgcp.CloudRouter{
					Name: "not-hugo-cr",
				},
			}
			newInfrastructureConfig.Networks.Workers = "10.96.0.0/16"
			newInfrastructureConfig.Networks.Worker = "10.96.0.0/16"
			newInfrastructureConfig.Networks.Internal = pointer.String("10.96.0.0/16")

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, fldPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.vpc.name"),
			}, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.vpc.cloudRouter"),
			}, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.internal"),
			}, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.workers"),
			}))
		})

		It("should forbid updating VPC value to nil", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.VPC = nil

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, fldPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.vpc"),
			}))
		})

		It("should allow expanding the worker subnet", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.Workers = "10.250.0.0/15"

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid shrinking the worker subnet", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.Workers = "10.250.0.0/17"

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, fldPath)
			Expect(errorList).To(ConsistOfFields(Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("networks.workers"),
			}))
		})
	})
})
