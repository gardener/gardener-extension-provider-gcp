package client

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
	"google.golang.org/api/compute/v1"
)

var _ Updater = &updater{}

// Updater can perform operations to update infrastructure resources.
type Updater interface {
	// VPC updates the respective infrastructure object according to desired state.
	VPC(ctx context.Context, desired, current *compute.Network) (*compute.Network, error)
	// Subnet updates the respective infrastructure object according to desired state.
	Subnet(ctx context.Context, region string, desired, current *compute.Subnetwork) (*compute.Subnetwork, error)
	// Router updates the respective infrastructure object according to desired state.
	Router(ctx context.Context, region string, desired, current *compute.Router) (*compute.Router, error)
	// NAT updates the respective infrastructure object according to desired state.
	NAT(ctx context.Context, region string, router compute.Router, desired compute.RouterNat) (*compute.Router, *compute.RouterNat, error)
	// DeleteNAT deletes the NAT gateway from the respective router.
	// Since NAT Gateways are a property of Cloud Routers, DeleteNAT is a special update function for the CloudRouter that
	// will remove the NAT with specified name.
	DeleteNAT(ctx context.Context, region, router, nat string) (*compute.Router, error)
	// Firewall updates the respective infrastructure object according to desired state.
	Firewall(ctx context.Context, desired, current *compute.Firewall) (*compute.Firewall, error)
}

type updater struct {
	log     logr.Logger
	compute ComputeClient
}

// NewUpdater returns a new instance of Updater.
func NewUpdater(log logr.Logger, compute ComputeClient) Updater {
	return &updater{
		log:     log,
		compute: compute,
	}
}

func (u *updater) VPC(ctx context.Context, desired, current *compute.Network) (*compute.Network, error) {
	// guard against invalid updates
	if desired.AutoCreateSubnetworks != current.AutoCreateSubnetworks {
		return nil, NewInvalidUpdateError("AutoCreateSubnetworks")
	}
	// dismiss non-functional changes that may block update
	desired.Description = ""

	modified := false
	if desired.RoutingConfig != current.RoutingConfig {
		modified = true
	}

	if !modified {
		return current, nil
	}

	u.log.Info("updating VPC")
	return u.compute.PatchNetwork(ctx, current.Name, desired)
}

func (u *updater) Subnet(ctx context.Context, region string, desired, current *compute.Subnetwork) (*compute.Subnetwork, error) {
	var (
		err error
	)

	// dismiss non-functional changes that may block update
	desired.Description = ""

	if current == nil {
		return nil, fmt.Errorf("current subnet must be provided")
	}

	if current.IpCidrRange != desired.IpCidrRange {
		u.log.Info("updating subnet with expanded CIDR", "Name", current.Name)
		current, err = u.compute.ExpandSubnet(ctx, region, current.Name, desired.IpCidrRange)
		if err != nil {
			u.log.Error(err, "failed subnet CIDR update")
			return nil, err
		}
	}

	// EnableFlowLogs can't be combined with updates to the LogConfig settings. Therefore always issue a separate update when it changes before making the rest of the changes
	// if desired.EnableFlowLogs && desired.EnableFlowLogs != current.EnableFlowLogs {
	if desired.EnableFlowLogs != current.EnableFlowLogs {
		subnet := &compute.Subnetwork{
			Fingerprint:     current.Fingerprint,
			EnableFlowLogs:  desired.EnableFlowLogs,
			ForceSendFields: []string{"EnableFlowLogs"},
		}

		u.log.Info("updating subnet FlowLogs", "Name", current.Name)
		current, err = u.compute.PatchSubnet(ctx, region, current.Name, subnet)
		if err != nil {
			u.log.Error(err, "failed subnet FlowLogs update")
			return nil, err
		}
	}

	modified := false
	if desired.LogConfig != current.LogConfig {
		modified = true
		if desired.LogConfig == nil {
			desired.NullFields = []string{"LogConfig"}
		}
	}

	if !modified {
		return current, nil
	}

	// We need to update fingerprint here too in case we performed a FlowLog update already
	desired.Fingerprint = current.Fingerprint
	u.log.Info("updating subnet", "Name", current.Name)
	return u.compute.PatchSubnet(ctx, region, current.Name, desired)
}

