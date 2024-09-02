package infraflow

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

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

func targetSubnetState(name, description, cidr, networkName string, flowLogs *gcp.FlowLogs) *compute.Subnetwork {
	subnet := &compute.Subnetwork{
		Description:           description,
		PrivateIpGoogleAccess: false,
		Name:                  name,
		IpCidrRange:           cidr,
		Network:               networkName,
		EnableFlowLogs:        false,
		LogConfig:             nil,
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
		ForceSendFields: []string{"Disabled", "Priority"},
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
		ForceSendFields: []string{"Disabled", "Priority"},
		NullFields:      []string{"Denied", "DestinationRanges", "SourceServiceAccounts", "SourceTags", "TargetTags", "TargetServiceAccounts"},
	}
}

func firewallRuleAllowHealthChecks(name, network string) *compute.Firewall {
	return &compute.Firewall{
		Name:      name,
		Network:   network,
		Direction: "INGRESS",
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
		ForceSendFields: []string{"Disabled", "Priority"},
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

// isSimilar return whether to slices are similar using reflect.DeepEqual
//
// Similar in this context means they contain the same elements, not necessarily in the same order.
// In essence, this is set equality using DeepEqual.
func isSimilar[T any](s1, s2 []T) bool {
	if len(s1) != len(s2) {
		return false
	}
	for _, v := range s1 {
		if !slices.ContainsFunc(s2, func(e T) bool { return reflect.DeepEqual(v, e) }) {
			return false
		}
	}
	return true
}

// firewallUpdate returns a firewall rule that reflects the differences from 'left' towards 'right'.
//
// The intended use for the returned firewall rule is to serve as a minimal patch when updating
// firewall rules on gcp.
func firewallUpdate(left, right *compute.Firewall) (*compute.Firewall, error) {
	if left == right {
		// both identical, nothing to do
		return nil, nil
	}
	if left == nil && right != nil {
		// everything is right, return right
		return right, nil
	}
	if right == nil && left != nil {
		// strictly speaking, the update here would be an empty Firewall-object with its
		// ForceSendFields set to all fields. In the intended context of this function,
		// just return nil signalling that there is nothing to do
		return nil, nil
	}

	// The basic idea is to construct a map holding all the differences from left to right and
	// then use Golang's json-marshalling to create an actual Firewall-instance with the
	// corresponding attributes set to the expected values.
	// If only there was a way to supply function args from maps, like foo(**kwargs) :(
	diff := map[string]interface{}{}

	// Note: CreationTimestamp is ignored, everything else is checked.

	if len(right.Allowed) != 0 {
		// n.b.: This replaces the entries on the left, it does not merge (same behaviour as Terraform)
		if len(left.Allowed) == 0 || !isSimilar(left.Allowed, right.Allowed) {
			diff["allowed"] = right.Allowed
		}
	}
	if len(right.Denied) != 0 {
		if len(left.Denied) == 0 || !isSimilar(left.Denied, right.Denied) {
			diff["denied"] = right.Denied
		}
	}
	if right.Description != "" && left.Description != right.Description {
		diff["description"] = right.Description
	}
	if len(right.DestinationRanges) != 0 {
		if len(left.DestinationRanges) == 0 || !isSimilar(left.DestinationRanges, right.DestinationRanges) {
			diff["destinationRanges"] = right.DestinationRanges
		}
	}

	if right.Direction != "" && left.Direction != right.Direction {
		diff["direction"] = right.Direction
	}
	if left.Disabled != right.Disabled {
		diff["disabled"] = right.Disabled
	}

	if right.Id != 0 && left.Id != right.Id {
		diff["id"] = fmt.Sprint(right.Id)
	}
	if right.Kind != "" && left.Kind != right.Kind {
		diff["kind"] = right.Kind
	}
	if right.LogConfig != nil {
		if left.LogConfig == nil {
			diff["logConfig"] = right.LogConfig
		} else {
			if left.LogConfig.Enable != right.LogConfig.Enable || left.LogConfig.Metadata != right.LogConfig.Metadata {
				diff["logConfig"] = right.LogConfig
			}
		}
	}
	if right.Name != "" && left.Name != right.Name {
		diff["name"] = right.Name
	}
	if right.Network != "" && left.Network != right.Network {
		diff["network"] = right.Network
	}
	if right.Priority != 0 && left.Priority != right.Priority {
		diff["priority"] = right.Priority
	}
	if right.SelfLink != "" && left.SelfLink != right.SelfLink {
		diff["selfLink"] = right.SelfLink
	}

	if len(right.SourceRanges) != 0 {
		if len(left.SourceRanges) == 0 || !isSimilar(left.SourceRanges, right.SourceRanges) {
			diff["sourceRanges"] = right.SourceRanges
		}
	}
	if len(right.SourceServiceAccounts) != 0 {
		if len(left.SourceServiceAccounts) == 0 || !isSimilar(left.SourceServiceAccounts, right.SourceServiceAccounts) {
			diff["sourceServiceAccounts"] = right.SourceServiceAccounts
		}
	}
	if len(right.SourceTags) != 0 {
		if len(left.SourceTags) == 0 || !isSimilar(left.SourceTags, right.SourceTags) {
			diff["sourceTags"] = right.SourceTags
		}
	}
	if len(right.TargetServiceAccounts) != 0 {
		if len(left.TargetServiceAccounts) == 0 || !isSimilar(left.TargetServiceAccounts, right.TargetServiceAccounts) {
			diff["targetServiceAccounts"] = right.TargetServiceAccounts
		}
	}
	if len(right.TargetTags) != 0 {
		if len(left.TargetTags) == 0 || !isSimilar(left.TargetTags, right.TargetTags) {
			diff["targetTags"] = right.TargetTags
		}
	}

	if len(diff) == 0 {
		return nil, nil
	}

	jsondiff, err := json.Marshal(diff)
	if err != nil {
		return nil, err
	}
	firewall := &compute.Firewall{}
	err = json.Unmarshal(jsondiff, firewall)

	if err != nil {
		return nil, err
	}
	return firewall, nil
}
