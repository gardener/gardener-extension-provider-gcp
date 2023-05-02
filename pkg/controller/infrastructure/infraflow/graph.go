package infraflow

import (
	"time"

	"github.com/gardener/gardener/pkg/utils/flow"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
)

const (
	defaultCreateTimeout time.Duration = 5 * time.Minute
	defaultDeleteTimeout time.Duration = 5 * time.Minute
)

func (c *FlowReconciler) buildReconcileGraph() *flow.Graph {
	g := flow.NewGraph("infrastructure reconciliation")

	c.AddTask(g, "ensure service account", c.ensureServiceAccount,
		shared.Timeout(defaultCreateTimeout),
	)
	ensureVPC := c.AddTask(g, "ensure VPC", c.ensureVPC,
		shared.Timeout(defaultCreateTimeout),
	)
	ensureSubnet := c.AddTask(g, "ensure worker subnet", c.ensureSubnet,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureInternalSubnet := c.AddTask(g, "ensure internal subnet", c.ensureInternalSubnet,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureRouter := c.AddTask(g, "ensure router", c.ensureCloudRouter,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureIpAddresses := c.AddTask(g, "ensure IP addresses", c.ensureAddresses,
		shared.Timeout(defaultCreateTimeout),
		shared.DoIf(c.config.Networks.CloudNAT != nil && len(c.config.Networks.CloudNAT.NatIPNames) > 0),
	)
	c.AddTask(g, "ensure nats", c.ensureCloudNAT,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureRouter, ensureSubnet, ensureIpAddresses))

	c.AddTask(g, "ensure firewall", c.ensureFirewallRules,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC, ensureSubnet, ensureInternalSubnet),
	)

	return g
}

func (c *FlowReconciler) buildDeleteGraph() *flow.Graph {
	g := flow.NewGraph("infrastructure deletion")

	c.AddTask(g, "destroy service account", c.ensureServiceAccountDeleted,
		shared.Timeout(defaultDeleteTimeout),
	)
	c.AddTask(g, "destroy kubernetes routes", c.ensureKubernetesRoutesDeleted, shared.Timeout(defaultDeleteTimeout))
	ensureFirewallDeleted := c.AddTask(g, "destroy infrastructure firewall", c.ensureFirewallRulesDeleted, shared.Timeout(defaultDeleteTimeout))
	ensureNatDeleted := c.AddTask(g, "destroy nats", c.ensureCloudNATDeleted,
		shared.Timeout(defaultDeleteTimeout),
		// we do not need to clean up CloudNAT for managed CloudRouters because it will be deleted with the router deletion.
		shared.DoIf(isUserRouter(c.config)),
	)
	ensureInternalSubnetDeleted := c.AddTask(g, "destroy internal subnet", c.ensureInternalSubnetDeleted,
		shared.Timeout(defaultDeleteTimeout),
	)
	ensureCloudRouterDeleted := c.AddTask(g, "ensure router deleted", c.ensureCloudRouterDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureNatDeleted),
		// for user-managed CloudRouters, skip deletion.
		shared.DoIf(!isUserRouter(c.config)),
	)
	ensureSubnetDeleted := c.AddTask(g, "destroy worker subnet", c.ensureSubnetDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureCloudRouterDeleted),
	)
	c.AddTask(g, "destroy vpc", c.ensureVPCDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureSubnetDeleted, ensureInternalSubnetDeleted, ensureCloudRouterDeleted, ensureFirewallDeleted),
		shared.DoIf(!isUserVPC(c.config)),
	)

	return g
}
