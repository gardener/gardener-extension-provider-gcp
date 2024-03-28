package infraflow

import (
	"time"

	"github.com/gardener/gardener/pkg/utils/flow"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
)

const (
	defaultCreateTimeout time.Duration = 5 * time.Minute
	defaultDeleteTimeout time.Duration = 5 * time.Minute
)

func (c *FlowContext) buildReconcileGraph() *flow.Graph {
	fctx := shared.NewBasicFlowContext().WithSpan().WithLogger(c.log).WithPersist(c.persistState)
	g := flow.NewGraph("infrastructure reconciliation")

	fctx.AddTask(g, "ensure service account", c.ensureServiceAccount,
		shared.Timeout(defaultCreateTimeout),
		shared.DoIf(!features.ExtensionFeatureGate.Enabled(features.DisableGardenerServiceAccountCreation)),
	)
	ensureVPC := fctx.AddTask(g, "ensure VPC", c.ensureVPC,
		shared.Timeout(defaultCreateTimeout),
	)
	ensureSubnet := fctx.AddTask(g, "ensure worker subnet", c.ensureSubnet,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureInternalSubnet := fctx.AddTask(g, "ensure internal subnet", c.ensureInternalSubnet,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureRouter := fctx.AddTask(g, "ensure router", c.ensureCloudRouter,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureIpAddresses := fctx.AddTask(g, "ensure IP addresses", c.ensureAddresses,
		shared.Timeout(defaultCreateTimeout),
		shared.DoIf(c.config.Networks.CloudNAT != nil && len(c.config.Networks.CloudNAT.NatIPNames) > 0),
	)
	fctx.AddTask(g, "ensure nats", c.ensureCloudNAT,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureRouter, ensureSubnet, ensureIpAddresses))

	fctx.AddTask(g, "ensure firewall", c.ensureFirewallRules,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC, ensureSubnet, ensureInternalSubnet),
	)

	return g
}

func (c *FlowContext) buildDeleteGraph() *flow.Graph {
	fctx := shared.NewBasicFlowContext().WithLogger(c.log).WithSpan()
	g := flow.NewGraph("infrastructure deletion")

	fctx.AddTask(g, "destroy service account", c.ensureServiceAccountDeleted,
		shared.Timeout(defaultDeleteTimeout),
	)
	fctx.AddTask(g, "destroy kubernetes routes", c.ensureKubernetesRoutesDeleted, shared.Timeout(defaultDeleteTimeout))
	ensureFirewallDeleted := fctx.AddTask(g, "destroy infrastructure firewall", c.ensureFirewallRulesDeleted, shared.Timeout(20*time.Minute))
	ensureNatDeleted := fctx.AddTask(g, "destroy nats", c.ensureCloudNATDeleted,
		shared.Timeout(defaultDeleteTimeout),
		// we do not need to clean up CloudNAT for managed CloudRouters because it will be deleted with the router deletion.
		shared.DoIf(isUserRouter(c.config)),
	)
	ensureInternalSubnetDeleted := fctx.AddTask(g, "destroy internal subnet", c.ensureInternalSubnetDeleted,
		shared.Timeout(defaultDeleteTimeout),
	)
	ensureCloudRouterDeleted := fctx.AddTask(g, "ensure router deleted", c.ensureCloudRouterDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureNatDeleted),
		// for user-managed CloudRouters, skip deletion.
		shared.DoIf(!isUserRouter(c.config)),
	)
	ensureSubnetDeleted := fctx.AddTask(g, "destroy worker subnet", c.ensureSubnetDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureCloudRouterDeleted),
	)
	fctx.AddTask(g, "destroy vpc", c.ensureVPCDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureSubnetDeleted, ensureInternalSubnetDeleted, ensureCloudRouterDeleted, ensureFirewallDeleted),
		shared.DoIf(!isUserVPC(c.config)),
	)

	return g
}
