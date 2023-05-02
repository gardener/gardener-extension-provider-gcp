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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta `json:",inline"`

	// Networks is the network configuration (VPC, subnets, etc.)
	Networks NetworkConfig `json:"networks"`
}

// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
	// VPC indicates whether to use an existing VPC or create a new one.
	// +optional
	VPC *VPC `json:"vpc,omitempty"`
	// CloudNAT contains configuration about the the CloudNAT resource
	// +optional
	CloudNAT *CloudNAT `json:"cloudNAT,omitempty"`
	// Internal is a private subnet (used for internal load balancers).
	// +optional
	Internal *string `json:"internal,omitempty"`
	// Worker is the worker subnet range to create (used for the VMs).
	// Deprecated - use `workers` instead.
	Worker string `json:"worker"`
	// Workers is the worker subnet range to create (used for the VMs).
	Workers string `json:"workers"`
	// FlowLogs contains the flow log configuration for the subnet.
	// +optional
	FlowLogs *FlowLogs `json:"flowLogs,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta `json:",inline"`

	// Networks is the status of the networks of the infrastructure.
	Networks NetworkStatus `json:"networks"`

	// ServiceAccountEmail is the email address of the service account.
	ServiceAccountEmail string `json:"serviceAccountEmail"`
}

// NetworkStatus is the current status of the infrastructure networks.
type NetworkStatus struct {
	// VPC states the name of the infrastructure VPC.
	VPC VPC `json:"vpc"`

	// Subnets are the subnets that have been created.
	Subnets []Subnet `json:"subnets"`

	// NatIPs is a list of all user provided external premium ips which can be used by the nat gateway
	// +optional
	NatIPs []NatIP `json:"natIPs,omitempty"`
}

// SubnetPurpose is a purpose of a subnet.
type SubnetPurpose string

const (
	// PurposeNodes is a SubnetPurpose for nodes.
	PurposeNodes SubnetPurpose = "nodes"
	// PurposeInternal is a SubnetPurpose for internal use.
	PurposeInternal SubnetPurpose = "internal"
)

// Subnet is a subnet that was created.
type Subnet struct {
	// Name is the name of the subnet.
	Name string `json:"name"`
	// Purpose is the purpose for which the subnet was created.
	Purpose SubnetPurpose `json:"purpose"`
}

// VPC contains information about the VPC and some related resources.
type VPC struct {
	// Name is the VPC name.
	Name string `json:"name,omitempty"`
	// CloudRouter indicates whether to use an existing CloudRouter or create a new one
	// +optional
	CloudRouter *CloudRouter `json:"cloudRouter,omitempty"`
}

// CloudRouter contains information about the the CloudRouter configuration
type CloudRouter struct {
	// Name is the CloudRouter name.
	Name string `json:"name,omitempty"`
}

// CloudNAT contains configuration about the CloudNAT resource
type CloudNAT struct {
	// EndpointIndependentMapping controls if endpoint independent mapping is enabled.
	EndpointIndependentMapping *EndpointIndependentMapping `json:"endpointIndependentMapping,omitempty"`
	// MinPortsPerVM is the minimum number of ports allocated to a VM in the NAT config.
	// The default value is 2048 ports.
	// +optional
	MinPortsPerVM *int32 `json:"minPortsPerVM,omitempty"`
	// NatIPNames is a list of all user provided external premium ips which can be used by the nat gateway
	// +optional
	NatIPNames []NatIPName `json:"natIPNames,omitempty"`
}

// EndpointIndependentMapping contains endpoint independent mapping options.
type EndpointIndependentMapping struct {
	// Enabled controls if endpoint independent mapping is enabled. Default is false.
	Enabled bool `json:"enabled"`
}

// NatIP is a user provided external ip which can be used by the nat gateway
type NatIP struct {
	// IP is the external premium IP address used in GCP
	IP string `json:"ip"`
}

// NatIPName is the name of a user provided external ip address which can be used by the nat gateway
type NatIPName struct {
	// Name of the external premium ip address which is used in gcp
	Name string `json:"name"`
}

// FlowLogs contains the configuration options for the vpc flow logs.
type FlowLogs struct {
	// AggregationInterval for collecting flow logs.
	// +optional
	AggregationInterval *string `json:"aggregationInterval,omitempty"`
	// FlowSampling sets the sampling rate of VPC flow logs within the subnetwork where 1.0 means all collected logs are reported and 0.0 means no logs are reported.
	// +optional
	FlowSampling *float32 `json:"flowSampling,omitempty"`
	// Metadata configures whether metadata fields should be added to the reported VPC flow logs.
	// +optional
	Metadata *string `json:"metadata,omitempty"`
}
