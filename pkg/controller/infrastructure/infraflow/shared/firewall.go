// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import "fmt"

// FirewallRuleAllowInternalName generate the name for firewall rule to allow internal access for IPv4
func FirewallRuleAllowInternalName(base string) string {
	return fmt.Sprintf("%s-allow-internal-access", base)
}

// FirewallRuleAllowInternalNameIPv6 generate the name for firewall rule to allow  internal access for IPv6
func FirewallRuleAllowInternalNameIPv6(base string) string {
	return fmt.Sprintf("%s-allow-internal-access-ipv6", base)
}

// FirewallRuleAllowExternalName generate the name for firewall rule to allow external access
func FirewallRuleAllowExternalName(base string) string {
	return fmt.Sprintf("%s-allow-external-access", base)
}

// FirewallRuleAllowHealthChecksName generate the name for firewall rule to allow healthchecks from IPv4 loadbalancers
func FirewallRuleAllowHealthChecksName(base string) string {
	return fmt.Sprintf("%s-allow-health-checks", base)
}

// FirewallRuleAllowHealthChecksNameIPv6 generate the name for firewall rule to allow healthchecks from IPv6 loadbalancers
func FirewallRuleAllowHealthChecksNameIPv6(base string) string {
	return fmt.Sprintf("%s-allow-health-checks-ipv6", base)
}
