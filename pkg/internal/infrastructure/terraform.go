// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apiv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultVPCName is the default VPC terraform name.
	DefaultVPCName = "google_compute_network.network.name"

	// TerraformerPurpose is the terraformer infrastructure purpose.
	TerraformerPurpose = "infra"

	// TerraformerOutputKeyVPCName is the name of the vpc_name terraform output variable.
	TerraformerOutputKeyVPCName = "vpc_name"
	// TerraformerOutputKeyServiceAccountEmail is the name of the service_account_email terraform output variable.
	TerraformerOutputKeyServiceAccountEmail = "service_account_email"
	// TerraformerOutputKeySubnetNodes is the name of the subnet_nodes terraform output variable.
	TerraformerOutputKeySubnetNodes = "subnet_nodes"
	// TerraformerOutputKeySubnetInternal is the name of the subnet_internal terraform output variable.
	TerraformerOutputKeySubnetInternal = "subnet_internal"
	// TerraformOutputKeyCloudNAT is the name of the cloud_nat terraform output variable.
	TerraformOutputKeyCloudNAT = "cloud_nat"
	// TerraformOutputKeyNATIPs is the name of the nat_ips terraform output variable.
	TerraformOutputKeyNATIPs = "nat_ips"
	// TerraformOutputKeyCloudRouter is the name of the cloud_router terraform output variable.
	TerraformOutputKeyCloudRouter = "cloud_router"
)

// StatusTypeMeta is the TypeMeta of the GCP InfrastructureStatus
var StatusTypeMeta = metav1.TypeMeta{
	APIVersion: apiv1alpha1.SchemeGroupVersion.String(),
	Kind:       "InfrastructureStatus",
}

// ComputeTerraformerTemplateValues computes the values for the GCP Terraformer chart.
func ComputeTerraformerTemplateValues(
	infra *extensionsv1alpha1.Infrastructure,
	account *gcp.ServiceAccount,
	config *api.InfrastructureConfig,
	podCIDR *string,
	createSA bool,
) (map[string]interface{}, error) {
	var (
		vpcName             = DefaultVPCName
		createVPC           = true
		createCloudRouter   = true
		cloudRouterName     string
		cN                  = map[string]interface{}{"minPortsPerVM": int32(2048)}
		privateGoogleAccess = false
	)

	if config.Networks.VPC != nil {
		vpcName = strconv.Quote(config.Networks.VPC.Name)
		createVPC = false
		createCloudRouter = false

		if config.Networks.VPC.CloudRouter != nil && len(config.Networks.VPC.CloudRouter.Name) > 0 {
			cloudRouterName = config.Networks.VPC.CloudRouter.Name
		}

		if config.Networks.VPC.EnablePrivateGoogleAccess {
			privateGoogleAccess = true
		}
	}

	if config.Networks.CloudNAT != nil {
		if config.Networks.CloudNAT.MinPortsPerVM != nil {
			cN["minPortsPerVM"] = *config.Networks.CloudNAT.MinPortsPerVM
		}
		if config.Networks.CloudNAT.NatIPNames != nil {
			natIPNames := []string{}
			for _, v := range config.Networks.CloudNAT.NatIPNames {
				natIPNames = append(natIPNames, v.Name)
			}
			cN["natIPNames"] = natIPNames
		}
	}

	vpc := map[string]interface{}{
		"name": vpcName,
	}

	if len(cloudRouterName) > 0 {
		vpc["cloudRouter"] = map[string]interface{}{
			"name": cloudRouterName,
		}
	}

	workersCIDR := config.Networks.Workers
	// Backwards compatibility - remove this code in a future version.
	if workersCIDR == "" {
		workersCIDR = config.Networks.Worker
	}

	outputKeys := map[string]interface{}{
		"vpcName":             TerraformerOutputKeyVPCName,
		"cloudNAT":            TerraformOutputKeyCloudNAT,
		"cloudRouter":         TerraformOutputKeyCloudRouter,
		"serviceAccountEmail": TerraformerOutputKeyServiceAccountEmail,
		"subnetNodes":         TerraformerOutputKeySubnetNodes,
		"subnetInternal":      TerraformerOutputKeySubnetInternal,
	}
	if manualNatIPsSet(config) {
		outputKeys["natIPs"] = TerraformOutputKeyNATIPs
	}

	values := map[string]interface{}{
		"google": map[string]interface{}{
			"region":  infra.Spec.Region,
			"project": account.ProjectID,
		},
		"create": map[string]interface{}{
			"vpc":            createVPC,
			"cloudRouter":    createCloudRouter,
			"serviceAccount": createSA,
		},
		"vpc":         vpc,
		"clusterName": infra.Namespace,
		"networks": map[string]interface{}{
			"workers":                   workersCIDR,
			"internal":                  config.Networks.Internal,
			"cloudNAT":                  cN,
			"enablePrivateGoogleAccess": privateGoogleAccess,
		},
		"podCIDR":    *podCIDR,
		"outputKeys": outputKeys,
	}

	if config.Networks.FlowLogs != nil {
		fl := make(map[string]interface{})

		if config.Networks.FlowLogs.AggregationInterval != nil {
			fl["aggregationInterval"] = *config.Networks.FlowLogs.AggregationInterval
		}

		if config.Networks.FlowLogs.FlowSampling != nil {
			fl["flowSampling"] = *config.Networks.FlowLogs.FlowSampling
		}

		if config.Networks.FlowLogs.Metadata != nil {
			fl["metadata"] = *config.Networks.FlowLogs.Metadata
		}

		values["networks"].(map[string]interface{})["flowLogs"] = fl
	}

	return values, nil
}

