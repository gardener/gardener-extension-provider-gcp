// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"strconv"

	"google.golang.org/api/compute/v1"
)

// IngressAllowSSH ingress rule to allow ssh access
func IngressAllowSSH(opt *Options, cidr []string) *compute.Firewall {
	return &compute.Firewall{
		Allowed:      []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{strconv.Itoa(SSHPort)}}},
		Description:  "SSH access for Bastion",
		Direction:    "INGRESS",
		TargetTags:   []string{opt.BastionInstanceName},
		Name:         FirewallIngressAllowSSHResourceName(opt.BastionInstanceName),
		Network:      opt.Network,
		SourceRanges: cidr,
		Priority:     50,
	}
}

// EgressDenyAll egress rule to deny all
func EgressDenyAll(opt *Options) *compute.Firewall {
	return &compute.Firewall{
		Denied:            []*compute.FirewallDenied{{IPProtocol: "all"}},
		Description:       "Bastion egress deny",
		Direction:         "EGRESS",
		TargetTags:        []string{opt.BastionInstanceName},
		Name:              FirewallEgressDenyAllResourceName(opt.BastionInstanceName),
		Network:           opt.Network,
		DestinationRanges: []string{"0.0.0.0/0"},
		Priority:          1000,
	}
}

// EgressAllowOnly egress rule to allow ssh traffic to workers cidr range.
func EgressAllowOnly(opt *Options) *compute.Firewall {
	return &compute.Firewall{
		Allowed:           []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{strconv.Itoa(SSHPort)}}},
		Description:       "Allow Bastion egress to Shoot workers",
		Direction:         "EGRESS",
		TargetTags:        []string{opt.BastionInstanceName},
		Name:              FirewallEgressAllowOnlyResourceName(opt.BastionInstanceName),
		Network:           opt.Network,
		DestinationRanges: []string{opt.WorkersCIDR},
		Priority:          60,
	}
}

// patchCIDRs use for patchFirewallRule to patch the firewall rule
func patchCIDRs(cidrs []string) *compute.Firewall {
	return &compute.Firewall{SourceRanges: cidrs}
}
