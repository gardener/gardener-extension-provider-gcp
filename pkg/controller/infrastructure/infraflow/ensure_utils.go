package infraflow

import (
	"fmt"
	"math/big"
	"net"

	"google.golang.org/api/compute/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
)

const (
	// DefaultVPCRoutingConfigRegional is a constant for VPC Routing configuration option that enables regional routing.
	DefaultVPCRoutingConfigRegional = "REGIONAL"
	// DefaultAggregationInterval is the default value for the aggregation interval.
	DefaultAggregationInterval = "INTERVAL_5_MIN"
	// DefaultFlowSampling is the default value for the flow sampling.
	DefaultFlowSampling = 0.5
	// DefaultMetadata is the default value for the Flow Logs metadata.
	DefaultMetadata = "EXCLUDE_ALL_METADATA"
)

// GetObject returns the object and attempts to cast it to the specified type.
func GetObject[T any](wb shared.Whiteboard, key string) T {
	if ok := wb.HasObject(key); !ok {
		return *new(T)
	}
	o := wb.GetObject(key)
	return o.(T)
}

func (fctx *FlowContext) ensureObjectKeys(keys ...string) error {
	for _, k := range keys {
		if fctx.whiteboard.GetObject(k) == nil {
			return fmt.Errorf("could not locate required key: %s", k)
		}
	}
	return nil
}

func (fctx *FlowContext) serviceAccountNameFromConfig() string {
	return fctx.clusterName
}

func (fctx *FlowContext) vpcNameFromConfig() string {
	vpcName := fctx.clusterName
	if fctx.config.Networks.VPC != nil {
		vpcName = fctx.config.Networks.VPC.Name
	}
	return vpcName
}

func (fctx *FlowContext) subnetNameFromConfig() string {
	return fmt.Sprintf("%s-nodes", fctx.clusterName)
}

func (fctx *FlowContext) internalSubnetNameFromConfig() string {
	return fmt.Sprintf("%s-internal", fctx.clusterName)
}

func (fctx *FlowContext) servicesSubnetNameFromConfig() string {
	return fmt.Sprintf("%s-services", fctx.clusterName)
}

func (fctx *FlowContext) cloudRouterNameFromConfig() string {
	routerName := fmt.Sprintf("%s-cloud-router", fctx.clusterName)
	if fctx.config.Networks.VPC != nil && fctx.config.Networks.VPC.CloudRouter != nil {
		routerName = fctx.config.Networks.VPC.CloudRouter.Name
	}
	return routerName
}

func (fctx *FlowContext) cloudNatNameFromConfig() string {
	return fmt.Sprintf("%s-cloud-nat", fctx.clusterName)
}

func firewallRuleAllowInternalName(base string) string {
	return fmt.Sprintf("%s-allow-internal-access", base)
}

func firewallRuleAllowExternalName(base string) string {
	return fmt.Sprintf("%s-allow-external-access", base)
}

func firewallRuleAllowHealthChecksName(base string) string {
	return fmt.Sprintf("%s-allow-health-checks", base)
}

func targetNetwork(name string) *compute.Network {
	return &compute.Network{
		Name:                  name,
		AutoCreateSubnetworks: false,
		RoutingConfig: &compute.NetworkRoutingConfig{
			RoutingMode: DefaultVPCRoutingConfigRegional,
		},
		ForceSendFields: []string{"AutoCreateSubnetworks"},
	}
}