// RenderTerraformerTemplate renders the gcp-infra chart with the given values.
func RenderTerraformerTemplate(
	infra *extensionsv1alpha1.Infrastructure,
	account *gcp.ServiceAccount,
	config *api.InfrastructureConfig,
	podCIDR *string,
	createSA bool,
) (*TerraformFiles, error) {
	values, err := ComputeTerraformerTemplateValues(infra, account, config, podCIDR, createSA)
	if err != nil {
		return nil, fmt.Errorf("failed to compute terraform values: %v", err)
	}

	var mainTF bytes.Buffer
	if err := mainTemplate.Execute(&mainTF, values); err != nil {
		return nil, fmt.Errorf("could not render Terraform template: %+v", err)
	}

	return &TerraformFiles{
		Main:      mainTF.String(),
		Variables: variablesTF,
		TFVars:    terraformTFVars,
	}, nil
}

// TerraformFiles are the files that have been rendered from the infrastructure chart.
type TerraformFiles struct {
	Main      string
	Variables string
	TFVars    []byte
}

// TerraformState is the Terraform state for an infrastructure.
type TerraformState struct {
	// VPCName is the name of the VPC created for an infrastructure.
	VPCName string
	// CloudRouterName is the name of the created / existing cloud router
	CloudRouterName string
	// CloudNATName is the name of the created Cloud NAT
	CloudNATName string
	// NatIPs is a list of external ips for the nat gateway
	NatIPs []apiv1alpha1.NatIP
	// ServiceAccountEmail is the service account email for a network.
	ServiceAccountEmail string
	// SubnetNodes is the CIDR of the nodes subnet of an infrastructure.
	SubnetNodes string
	// SubnetInternal is the CIDR of the internal subnet of an infrastructure.
	SubnetInternal *string
}

