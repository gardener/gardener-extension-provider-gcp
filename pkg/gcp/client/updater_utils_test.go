package client

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/compute/v1"
)

var _ = Describe("Firewall Rules", func() {
	Context("Equivalence function", func() {
		It("Should not consider the order of elements", func() {
			s1 := []int{1, 2, 3}
			s2 := []int{2, 4, 6}
			s3 := []int{3, 1, 2}

			Expect(isEquivalent(s1, s2)).To(BeFalse())
			Expect(isEquivalent(s1, s3)).To(BeTrue())
		})
	})

	var baseRule, newRule *compute.Firewall

	Context("Equivalence function", func() {
		BeforeEach(func() {
			baseRule = setupFirewall()
			newRule = setupFirewall()
		})

		It("Should handle identical inputs", func() {
			Expect(shouldPatchFirewallRule(baseRule, baseRule)).To(BeFalse())
		})

		It("Should handle nil-pointer correctly", func() {
			Expect(shouldPatchFirewallRule(nil, nil)).To(BeFalse())
			Expect(shouldPatchFirewallRule(baseRule, nil)).To(BeTrue())
			Expect(shouldPatchFirewallRule(nil, baseRule)).To(BeTrue())
		})

		// Test example cases for FirewallRules - a slice, bool, string, and int
		It("Should detect changes to 'Allowed'", func() {
			// A little bit of paranoia: Test order-invariance once on the actual rule
			newRule.Allowed[0], newRule.Allowed[1] = newRule.Allowed[1], newRule.Allowed[0]
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeFalse())

			newRule.Allowed = []*compute.FirewallAllowed{
				{
					IPProtocol: "Baz",
					Ports:      []string{"1", "2", "3"},
				},
				{
					IPProtocol: "Foobar",
					Ports:      []string{"7", "8", "9"},
				},
				{
					IPProtocol: "Barfoo",
					Ports:      []string{"2", "4", "6"},
				},
			}
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeTrue())
		})

		It("Should detect changes to 'Direction'", func() {
			newRule.Direction = "EGRESS"
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeTrue())
		})

		It("Should detect changes to 'Disabled'", func() {
			newRule.Disabled = true
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeTrue())
		})

		It("Should detect changes to 'Priority'", func() {
			newRule.Priority = 56565
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeTrue())
		})

		It("Should ignore changes to immutable fields", func() {
			newRule.Name = "Foobar"
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeFalse())

			newRule.Description = "Foobar"
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeFalse())

			newRule.SelfLink = "Foobar"
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeFalse())

			newRule.Id = 7
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeFalse())

			newRule.Kind = "Foobar"
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeFalse())

			// Also check that irrelevant fields are ignored
			newRule.CreationTimestamp = "Foobar"
			Expect(shouldPatchFirewallRule(baseRule, newRule)).To(BeFalse())
		})
	})
})

func setupFirewall() *compute.Firewall {
	return &compute.Firewall{
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: "Foo",
				Ports:      []string{"1", "2", "3"},
			},
			{
				IPProtocol: "Bar",
				Ports:      []string{"2", "3", "4"},
			},
		},
		Denied:                nil,
		Description:           "TestFirewall",
		Direction:             "INGRESS",
		Disabled:              false,
		Id:                    42,
		Kind:                  "compute#firewall",
		Name:                  "Foo",
		Network:               "foo/bar/baz",
		Priority:              31337,
		LogConfig:             &compute.FirewallLogConfig{Enable: false, Metadata: "INCLUDE_ALL_METADATA"},
		SourceRanges:          []string{"0.0.0.0/0"},
		SourceTags:            []string{"foobar"},
		DestinationRanges:     []string{"192.168.100/24"},
		TargetTags:            []string{"barfoo", "barbaz"},
		SourceServiceAccounts: []string{"robot1"},
		TargetServiceAccounts: []string{"robot2"},
	}
}