func targetSubnetState(name, description, cidr, networkName string, flowLogs *gcp.FlowLogs, dualStack *gcp.DualStack) *compute.Subnetwork {
	subnet := &compute.Subnetwork{
		Description:           description,
		PrivateIpGoogleAccess: false,
		Name:                  name,
		IpCidrRange:           cidr,
		Network:               networkName,
		EnableFlowLogs:        false,
		LogConfig:             nil,
	}

	if dualStack.Enabled {
		subnet.Ipv6AccessType = "EXTERNAL"
		subnet.StackType = "IPV4_IPV6"
		subnet.SecondaryIpRanges = []*compute.SubnetworkSecondaryRange{
			{
				IpCidrRange: "192.168.0.0/16",
				RangeName:   "ipv4-pod-cidr",
			},
		}
	}

	if flowLogs != nil {
		subnet.EnableFlowLogs = true
		subnet.LogConfig = &compute.SubnetworkLogConfig{}

		subnet.LogConfig.AggregationInterval = DefaultAggregationInterval
		if flowLogs.AggregationInterval != nil {
			subnet.LogConfig.AggregationInterval = *flowLogs.AggregationInterval
		}

		subnet.LogConfig.FlowSampling = DefaultFlowSampling
		if flowLogs.FlowSampling != nil {
			subnet.LogConfig.FlowSampling = *flowLogs.FlowSampling
		}

		subnet.LogConfig.Metadata = DefaultMetadata
		if flowLogs.Metadata != nil {
			subnet.LogConfig.Metadata = *flowLogs.Metadata
		}
	}

	return subnet
}

func targetRouterState(name, description, vpcName string) *compute.Router {
	return &compute.Router{
		Name:        name,
		Description: description,
		Network:     vpcName,
	}
}

func targetNATState(name, subnetURL string, natConfig *gcp.CloudNAT, natIps []*compute.Address) *compute.RouterNat {
	nat := &compute.RouterNat{
		DrainNatIps:                      nil,
		EnableDynamicPortAllocation:      false,
		EnableEndpointIndependentMapping: false,
		EndpointTypes:                    nil,
		LogConfig: &compute.RouterNatLogConfig{
			Enable: true,
			Filter: "ERRORS_ONLY",
		},
		MaxPortsPerVm:                 65536,
		MinPortsPerVm:                 2048,
		Name:                          name,
		NatIpAllocateOption:           "AUTO_ONLY",
		NatIps:                        nil,
		Rules:                         nil,
		SourceSubnetworkIpRangesToNat: "LIST_OF_SUBNETWORKS",
		Subnetworks: []*compute.RouterNatSubnetworkToNat{
			{
				Name:                subnetURL,
				SourceIpRangesToNat: []string{"ALL_IP_RANGES"},
			},
		},
		IcmpIdleTimeoutSec:           30,
		TcpEstablishedIdleTimeoutSec: 1200,
		TcpTimeWaitTimeoutSec:        120,
		TcpTransitoryIdleTimeoutSec:  30,
		UdpIdleTimeoutSec:            30,
		ForceSendFields:              nil,
		NullFields:                   nil,
	}

	if natConfig != nil {
		nat.EnableDynamicPortAllocation = natConfig.EnableDynamicPortAllocation
		if natConfig.MinPortsPerVM != nil {
			nat.MinPortsPerVm = int64(*natConfig.MinPortsPerVM)
		}

		if natConfig.MaxPortsPerVM != nil {
			nat.MaxPortsPerVm = int64(*natConfig.MaxPortsPerVM)
		}

		if natConfig.EndpointIndependentMapping != nil {
			nat.EnableEndpointIndependentMapping = natConfig.EndpointIndependentMapping.Enabled
		}

		if natConfig.IcmpIdleTimeoutSec != nil {
			nat.IcmpIdleTimeoutSec = int64(*natConfig.IcmpIdleTimeoutSec)
		}

		if natConfig.TcpEstablishedIdleTimeoutSec != nil {
			nat.TcpEstablishedIdleTimeoutSec = int64(*natConfig.TcpEstablishedIdleTimeoutSec)
		}

		if natConfig.TcpTimeWaitTimeoutSec != nil {
			nat.TcpTimeWaitTimeoutSec = int64(*natConfig.TcpTimeWaitTimeoutSec)
		}

		if natConfig.TcpTransitoryIdleTimeoutSec != nil {
			nat.TcpTransitoryIdleTimeoutSec = int64(*natConfig.TcpTransitoryIdleTimeoutSec)
		}

		if natConfig.UdpIdleTimeoutSec != nil {
			nat.UdpIdleTimeoutSec = int64(*natConfig.UdpIdleTimeoutSec)
		}
	}

	if len(natIps) > 0 {
		nat.NatIpAllocateOption = "MANUAL_ONLY"
		for _, natIp := range natIps {
			nat.NatIps = append(nat.NatIps, natIp.SelfLink)
		}
	}
	return nat
}

