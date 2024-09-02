package infraflow

import (
	"testing"

	"github.com/onsi/gomega"
	"google.golang.org/api/compute/v1"
)

func TestSimilarityFunc(t *testing.T) {
	g := gomega.NewWithT(t)

	// Order of top level elements should not matter
	s1 := []int{1, 2, 3}
	s2 := []int{2, 4, 6}
	s3 := []int{3, 1, 2}

	g.Expect(isSimilar(s1, s2)).To(gomega.BeFalse())
	g.Expect(isSimilar(s1, s3)).To(gomega.BeTrue())

	// Order of deeper elements, however, does matter
	s4 := [][]int{{1}, {2, 3}, {4}}
	s5 := [][]int{{1}, {3, 2}, {4}}
	s6 := [][]int{{1}, {4}, {2, 3}}

	g.Expect(isSimilar(s4, s5)).To(gomega.BeFalse())
	g.Expect(isSimilar(s4, s6)).To(gomega.BeTrue())

}

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

func TestIdentity(t *testing.T) {
	g := gomega.NewWithT(t)
	firewall := setupFirewall()

	update, err := firewallUpdate(firewall, firewall)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	if update != nil {
		t.Errorf("Error constructing diff: Got %v, wanted %v", update, nil)
	}
}

func TestIdenticalFirewalls(t *testing.T) {
	g := gomega.NewWithT(t)
	old_rule := setupFirewall()
	new_rule := setupFirewall()

	update, err := firewallUpdate(old_rule, new_rule)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(update).To(gomega.BeNil())
}

func TestEmptyFirewall(t *testing.T) {
	g := gomega.NewWithT(t)
	old_rule := setupFirewall()
	new_rule := &compute.Firewall{}

	update_rule, err := firewallUpdate(old_rule, new_rule)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(update_rule).To(gomega.BeNil())
}

func TestNewFirewallNil(t *testing.T) {
	g := gomega.NewWithT(t)
	old_rule := setupFirewall()
	var new_rule *compute.Firewall

	update_rule, err := firewallUpdate(old_rule, new_rule)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(update_rule).To(gomega.BeNil())
}

func TestCompletelyNewFirewall(t *testing.T) {
	g := gomega.NewWithT(t)
	old_rule := &compute.Firewall{}
	new_rule := setupFirewall()

	update_rule, err := firewallUpdate(old_rule, new_rule)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())
	g.Expect(new_rule).To(gomega.BeEquivalentTo(update_rule))
}

func TestUpdateFunc(t *testing.T) {
	g := gomega.NewWithT(t)
	old_rule := setupFirewall()
	new_rule := setupFirewall()

	testAllowed := []*compute.FirewallAllowed{
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
	testName := "Bar"

	new_rule.Allowed = testAllowed
	new_rule.Name = testName

	update_rule, err := firewallUpdate(old_rule, new_rule)
	g.Expect(err).ShouldNot(gomega.HaveOccurred())

	// The two changed values should be changed
	g.Expect(update_rule.Allowed).To(gomega.BeEquivalentTo(testAllowed))
	g.Expect(update_rule.Name).To(gomega.BeEquivalentTo(testName))

	// Every value not changed should be left zero
	g.Expect(update_rule.Denied).To(gomega.BeZero())
	g.Expect(update_rule.Description).To(gomega.BeZero())
	g.Expect(update_rule.DestinationRanges).To(gomega.BeZero())
	g.Expect(update_rule.Direction).To(gomega.BeZero())
	g.Expect(update_rule.Disabled).To(gomega.BeZero())
	g.Expect(update_rule.Id).To(gomega.BeZero())
	g.Expect(update_rule.Kind).To(gomega.BeZero())
	g.Expect(update_rule.LogConfig).To(gomega.BeZero())
	g.Expect(update_rule.Network).To(gomega.BeZero())
	g.Expect(update_rule.Priority).To(gomega.BeZero())
	g.Expect(update_rule.SelfLink).To(gomega.BeZero())
	g.Expect(update_rule.SourceRanges).To(gomega.BeZero())
	g.Expect(update_rule.SourceServiceAccounts).To(gomega.BeZero())
	g.Expect(update_rule.SourceTags).To(gomega.BeZero())
	g.Expect(update_rule.TargetServiceAccounts).To(gomega.BeZero())
	g.Expect(update_rule.TargetTags).To(gomega.BeZero())
}