// ExtractTerraformState extracts the TerraformState from the given Terraformer.
func ExtractTerraformState(
	ctx context.Context,
	tf terraformer.Terraformer,
	config *api.InfrastructureConfig,
	createSA bool,
) (*TerraformState, error) {
	var (
		outputKeys = []string{
			TerraformerOutputKeyVPCName,
			TerraformerOutputKeySubnetNodes,
		}
		vpcSpecifiedWithoutCloudRouter = config.Networks.VPC != nil && config.Networks.VPC.CloudRouter == nil
	)

	if createSA {
		outputKeys = append(outputKeys, TerraformerOutputKeyServiceAccountEmail)
	}

	if !vpcSpecifiedWithoutCloudRouter {
		outputKeys = append(outputKeys, TerraformOutputKeyCloudRouter, TerraformOutputKeyCloudNAT)
	}

	if manualNatIPsSet(config) {
		outputKeys = append(outputKeys, TerraformOutputKeyNATIPs)
	}

	hasInternal := config.Networks.Internal != nil
	if hasInternal {
		outputKeys = append(outputKeys, TerraformerOutputKeySubnetInternal)
	}

	vars, err := tf.GetStateOutputVariables(ctx, outputKeys...)
	if err != nil {
		return nil, err
	}

	state := &TerraformState{
		VPCName:     vars[TerraformerOutputKeyVPCName],
		SubnetNodes: vars[TerraformerOutputKeySubnetNodes],
	}

	if createSA {
		state.ServiceAccountEmail = vars[TerraformerOutputKeyServiceAccountEmail]
	}

	if manualNatIPsSet(config) {
		state.NatIPs = []apiv1alpha1.NatIP{}
		for _, ip := range strings.Split(vars[TerraformOutputKeyNATIPs], ",") {
			state.NatIPs = append(state.NatIPs, apiv1alpha1.NatIP{IP: ip})
		}
	}

	if !vpcSpecifiedWithoutCloudRouter {
		state.CloudRouterName = vars[TerraformOutputKeyCloudRouter]
		state.CloudNATName = vars[TerraformOutputKeyCloudNAT]
	}

	if hasInternal {
		subnetInternal := vars[TerraformerOutputKeySubnetInternal]
		state.SubnetInternal = &subnetInternal
	}

	return state, nil
}

// StatusFromTerraformState computes an InfrastructureStatus from the given
// Terraform variables.
func StatusFromTerraformState(state *TerraformState) *apiv1alpha1.InfrastructureStatus {
	status := &apiv1alpha1.InfrastructureStatus{
		TypeMeta: StatusTypeMeta,
		Networks: apiv1alpha1.NetworkStatus{
			VPC: apiv1alpha1.VPC{
				Name: state.VPCName,
			},
			Subnets: []apiv1alpha1.Subnet{
				{
					Purpose: apiv1alpha1.PurposeNodes,
					Name:    state.SubnetNodes,
				},
			},
		},
		ServiceAccountEmail: state.ServiceAccountEmail,
	}

	if len(state.CloudRouterName) > 0 {
		status.Networks.VPC.CloudRouter = &apiv1alpha1.CloudRouter{
			Name: state.CloudRouterName,
		}
	}

	if state.NatIPs != nil {
		status.Networks.NatIPs = state.NatIPs
	}

	if state.SubnetInternal != nil {
		status.Networks.Subnets = append(status.Networks.Subnets, apiv1alpha1.Subnet{
			Purpose: apiv1alpha1.PurposeInternal,
			Name:    *state.SubnetInternal,
		})
	}

	return status
}

// ComputeStatus computes the status based on the Terraformer and the given InfrastructureConfig.
func ComputeStatus(ctx context.Context, tf terraformer.Terraformer, config *api.InfrastructureConfig, createSA bool) (*apiv1alpha1.InfrastructureStatus, error) {
	state, err := ExtractTerraformState(ctx, tf, config, createSA)
	if err != nil {
		return nil, err
	}

	return StatusFromTerraformState(state), nil
}

func manualNatIPsSet(config *api.InfrastructureConfig) bool {
	return config.Networks.CloudNAT != nil && config.Networks.CloudNAT.NatIPNames != nil
}