func firewallRuleAllowInternal(name, network string, cidrs []*string) *compute.Firewall {
	firewall := &compute.Firewall{
		Name:      name,
		Network:   network,
		Direction: "INGRESS",
		Priority:  1000,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "icmp",
			},
			{
				IPProtocol: "ipip",
			},
			{
				IPProtocol: "tcp",
				Ports:      []string{"1-65535"},
			},
			{
				IPProtocol: "udp",
				Ports:      []string{"1-65535"},
			},
		},
		ForceSendFields: []string{"Disabled"},
		NullFields:      []string{"Denied", "DestinationRanges", "SourceServiceAccounts", "SourceTags", "TargetTags", "TargetServiceAccounts"},
	}
	for _, cidr := range cidrs {
		if cidr != nil && len(*cidr) > 0 {
			firewall.SourceRanges = append(firewall.SourceRanges, *cidr)
		}
	}

	return firewall
}

func firewallRuleAllowExternal(name, network string) *compute.Firewall {
	return &compute.Firewall{
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "tcp",
				Ports:      []string{"443"},
			},
		},
		SourceRanges:    []string{"0.0.0.0/0"},
		Direction:       "INGRESS",
		Name:            name,
		Network:         network,
		Priority:        1000,
		ForceSendFields: []string{"Disabled"},
		NullFields:      []string{"Denied", "DestinationRanges", "SourceServiceAccounts", "SourceTags", "TargetTags", "TargetServiceAccounts"},
	}
}

func firewallRuleAllowHealthChecks(name, network string) *compute.Firewall {
	return &compute.Firewall{
		Name:      name,
		Network:   network,
		Direction: "INGRESS",
		Priority:  1000,
		SourceRanges: []string{
			"35.191.0.0/16",
			"209.85.204.0/22",
			"209.85.152.0/22",
			"130.211.0.0/22",
		},
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "udp",
				Ports:      []string{"30000-32767"},
			},
			{
				IPProtocol: "tcp",
				Ports:      []string{"30000-32767"},
			},
		},
		ForceSendFields: []string{"Disabled"},
		NullFields:      []string{"Denied", "DestinationRanges", "SourceServiceAccounts", "SourceTags", "TargetTags", "TargetServiceAccounts"},
	}
}

func isUserRouter(config *gcp.InfrastructureConfig) bool {
	return config.Networks.VPC != nil &&
		config.Networks.VPC.CloudRouter != nil &&
		len(config.Networks.VPC.CloudRouter.Name) > 0
}

func isUserVPC(config *gcp.InfrastructureConfig) bool {
	return config.Networks.VPC != nil && len(config.Networks.VPC.Name) > 0
}

// Helper function to convert IP to big.Int
func ipToBigInt(ip net.IP) *big.Int {
	i := new(big.Int)
	i.SetBytes(ip)
	return i
}

// Helper function to convert big.Int to IP
func bigIntToIP(i *big.Int) net.IP {
	return net.IP(i.Bytes())
}

// Function to get the latest /96 subnet in the /64 range
func getLast96Subnet(prefix string) (string, error) {
	// Parse the IP and CIDR notation
	ip, ipnet, err := net.ParseCIDR(prefix)
	if err != nil {
		return "", fmt.Errorf("failed to parse prefix: %v", err)
	}

	// Check if the prefix is a /64 IPv6 prefix
	if ones, bits := ipnet.Mask.Size(); ones != 64 || bits != 128 {
		return "", fmt.Errorf("prefix must be a /64 IPv6 prefix")
	}

	// Convert the IP to a 16-byte array and set bits 65-96 to 1
	ip[8], ip[9], ip[10], ip[11] = 0xFF, 0xFF, 0xFF, 0xFF

	// Return the result as a /96 prefix
	return fmt.Sprintf("%s/96", ip.String()), nil
}
