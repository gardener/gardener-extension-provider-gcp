// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

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
					MinPortsPerVM: ptr.To[int32](20),
					NatIPNames: []apisgcp.NatIPName{{
						Name: "test",
					}},
				},
				FlowLogs: &apisgcp.FlowLogs{
					AggregationInterval: ptr.To("INTERVAL_5_SEC"),
					Metadata:            ptr.To("INCLUDE_ALL_METADATA"),
					FlowSampling:        ptr.To[float64](0.4),
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
					"Detail": Equal(`must be a subset of "networking.nodes" ("10.250.0.0/16")`),
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
					"Detail": Equal(`must not overlap with "networking.nodes" ("10.250.1.0/30")`),
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
				flowSampling := float64(1.2)
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
				flowSampling := float64(1.2)
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
				flowSampling := float64(0.5)
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
					AggregationInterval: ptr.To("INTERVAL_1_MIN"),
					Metadata:            ptr.To("INCLUDE_ALL_METADATA"),
					FlowSampling:        ptr.To[float64](0.5),
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
					"Detail": Equal("nat IP names cannot be empty."),
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
				MinPortsPerVM: ptr.To[int32](30),
				NatIPNames: []apisgcp.NatIPName{{
					Name: "not-test",
				}},
			}
			newInfrastructureConfig.Networks.FlowLogs = &apisgcp.FlowLogs{
				AggregationInterval: ptr.To("INTERVAL_30_SEC"),
			}

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid changing existing infrastructure network details", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			newInfrastructureConfig.Networks.VPC = &apisgcp.VPC{
				Name: "not-hugo",
				CloudRouter: &apisgcp.CloudRouter{
					Name: "not-hugo-cr",
				},
			}
			newInfrastructureConfig.Networks.Workers = "10.96.0.0/16"
			newInfrastructureConfig.Networks.Worker = "10.96.0.0/16"
			newInfrastructureConfig.Networks.Internal = ptr.To("10.96.0.0/16")

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

		It("should allow adding an internal subnet if it was not set before", func() {
			newInfrastructureConfig := infrastructureConfig.DeepCopy()
			// delete the internal subnet to simulate it was not set before
			infrastructureConfig.Networks.Internal = nil
			newInfrastructureConfig.Networks.Internal = ptr.To("10.96.0.0/16")

			errorList := ValidateInfrastructureConfigUpdate(infrastructureConfig, newInfrastructureConfig, fldPath)
			Expect(errorList).To(BeEmpty())
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
