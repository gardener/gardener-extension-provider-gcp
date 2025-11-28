package infraflow

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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
	// DefaultSecondarySubnetName is the default name of the secondary ipv4 subnet that will be used in dualstack shoot
	DefaultSecondarySubnetName = "ipv4-pod-cidr"
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

func targetSubnetState(name, description, cidr, networkName string, flowLogs *gcp.FlowLogs, dualStack bool, secondaryRange *string) *compute.Subnetwork {
	subnet := &compute.Subnetwork{
		Description:           description,
		PrivateIpGoogleAccess: false, // Gardener GCP shoot clusters enable PGA by default through a NAT gateway
		Name:                  name,
		IpCidrRange:           cidr,
		Network:               networkName,
		EnableFlowLogs:        false,
		LogConfig:             nil,
	}

	if dualStack {
		subnet.Ipv6AccessType = "EXTERNAL"
		subnet.StackType = "IPV4_IPV6"

		if secondaryRange != nil {
			subnet.SecondaryIpRanges = []*compute.SubnetworkSecondaryRange{
				{
					IpCidrRange: *secondaryRange,
					RangeName:   DefaultSecondarySubnetName,
				},
			}
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

const (
	ipProtocolICMPv4 = "icmp"
	ipProtocolICMPv6 = "58" // we have use the number as GCP doesn't recognize any variants of the name for ICMPv6
)

func firewallRuleAllowInternal(name, network string, cidrs []*string) *compute.Firewall {
	firewall := &compute.Firewall{
		Name:      name,
		Network:   network,
		Direction: "INGRESS",
		Priority:  1000,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: ipProtocolICMPv4,
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

func firewallRuleAllowInternalIPv6(name, network string, cidrs []*string) *compute.Firewall {
	firewall := &compute.Firewall{
		Name:      name,
		Network:   network,
		Direction: "INGRESS",
		Priority:  1000,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: ipProtocolICMPv6,
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

// Following ranges documented at https://cloud.google.com/load-balancing/docs/health-check-concepts#ip-ranges
var (
	healthCheckSourceRangesIPv4 = []string{
		"35.191.0.0/16",
		"209.85.204.0/22",
		"209.85.152.0/22",
		"130.211.0.0/22",
	}

	healthCheckSourceRangesIPv6 = []string{
		"2600:2d00:1:b029::/64",
		"2600:2d00:1:1::/64",
		"2600:1901:8001::/48",
	}
)

func firewallRuleAllowHealthChecks(name, network string, cidrs []string) *compute.Firewall {
	return &compute.Firewall{
		Name:         name,
		Network:      network,
		Direction:    "INGRESS",
		Priority:     1000,
		SourceRanges: cidrs,
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

func isToDualStackMigration(shoot *gardencorev1beta1.Shoot) bool {
	nodesInMigration := false
	condition := gardencorev1beta1helper.GetCondition(shoot.Status.Constraints, "DualStackNodesMigrationReady")

	if condition != nil && condition.Status != gardencorev1beta1.ConditionTrue {
		nodesInMigration = true
	}
	return nodesInMigration
}

// extractInstanceAndZone parses the NextHopInstance URL to extract the instance and zone.
func extractInstanceAndZone(nextHopInstance string) (string, string, error) {
	u, err := url.Parse(nextHopInstance)
	if err != nil {
		return "", "", err
	}

	parts := strings.Split(u.Path, "/")
	if len(parts) < 8 {
		return "", "", fmt.Errorf("invalid NextHopInstance URL: %s", nextHopInstance)
	}

	zone := parts[6]
	instance := parts[8]

	return instance, zone, nil
}

// IPFamiliesFromCIDRs returns a slice of IP families (IPv4 / IPv6) corresponding
// to the provided CIDRs. Returns at most 2 families. If both CIDRs have the same
// IP family, returns a slice with a single element.
func IPFamiliesFromCIDRs(cidrs []string) []gardencorev1beta1.IPFamily {
	var result []gardencorev1beta1.IPFamily
	seen := make(map[gardencorev1beta1.IPFamily]bool)

	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if ip, _, err := net.ParseCIDR(c); err == nil {
			var family gardencorev1beta1.IPFamily
			if ip.To4() != nil {
				family = gardencorev1beta1.IPFamilyIPv4
			} else {
				family = gardencorev1beta1.IPFamilyIPv6
			}
			if !seen[family] {
				result = append(result, family)
				seen[family] = true
				if len(result) == 2 {
					break
				}
			}
		}
	}
	return result
}
