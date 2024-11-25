// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"fmt"
	"strconv"

	mockterraformer "github.com/gardener/gardener/extensions/pkg/terraformer/mock"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Terraform", func() {
	var (
		infra              *extensionsv1alpha1.Infrastructure
		config             *api.InfrastructureConfig
		projectID          string
		serviceAccountData []byte
		serviceAccount     *gcp.ServiceAccount

		podCIDR                          = "100.96.0.0/11"
		minPortsPerVM                    = int32(2048)
		maxPortsPerVM                    = int32(65536)
		enableEndpointIndependentMapping = false
		enableDynamicPortAllocation      = false
		icmpIdleTimeoutSec               = int32(30)
		tcpEstablishedIdleTimeoutSec     = int32(1200)
		tcpTimeWaitTimeoutSec            = int32(120)
		tcpTransitoryIdleTimeoutSec      = int32(30)
		udpIdleTimeoutSec                = int32(30)

		cloudNatDefaults = map[string]interface{}{
			"minPortsPerVM":                    minPortsPerVM,
			"maxPortsPerVM":                    maxPortsPerVM,
			"enableEndpointIndependentMapping": enableEndpointIndependentMapping,
			"enableDynamicPortAllocation":      enableDynamicPortAllocation,
			"icmpIdleTimeoutSec":               icmpIdleTimeoutSec,
			"tcpEstablishedIdleTimeoutSec":     tcpEstablishedIdleTimeoutSec,
			"tcpTimeWaitTimeoutSec":            tcpTimeWaitTimeoutSec,
			"tcpTransitoryIdleTimeoutSec":      tcpTransitoryIdleTimeoutSec,
			"udpIdleTimeoutSec":                udpIdleTimeoutSec,
		}

		ctrl *gomock.Controller
		tf   *mockterraformer.MockTerraformer
		ctx  context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		tf = mockterraformer.NewMockTerraformer(ctrl)
		ctx = context.Background()

		internalCIDR := "192.168.0.0/16"

		config = &api.InfrastructureConfig{
			Networks: api.NetworkConfig{
				VPC: &api.VPC{
					Name: "vpc",
					CloudRouter: &api.CloudRouter{
						Name: "cloudrouter",
					},
				},
				DualStack: &api.DualStack{
					Enabled: false,
				},
				Internal: &internalCIDR,
				Workers:  "10.1.0.0/16",
			},
		}

		rawconfig := &apiv1alpha1.InfrastructureConfig{
			Networks: apiv1alpha1.NetworkConfig{
				VPC: &apiv1alpha1.VPC{
					Name: "vpc",
					CloudRouter: &apiv1alpha1.CloudRouter{
						Name: "cloudrouter",
					},
				},
				DualStack: &apiv1alpha1.DualStack{
					Enabled: false,
				},
				Internal: &internalCIDR,
				Workers:  "10.1.0.0/16",
			},
		}

		infra = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},

			Spec: extensionsv1alpha1.InfrastructureSpec{
				Region: "eu-west-1",
				SecretRef: corev1.SecretReference{
					Namespace: "foo",
					Name:      "gcp-credentials",
				},
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					ProviderConfig: &runtime.RawExtension{
						Object: rawconfig,
					},
				},
			},
		}

		projectID = "project"
		serviceAccountData = []byte(fmt.Sprintf(`{"project_id": "%s"}`, projectID))
		serviceAccount = &gcp.ServiceAccount{ProjectID: projectID, Raw: serviceAccountData}
	})

	Describe("#ExtractTerraformState", func() {
		It("should return correct state when cloudRouter name is specified", func() {
			var (
				cloudRouterName             = "test"
				vpcWithoutCloudRouterConfig = &api.InfrastructureConfig{
					Networks: api.NetworkConfig{
						VPC: &api.VPC{
							Name:        "vpc",
							CloudRouter: &api.CloudRouter{Name: cloudRouterName},
						},
						DualStack: &api.DualStack{
							Enabled: false,
						},
						Workers: "10.1.0.0/16",
					},
				}

				outputKeys = []string{
					TerraformerOutputKeyVPCName,
					TerraformerOutputKeySubnetNodes,
					TerraformerOutputKeyServiceAccountEmail,
					TerraformOutputKeyCloudRouter,
					TerraformOutputKeyCloudNAT,
				}
			)

			var (
				vpcName             = "vpc"
				subnetNodes         = "subnet"
				serviceAccountEmail = "email"
				cloudNATName        = "cloudnat"
			)

			tf.EXPECT().GetStateOutputVariables(ctx, outputKeys).DoAndReturn(func(_ context.Context, _ ...string) (map[string]string, error) {
				return map[string]string{
					TerraformerOutputKeyVPCName:             vpcName,
					TerraformerOutputKeySubnetNodes:         subnetNodes,
					TerraformerOutputKeyServiceAccountEmail: serviceAccountEmail,
					TerraformOutputKeyCloudRouter:           cloudRouterName,
					TerraformOutputKeyCloudNAT:              cloudNATName,
				}, nil
			})

			state, err := ExtractTerraformState(ctx, tf, vpcWithoutCloudRouterConfig, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(&TerraformState{
				VPCName:             vpcName,
				SubnetNodes:         subnetNodes,
				ServiceAccountEmail: serviceAccountEmail,
				CloudRouterName:     cloudRouterName,
				CloudNATName:        cloudNATName,
			}))
		})
		It("should return correct state when cloudRouter name is NOT specified", func() {
			var (
				vpcWithoutCloudRouterConfig = &api.InfrastructureConfig{
					Networks: api.NetworkConfig{
						VPC: &api.VPC{
							Name: "vpc",
						},
						Workers: "10.1.0.0/16",
					},
				}

				outputKeys = []string{
					TerraformerOutputKeyVPCName,
					TerraformerOutputKeySubnetNodes,
					TerraformerOutputKeyServiceAccountEmail,
				}
			)

			var (
				vpcName             = "vpc"
				subnetNodes         = "subnet"
				serviceAccountEmail = "email"
			)

			tf.EXPECT().GetStateOutputVariables(ctx, outputKeys).DoAndReturn(func(_ context.Context, _ ...string) (map[string]string, error) {
				return map[string]string{
					TerraformerOutputKeyVPCName:             vpcName,
					TerraformerOutputKeySubnetNodes:         subnetNodes,
					TerraformerOutputKeyServiceAccountEmail: serviceAccountEmail,
				}, nil
			})

			state, err := ExtractTerraformState(ctx, tf, vpcWithoutCloudRouterConfig, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(state).To(Equal(&TerraformState{
				VPCName:             vpcName,
				SubnetNodes:         subnetNodes,
				ServiceAccountEmail: serviceAccountEmail,
			}))
		})
	})

	Describe("#ComputeTerraformerTemplateValues", func() {
		It("should correctly compute the terraformer chart values without serviceAccount", func() {
			values, err := ComputeTerraformerTemplateValues(infra, serviceAccount, config, &podCIDR, false)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"google": map[string]interface{}{
					"region":  infra.Spec.Region,
					"project": projectID,
				},
				"create": map[string]interface{}{
					"vpc":            false,
					"cloudRouter":    false,
					"serviceAccount": false,
				},
				"vpc": map[string]interface{}{
					"name": strconv.Quote(config.Networks.VPC.Name),
					"cloudRouter": map[string]interface{}{
						"name": "cloudrouter",
					},
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"workers":   config.Networks.Workers,
					"internal":  config.Networks.Internal,
					"cloudNAT":  cloudNatDefaults,
					"dualStack": config.Networks.DualStack.Enabled,
				},
				"podCIDR": podCIDR,
				"outputKeys": map[string]interface{}{
					"vpcName":             TerraformerOutputKeyVPCName,
					"cloudNAT":            TerraformOutputKeyCloudNAT,
					"cloudRouter":         TerraformOutputKeyCloudRouter,
					"subnetNodes":         TerraformerOutputKeySubnetNodes,
					"subnetInternal":      TerraformerOutputKeySubnetInternal,
					"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
				},
			}))
		})

		It("should correctly compute the terraformer chart values with serviceAccount", func() {
			values, err := ComputeTerraformerTemplateValues(infra, serviceAccount, config, &podCIDR, true)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"google": map[string]interface{}{
					"region":  infra.Spec.Region,
					"project": projectID,
				},
				"create": map[string]interface{}{
					"vpc":            false,
					"cloudRouter":    false,
					"serviceAccount": true,
				},
				"vpc": map[string]interface{}{
					"name": strconv.Quote(config.Networks.VPC.Name),
					"cloudRouter": map[string]interface{}{
						"name": "cloudrouter",
					},
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"workers":   config.Networks.Workers,
					"internal":  config.Networks.Internal,
					"cloudNAT":  cloudNatDefaults,
					"dualStack": config.Networks.DualStack.Enabled,
				},
				"podCIDR": podCIDR,
				"outputKeys": map[string]interface{}{
					"vpcName":             TerraformerOutputKeyVPCName,
					"cloudNAT":            TerraformOutputKeyCloudNAT,
					"cloudRouter":         TerraformOutputKeyCloudRouter,
					"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
					"subnetNodes":         TerraformerOutputKeySubnetNodes,
					"subnetInternal":      TerraformerOutputKeySubnetInternal,
				},
			}))
		})

		It("should correctly compute the terraformer chart values with external CloudNAT IPs and EIM", func() {
			infra.Spec.Region = "europe-west1"
			projectID = "project"
			internalCIDR := "192.168.0.0/16"
			ipName1 := "manualnat1"
			ipName2 := "manualnat2"
			natIPNamesInput := []api.NatIPName{{Name: ipName1}, {Name: ipName2}}
			natIPNamesOutput := []string{ipName1, ipName2}

			config = &api.InfrastructureConfig{
				Networks: api.NetworkConfig{
					VPC: &api.VPC{
						Name: "vpc",
						CloudRouter: &api.CloudRouter{
							Name: "cloudrouter",
						},
					},
					CloudNAT: &api.CloudNAT{
						MinPortsPerVM: &minPortsPerVM,
						NatIPNames:    natIPNamesInput,
						EndpointIndependentMapping: &api.EndpointIndependentMapping{
							Enabled: true,
						},
					},
					Internal: &internalCIDR,
					DualStack: &api.DualStack{
						Enabled: false,
					},
					Workers: "10.1.0.0/16",
				},
			}

			values, err := ComputeTerraformerTemplateValues(infra, serviceAccount, config, &podCIDR, true)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"google": map[string]interface{}{
					"region":  infra.Spec.Region,
					"project": projectID,
				},
				"create": map[string]interface{}{
					"vpc":            false,
					"cloudRouter":    false,
					"serviceAccount": true,
				},
				"vpc": map[string]interface{}{
					"name": strconv.Quote(config.Networks.VPC.Name),
					"cloudRouter": map[string]interface{}{
						"name": "cloudrouter",
					},
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"workers":   config.Networks.Workers,
					"dualStack": config.Networks.DualStack.Enabled,
					"internal":  config.Networks.Internal,
					"cloudNAT": map[string]interface{}{
						"minPortsPerVM":                    minPortsPerVM,
						"natIPNames":                       natIPNamesOutput,
						"enableEndpointIndependentMapping": true,
						// The rest are the defaults
						"maxPortsPerVM":                maxPortsPerVM,
						"enableDynamicPortAllocation":  enableDynamicPortAllocation,
						"icmpIdleTimeoutSec":           icmpIdleTimeoutSec,
						"tcpEstablishedIdleTimeoutSec": tcpEstablishedIdleTimeoutSec,
						"tcpTimeWaitTimeoutSec":        tcpTimeWaitTimeoutSec,
						"tcpTransitoryIdleTimeoutSec":  tcpTransitoryIdleTimeoutSec,
						"udpIdleTimeoutSec":            udpIdleTimeoutSec,
					},
				},
				"podCIDR": podCIDR,
				"outputKeys": map[string]interface{}{
					"vpcName":             TerraformerOutputKeyVPCName,
					"cloudNAT":            TerraformOutputKeyCloudNAT,
					"cloudRouter":         TerraformOutputKeyCloudRouter,
					"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
					"subnetNodes":         TerraformerOutputKeySubnetNodes,
					"subnetInternal":      TerraformerOutputKeySubnetInternal,
					"natIPs":              TerraformOutputKeyNATIPs,
				},
			}))
		})

		It("should correctly compute the terraformer chart values with vpc flow logs", func() {
			internalCIDR := "192.168.0.0/16"
			aggregationInterval := "INTERVAL_30_SEC"
			metadata := "INCLUDE_ALL_METADATA"
			flowSampling := float64(0.5)
			config = &api.InfrastructureConfig{
				Networks: api.NetworkConfig{
					VPC: &api.VPC{
						Name: "vpc",
						CloudRouter: &api.CloudRouter{
							Name: "cloudrouter",
						},
					},
					DualStack: &api.DualStack{
						Enabled: false,
					},
					FlowLogs: &api.FlowLogs{
						AggregationInterval: &aggregationInterval,
						FlowSampling:        &flowSampling,
						Metadata:            &metadata,
					},
					Internal: &internalCIDR,
					Workers:  "10.1.0.0/16",
				},
			}

			values, err := ComputeTerraformerTemplateValues(infra, serviceAccount, config, &podCIDR, true)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"google": map[string]interface{}{
					"region":  infra.Spec.Region,
					"project": projectID,
				},
				"create": map[string]interface{}{
					"vpc":            false,
					"cloudRouter":    false,
					"serviceAccount": true,
				},
				"vpc": map[string]interface{}{
					"name": strconv.Quote(config.Networks.VPC.Name),
					"cloudRouter": map[string]interface{}{
						"name": "cloudrouter",
					},
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"workers":   config.Networks.Workers,
					"internal":  config.Networks.Internal,
					"dualStack": config.Networks.DualStack.Enabled,
					"cloudNAT":  cloudNatDefaults,
					"flowLogs": map[string]interface{}{
						"aggregationInterval": *config.Networks.FlowLogs.AggregationInterval,
						"flowSampling":        *config.Networks.FlowLogs.FlowSampling,
						"metadata":            *config.Networks.FlowLogs.Metadata,
					},
				},
				"podCIDR": podCIDR,
				"outputKeys": map[string]interface{}{
					"vpcName":             TerraformerOutputKeyVPCName,
					"cloudNAT":            TerraformOutputKeyCloudNAT,
					"cloudRouter":         TerraformOutputKeyCloudRouter,
					"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
					"subnetNodes":         TerraformerOutputKeySubnetNodes,
					"subnetInternal":      TerraformerOutputKeySubnetInternal,
				},
			}))
		})

		It("should correctly compute the terraformer chart values with vpc creation", func() {
			config.Networks.VPC = nil
			values, err := ComputeTerraformerTemplateValues(infra, serviceAccount, config, &podCIDR, true)
			Expect(err).To(BeNil())
			Expect(values).To(Equal(map[string]interface{}{
				"google": map[string]interface{}{
					"region":  infra.Spec.Region,
					"project": projectID,
				},
				"create": map[string]interface{}{
					"vpc":            true,
					"cloudRouter":    true,
					"serviceAccount": true,
				},
				"vpc": map[string]interface{}{
					"name": DefaultVPCName,
				},
				"clusterName": infra.Namespace,
				"networks": map[string]interface{}{
					"workers":   config.Networks.Workers,
					"internal":  config.Networks.Internal,
					"dualStack": config.Networks.DualStack.Enabled,
					"cloudNAT":  cloudNatDefaults,
				},
				"podCIDR": podCIDR,
				"outputKeys": map[string]interface{}{
					"vpcName":             TerraformerOutputKeyVPCName,
					"cloudNAT":            TerraformOutputKeyCloudNAT,
					"cloudRouter":         TerraformOutputKeyCloudRouter,
					"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
					"subnetNodes":         TerraformerOutputKeySubnetNodes,
					"subnetInternal":      TerraformerOutputKeySubnetInternal,
				},
			}))
		})
	})

	Describe("#StatusFromTerraformState", func() {
		var (
			serviceAccountEmail string
			vpcName             string
			cloudRouterName     string
			cloudNATName        string
			subnetNodes         string
			subnetInternal      string

			state *TerraformState
		)

		BeforeEach(func() {
			serviceAccountEmail = "gardener@cloud"
			vpcName = "vpc-name"
			cloudRouterName = "cloudrouter-name"
			cloudNATName = "cloudnat-name"
			subnetNodes = "nodes-subnet"
			subnetInternal = "internal"

			state = &TerraformState{
				VPCName:             vpcName,
				CloudRouterName:     cloudRouterName,
				CloudNATName:        cloudNATName,
				ServiceAccountEmail: serviceAccountEmail,
				SubnetNodes:         subnetNodes,
				SubnetInternal:      &subnetInternal,
			}
		})

		It("should correctly compute the status", func() {
			status := StatusFromTerraformState(state)

			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				Networks: apiv1alpha1.NetworkStatus{
					VPC: apiv1alpha1.VPC{
						Name:        vpcName,
						CloudRouter: &apiv1alpha1.CloudRouter{Name: cloudRouterName},
					},
					Subnets: []apiv1alpha1.Subnet{
						{
							Purpose: apiv1alpha1.PurposeNodes,
							Name:    subnetNodes,
						},
						{
							Purpose: apiv1alpha1.PurposeInternal,
							Name:    subnetInternal,
						},
					},
				},
				ServiceAccountEmail: serviceAccountEmail,
			}))
		})

		It("should correctly compute the status without internal subnet", func() {
			state.SubnetInternal = nil
			status := StatusFromTerraformState(state)

			Expect(status).To(Equal(&apiv1alpha1.InfrastructureStatus{
				TypeMeta: StatusTypeMeta,
				Networks: apiv1alpha1.NetworkStatus{
					VPC: apiv1alpha1.VPC{
						Name:        vpcName,
						CloudRouter: &apiv1alpha1.CloudRouter{Name: cloudRouterName},
					},
					Subnets: []apiv1alpha1.Subnet{
						{
							Purpose: apiv1alpha1.PurposeNodes,
							Name:    subnetNodes,
						},
					},
				},
				ServiceAccountEmail: serviceAccountEmail,
			}))
		})
	})
})
