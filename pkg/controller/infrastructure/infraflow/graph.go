package infraflow

import (
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/flow"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
)

const (
	defaultCreateTimeout time.Duration = 5 * time.Minute
	defaultDeleteTimeout time.Duration = 5 * time.Minute
)

func (fctx *FlowContext) buildReconcileGraph() *flow.Graph {
	fctx.BasicFlowContext = shared.NewBasicFlowContext().WithSpan().WithLogger(fctx.log).WithPersist(fctx.persistState)
	g := flow.NewGraph("infrastructure reconciliation")

	fctx.AddTask(g, "ensure service account", fctx.ensureServiceAccount,
		shared.Timeout(defaultCreateTimeout),
		shared.DoIf(
			!features.ExtensionFeatureGate.Enabled(features.DisableGardenerServiceAccountCreation) || fctx.whiteboard.Get(CreatedServiceAccountKey) != nil,
		),
	)
	ensureVPC := fctx.AddTask(g, "ensure VPC", fctx.ensureVPC,
		shared.Timeout(defaultCreateTimeout),
	)
	ensureDualStackKubernetesRoutesCleanup := fctx.AddTask(g, "ensure kubernetes routes cleanup", fctx.ensureKubernetesRoutesCleanup,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
		shared.DoIf(isToDualStackMigration(fctx.shoot)),
	)
	ensureNodesSubnet := fctx.AddTask(g, "ensure worker subnet", fctx.ensureNodesSubnet,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC, ensureDualStackKubernetesRoutesCleanup),
	)
	fctx.AddTask(g, "ensure alias ip ranges", fctx.ensureAliasIpRanges,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC, ensureDualStackKubernetesRoutesCleanup, ensureNodesSubnet),
	)
	ensureInternalSubnet := fctx.AddTask(g, "ensure internal subnet", fctx.ensureInternalSubnet,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureServicesSubnet := fctx.AddTask(g, "ensure IPv6 services subnet", fctx.ensureServicesSubnet,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
		shared.DoIf(!gardencorev1beta1.IsIPv4SingleStack(fctx.networking.IPFamilies)),
	)
	ensureIPv6Services := fctx.AddTask(g, "ensure IPv6 CIDR services", fctx.ensureIPv6CIDRs,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureNodesSubnet, ensureServicesSubnet),
		shared.DoIf(!gardencorev1beta1.IsIPv4SingleStack(fctx.networking.IPFamilies)),
	)
	ensureRouter := fctx.AddTask(g, "ensure router", fctx.ensureCloudRouter,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC),
	)
	ensureIpAddresses := fctx.AddTask(g, "ensure IP addresses", fctx.ensureAddresses,
		shared.Timeout(defaultCreateTimeout),
		shared.DoIf(fctx.config.Networks.CloudNAT != nil && len(fctx.config.Networks.CloudNAT.NatIPNames) > 0),
	)
	fctx.AddTask(g, "ensure nats", fctx.ensureCloudNAT,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureRouter, ensureNodesSubnet, ensureIpAddresses),
	)
	fctx.AddTask(g, "ensure firewall", fctx.ensureFirewallRules,
		shared.Timeout(defaultCreateTimeout),
		shared.Dependencies(ensureVPC, ensureNodesSubnet, ensureInternalSubnet, ensureIPv6Services),
	)

	return g
}

func (fctx *FlowContext) buildDeleteGraph() *flow.Graph {
	fctx.BasicFlowContext = shared.NewBasicFlowContext().WithLogger(fctx.log).WithSpan()
	g := flow.NewGraph("infrastructure deletion")

	fctx.AddTask(g, "destroy service account", fctx.ensureServiceAccountDeleted,
		shared.Timeout(defaultDeleteTimeout), shared.DoIf(fctx.whiteboard.Get(CreatedServiceAccountKey) != nil),
	)
	fctx.AddTask(g, "destroy kubernetes routes", fctx.ensureKubernetesRoutesDeleted, shared.Timeout(defaultDeleteTimeout))
	ensureFirewallDeleted := fctx.AddTask(g, "destroy infrastructure firewall", fctx.ensureFirewallRulesDeleted, shared.Timeout(20*time.Minute))
	ensureNatDeleted := fctx.AddTask(g, "destroy nats", fctx.ensureCloudNATDeleted,
		shared.Timeout(defaultDeleteTimeout),
		// we do not need to clean up CloudNAT for managed CloudRouters because it will be deleted with the router deletion.
		shared.DoIf(isUserRouter(fctx.config)),
	)
	ensureInternalSubnetDeleted := fctx.AddTask(g,
		"destroy internal subnet",
		fctx.ensureSubnetDeletedFactory(fctx.internalSubnetNameFromConfig(), ObjectKeyInternalSubnet),
		shared.Timeout(defaultDeleteTimeout),
	)
	ensureServicesSubnetDeleted := fctx.AddTask(g,
		"destroy services subnet",
		fctx.ensureSubnetDeletedFactory(fctx.servicesSubnetNameFromConfig(), ObjectKeyServicesSubnet),
		shared.Timeout(defaultDeleteTimeout),
		shared.DoIf(!gardencorev1beta1.IsIPv4SingleStack(fctx.networking.IPFamilies)),
	)
	ensureCloudRouterDeleted := fctx.AddTask(g, "ensure router deleted", fctx.ensureCloudRouterDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureNatDeleted),
		// for user-managed CloudRouters, skip deletion.
		shared.DoIf(!isUserRouter(fctx.config)),
	)
	ensureSubnetDeleted := fctx.AddTask(g,
		"destroy worker subnet",
		fctx.ensureSubnetDeletedFactory(fctx.subnetNameFromConfig(), ObjectKeyNodeSubnet),
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(ensureCloudRouterDeleted),
	)
	fctx.AddTask(g,
		"destroy vpc",
		fctx.ensureVPCDeleted,
		shared.Timeout(defaultDeleteTimeout),
		shared.Dependencies(
			ensureSubnetDeleted,
			ensureInternalSubnetDeleted,
			ensureServicesSubnetDeleted,
			ensureCloudRouterDeleted,
			ensureFirewallDeleted,
		),
		shared.DoIf(!isUserVPC(fctx.config)),
	)

	return g
}