func (u *updater) Router(ctx context.Context, region string, desired, current *compute.Router) (*compute.Router, error) {
	// While Nats can and should be updated via the router API, we want to handle Nats as a separate resource and allow
	// updates to Nats only via the specialized methods. Therefor, we will deny updates to Nats via this function call.
	desired.Nats = nil
	forceSendFields := []string{}
	nullFields := []string{}

	modified := false
	if !reflect.DeepEqual(desired.Bgp, current.Bgp) {
		modified = true
		if desired.Bgp == nil {
			nullFields = append(nullFields, "Bgp")
		}
	}
	if !reflect.DeepEqual(desired.BgpPeers, current.BgpPeers) {
		modified = true
		if len(desired.BgpPeers) == 0 {
			forceSendFields = append(forceSendFields, "BgpPeers")
		}
	}

	if modified {
		desired.ForceSendFields = forceSendFields
		desired.NullFields = nullFields
		return u.compute.PatchRouter(ctx, region, current.Name, current)
	}
	return current, nil
}

// NAT handles updates and create calls for Cloud NATs. Since NATs are not independent resources, their existence depends on the CloudRouter
// they are associated with. Therefore, we always fetch a "fresh" copy of the router before attempting to update NATs.
// NAT is an "upsert" call. If there are not any CloudNAT in the router with the specified name, we will insert the desired spec (Create).
// If there is already a NAT with the specified name, we will "replaces" the spec with the desired one.
func (u *updater) NAT(ctx context.Context, region string, router compute.Router, desired compute.RouterNat) (*compute.Router, *compute.RouterNat, error) {
	// force update certain fields
	if !desired.EnableDynamicPortAllocation {
		desired.ForceSendFields = append(desired.ForceSendFields, "EnableDynamicPortAllocation")
	}
	if desired.MinPortsPerVm == 0 {
		desired.ForceSendFields = append(desired.ForceSendFields, "MinPortsPerVM")
	}
	if desired.MaxPortsPerVm == 0 {
		desired.ForceSendFields = append(desired.ForceSendFields, "MaxPortsPerVM")
	}
	if desired.IcmpIdleTimeoutSec == 0 {
		desired.ForceSendFields = append(desired.ForceSendFields, "IcmpIdleTimeoutSec")
	}
	if desired.TcpEstablishedIdleTimeoutSec == 0 {
		desired.ForceSendFields = append(desired.ForceSendFields, "TcpEstablishedIdleTimeoutSec")
	}
	if desired.TcpTimeWaitTimeoutSec == 0 {
		desired.ForceSendFields = append(desired.ForceSendFields, "TcpTimeWaitTimeoutSec")
	}
	if desired.TcpTransitoryIdleTimeoutSec == 0 {
		desired.ForceSendFields = append(desired.ForceSendFields, "TcpTransitoryIdleTimeoutSec")
	}
	if desired.UdpIdleTimeoutSec == 0 {
		desired.ForceSendFields = append(desired.ForceSendFields, "UdpIdleTimeoutSec")
	}
	if len(desired.NatIps) == 0 {
		desired.NullFields = append(desired.NullFields, "NatIps")
	}

	index := slices.IndexFunc(router.Nats, func(nat *compute.RouterNat) bool {
		return nat.Name == desired.Name
	})

	modified := false
	// not found case
	if index < 0 {
		router.Nats = append(router.Nats, &desired)
		modified = true
	} else {
		nats := router.Nats[index]
		if !reflect.DeepEqual(nats, desired) {
			modified = true
			router.Nats[index] = &desired
		}
	}

	if !modified {
		return &router, &desired, nil
	}

	routerPatch := compute.Router{
		Nats: router.Nats,
	}
	u.log.Info("updating router with NAT", "Name", routerPatch.Name)
	result, err := u.compute.PatchRouter(ctx, region, router.Name, &routerPatch)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update CloudNAT: %s", err)
	}

	for _, nat := range result.Nats {
		if nat.Name == desired.Name {
			return result, nat, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to locate CloudNAT in router")
}

func (u *updater) DeleteNAT(ctx context.Context, region, routerId, natId string) (*compute.Router, error) {
	router, err := u.compute.GetRouter(ctx, region, routerId)
	if err != nil {
		return nil, err
	}
	if router == nil {
		return nil, nil
	}

	index := slices.IndexFunc(router.Nats, func(nat *compute.RouterNat) bool {
		if nat != nil && nat.Name == natId {
			return true
		}
		return false
	})

	if index < 0 {
		return router, nil
	}

	routerPatch := compute.Router{
		Nats: append(router.Nats[:index], router.Nats[index+1:]...),
		// in case this was the last CloudNAT we need to force send the empty array.
		ForceSendFields: append(router.ForceSendFields, "Nats"),
	}

	if _, err = u.compute.PatchRouter(ctx, region, routerId, &routerPatch); err != nil {
		return router, err
	}

	return router, nil
}

// Firewall updates a given Firewall in GCP to the desired state, making sure to only apply if there are
// applicable changes between the objects.
func (u *updater) Firewall(ctx context.Context, current, desired *compute.Firewall) (*compute.Firewall, error) {
	if shouldUpdate(current, desired) {
		fw, err := u.compute.PatchFirewallRule(ctx, desired.Name, desired)
		if err != nil {
			return nil, fmt.Errorf("failed to update firewall rule [name=%s]: %v", desired.Name, err)
		}
		return fw, nil
	}
	u.log.Info(fmt.Sprintf("no change to firewall %s rule, skipping update", desired.Name))
	return desired, nil
}

// shouldUpdate returns a boolean indicating whether there is a change between the two given rules
// that would necessitate an update
func shouldUpdate(oldRule, newRule *compute.Firewall) bool {
	if oldRule == newRule {
		return false
	}
	if oldRule == nil || newRule == nil {
		return true
	}

	// Check everything except the immutable 'Name', 'Description', 'Id', and 'SelfLink'.
	// Also CreationTimestamp is ignored.

	if !isEquivalent(oldRule.Allowed, newRule.Allowed) {
		return true
	}
	if !isEquivalent(oldRule.Denied, newRule.Denied) {
		return true
	}
	if !isEquivalent(oldRule.DestinationRanges, newRule.DestinationRanges) {
		return true
	}
	if oldRule.Direction != newRule.Direction {
		return true
	}
	if oldRule.Disabled != newRule.Disabled {
		return true
	}
	if oldRule.Kind != newRule.Kind {
		return true
	}
	if newRule.LogConfig != nil {
		if oldRule.LogConfig == nil || oldRule.LogConfig.Enable != newRule.LogConfig.Enable || oldRule.LogConfig.Metadata != newRule.LogConfig.Metadata {
			return true
		}
	}
	if oldRule.Network != newRule.Network {
		return true
	}
	if oldRule.Priority != newRule.Priority {
		return true
	}
	if !isEquivalent(oldRule.SourceRanges, newRule.SourceRanges) {
		return true
	}
	if !isEquivalent(oldRule.SourceServiceAccounts, newRule.SourceServiceAccounts) {
		return true
	}
	if !isEquivalent(oldRule.SourceTags, newRule.SourceTags) {
		return true
	}
	if !isEquivalent(oldRule.TargetServiceAccounts, newRule.TargetServiceAccounts) {
		return true
	}
	if !isEquivalent(oldRule.TargetTags, newRule.TargetTags) {
		return true
	}
	return false
}
