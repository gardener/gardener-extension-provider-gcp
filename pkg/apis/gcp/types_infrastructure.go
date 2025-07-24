// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	corev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureConfig infrastructure configuration resource
type InfrastructureConfig struct {
	metav1.TypeMeta

	// Networks is the network configuration (VPC, subnets, etc.)
	Networks NetworkConfig
}

// NetworkConfig holds information about the Kubernetes and infrastructure networks.
type NetworkConfig struct {
	// VPC indicates whether to use an existing VPC or create a new one.
	VPC *VPC
	// CloudNAT contains configuration about the CloudNAT resource
	CloudNAT *CloudNAT
	// Internal is a private subnet (used for internal load balancers).
	Internal *string
	// Worker is the worker subnet range to create (used for the VMs).
	// Deprecated - use `workers` instead.
	Worker string
	// Workers is the worker subnet range to create (used for the VMs).
	Workers string
	// FlowLogs contains the flow log configuration for the subnet.
	FlowLogs *FlowLogs
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureStatus contains information about created infrastructure resources.
type InfrastructureStatus struct {
	metav1.TypeMeta

	// Networks is the status of the networks of the infrastructure.
	Networks NetworkStatus

	// ServiceAccountEmail is the email address of the service account.
	ServiceAccountEmail string
}

// NetworkStatus is the current status of the infrastructure networks.
type NetworkStatus struct {
	// VPC states the name of the infrastructure VPC.
	VPC VPC

	// Subnets are the subnets that have been created.
	Subnets []Subnet

	// NatIPs is a list of all user provided external premium ips which can be used by the nat gateway
	NatIPs []NatIP

	// IPFamilies is the list of the used ip families.
	IPFamilies []corev1beta1.IPFamily
}

// SubnetPurpose is a purpose of a subnet.
type SubnetPurpose string

const (
	// PurposeNodes is a SubnetPurpose for nodes.
	PurposeNodes SubnetPurpose = "nodes"
	// PurposeInternal is a SubnetPurpose for internal use.
	PurposeInternal SubnetPurpose = "internal"
	// PurposeServices is a SubnetPurpose for services.
	PurposeServices SubnetPurpose = "services"
)

// Subnet is a subnet that was created.
type Subnet struct {
	// Name is the name of the subnet.
	Name string
	// Purpose is the purpose for which the subnet was created.
	Purpose SubnetPurpose
}

// VPC contains information about the VPC and some related resources.
type VPC struct {
	// Name is the VPC name.
	Name string
	// CloudRouter indicates whether to use an existing CloudRouter or create a new one
	CloudRouter *CloudRouter
}

// CloudRouter contains information about the CloudRouter configuration
type CloudRouter struct {
	// Name is the CloudRouter name.
	Name string
}

// CloudNAT contains configuration about the CloudNAT resource
type CloudNAT struct {
	// EndpointIndependentMapping controls if endpoint independent mapping is enabled.
	EndpointIndependentMapping *EndpointIndependentMapping
	// MinPortsPerVM is the minimum number of ports allocated to a VM in the NAT config.
	// The default value is 2048 ports.
	MinPortsPerVM *int32
	// MaxPortsPerVM is the maximum number of ports allocated to a VM in the NAT config.
	// The default value is 65536 ports.
	// +optional
	MaxPortsPerVM *int32
	// EnableDynamicPortAllocation controls port allocation behavior for the CloudNAT.
	// +optional
	EnableDynamicPortAllocation bool
	// NatIPNames is a list of all user provided external premium ips which can be used by the nat gateway
	// +optional
	// NatIPNames is a list of all names of user provided external premium ips which can be used by the nat gateway
	NatIPNames []NatIPName
	// IcmpIdleTimeoutSec is the timeout (in seconds) for ICMP connections. Defaults to 30.
	// +optional
	IcmpIdleTimeoutSec *int32
	// TcpEstablishedIdleTimeoutSec is the timeout (in seconds) for established TCP connections. Defaults to 1200.
	// +optional
	TcpEstablishedIdleTimeoutSec *int32
	// TcpTimeWaitTimeoutSec is the timeout (in seconds) for TCP connections in 'TIME_WAIT' state. Defaults to 120.
	// +optional
	TcpTimeWaitTimeoutSec *int32
	// TcpTransitoryIdleTimeoutSec is the timeout (in seconds) for transitory TCP connections. Defaults to 30.
	// +optional
	TcpTransitoryIdleTimeoutSec *int32
	// UDPIdleTimeoutSec is the timeout (in seconds) for UDP connections. Defaults to 30.
	// +optional
	UdpIdleTimeoutSec *int32
}

// EndpointIndependentMapping contains endpoint independent mapping options.
type EndpointIndependentMapping struct {
	// Enabled controls if endpoint independent mapping is enabled. Default is false.
	Enabled bool
}

// NatIP is a user provided external ip which can be used by the nat gateway
type NatIP struct {
	// IP is the external premium IP address used in GCP
	IP string
}

// NatIPName is the name of a user provided external ip address which can be used by the nat gateway
type NatIPName struct {
	// Name of the external premium ip address which is used in gcp
	Name string
}

// FlowLogs contains the configuration options for the vpc flow logs.
type FlowLogs struct {
	// AggregationInterval for collecting flow logs.
	AggregationInterval *string
	// FlowSampling sets the sampling rate of VPC flow logs within the subnetwork where 1.0 means all collected logs are reported and 0.0 means no logs are reported.
	FlowSampling *float64
	// Metadata configures whether metadata fields should be added to the reported VPC flow logs.
	Metadata *string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfrastructureState contains state information of the infrastructure resource.
type InfrastructureState struct {
	metav1.TypeMeta
	// Data is map to store things.
	// +optional
	Data map[string]string
	// Routes contains information about cluster routes
	// +optional
	Routes []Route
}

// Route is a structure containing information about the routes.
type Route struct {
	// InstanceName
	InstanceName string
	// DestinationCIDR
	DestinationCIDR string
	// Zone is the zone of the route
	Zone string
}
