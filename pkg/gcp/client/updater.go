package client

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
)

var _ Updater = &updater{}

// Updater can perform operations to update infrastructure resources.
type Updater interface {
	// VPC updates the respective infrastructure object according to desired state.
	VPC(ctx context.Context, desired, current *Network) (*Network, error)
	// Subnet updates the respective infrastructure object according to desired state.
	Subnet(ctx context.Context, region string, desired, current *Subnetwork) (*Subnetwork, error)
	// Router updates the respective infrastructure object according to desired state.
	Router(ctx context.Context, region string, desired, current *Router) (*Router, error)
	// NAT updates the respective infrastructure object according to desired state.
	NAT(ctx context.Context, region string, router *Router, desired *RouterNat) (*Router, *RouterNat, error)
	// DeleteNAT deletes the NAT gateway from the respective router.
	// Since NAT Gateways are a property of Cloud Routers, DeleteNAT is a special update function for the CloudRouter that
	// will remove the NAT with specified name.
	DeleteNAT(ctx context.Context, region, router, nat string) (*Router, error)
	// Firewall updates the respective infrastructure object according to desired state.
	Firewall(ctx context.Context, firewall *Firewall) (*Firewall, error)
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

func (u *updater) VPC(ctx context.Context, desired, current *Network) (*Network, error) {
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

func (u *updater) Subnet(ctx context.Context, region string, desired, current *Subnetwork) (*Subnetwork, error) {
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
		subnet := &Subnetwork{
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

func (u *updater) Router(ctx context.Context, region string, desired, current *Router) (*Router, error) {
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
func (u *updater) NAT(ctx context.Context, region string, router *Router, desired *RouterNat) (*Router, *RouterNat, error) {
	if router == nil {
		err := fmt.Errorf("router cannot be nil")
		u.log.Error(err, "failed to update NAT")
		return nil, nil, err
	}
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

	index := slices.IndexFunc(router.Nats, func(nat *RouterNat) bool {
		return nat.Name == desired.Name
	})

	modified := false
	// not found case
	if index < 0 {
		router.Nats = append(router.Nats, desired)
		modified = true
	} else {
		nats := router.Nats[index]
		if !reflect.DeepEqual(nats, desired) {
			modified = true
			router.Nats[index] = desired
		}
	}

	if !modified {
		return router, desired, nil
	}

	u.log.Info("updating router with NAT", "Name", router.Name)
	router, err := u.compute.PatchRouter(ctx, region, router.Name, router)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update CloudNAT: %s", err)
	}

	for _, nat := range router.Nats {
		if nat.Name == desired.Name {
			return router, nat, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to locate CloudNAT in router")
}

func (u *updater) DeleteNAT(ctx context.Context, region, routerId, natId string) (*Router, error) {
	router, err := u.compute.GetRouter(ctx, region, routerId)
	if err != nil {
		return nil, err
	}
	if router == nil {
		return nil, nil
	}

	index := slices.IndexFunc(router.Nats, func(nat *RouterNat) bool {
		if nat != nil && nat.Name == natId {
			return true
		}
		return false
	})

	if index < 0 {
		return router, nil
	}

	router.Nats = append(router.Nats[:index], router.Nats[index+1:]...)
	// in case this was the last CloudNAT we need to force send the empty array.
	router.ForceSendFields = append(router.ForceSendFields, "Nats")

	if _, err = u.compute.PatchRouter(ctx, region, routerId, router); err != nil {
		return router, err
	}

	return router, nil
}

func (u *updater) Firewall(ctx context.Context, firewall *Firewall) (*Firewall, error) {
	fw, err := u.compute.PatchFirewallRule(ctx, firewall.Name, firewall)
	if err != nil {
		return nil, err
	}

	return fw, nil
}
