package infraflow

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

func (fctx *FlowContext) ensureServiceAccount(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	serviceAccountName := fctx.serviceAccountNameFromConfig()
	sa, err := fctx.iamClient.GetServiceAccount(ctx, serviceAccountName)
	if err != nil {
		return err
	}

	if sa == nil {
		sa, err = fctx.iamClient.CreateServiceAccount(ctx, serviceAccountName)
		if err != nil {
			log.Error(err, "failed to create service account", "name", serviceAccountName)
			return err
		}
		fctx.whiteboard.GetChild(ChildKeyIDs).Set(KeyServiceAccountEmail, sa.Email)
		fctx.whiteboard.Set(CreatedServiceAccountKey, "true")
		fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
	}

	return nil
}

func (fctx *FlowContext) ensureVPC(ctx context.Context) error {
	var (
		err error
	)

	if fctx.config.Networks.VPC != nil {
		return fctx.ensureUserManagedVPC(ctx)
	}

	vpcName := fctx.vpcNameFromConfig()

	current, err := fctx.computeClient.GetNetwork(ctx, vpcName)
	if err != nil {
		return err
	}

	targetVPC := targetNetwork(vpcName)
	if current == nil {
		current, err = fctx.computeClient.InsertNetwork(ctx, targetVPC)
		if err != nil {
			return err
		}
	} else {
		current, err = fctx.updater.VPC(ctx, targetVPC, current)
		if err != nil {
			return err
		}
	}

	fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
	fctx.whiteboard.SetObject(ObjectKeyVPC, current)
	return nil
}

func (fctx *FlowContext) ensureUserManagedVPC(ctx context.Context) error {
	var (
		log     = shared.LogFromContext(ctx)
		vpcSpec = fctx.config.Networks.VPC
		vpcName = vpcSpec.Name
		err     error
	)

	vpc, err := fctx.computeClient.GetNetwork(ctx, vpcName)
	if err != nil {
		return err
	}
	if vpc == nil {
		log.Error(nil, fmt.Sprintf("failed to locate user-managed VPC [Name=%s]", vpcName))
		return fmt.Errorf("failed to locate user-managed VPC [Name=%s]", vpcName)
	}

	fctx.whiteboard.SetObject(ObjectKeyVPC, vpc)
	return nil
}

func (fctx *FlowContext) ensureSubnet(ctx context.Context) error {
	var (
		region = fctx.infra.Spec.Region
	)

	if err := fctx.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)

	subnetName := fctx.subnetNameFromConfig()
	cidr := fctx.config.Networks.Workers
	if len(cidr) == 0 {
		cidr = fctx.config.Networks.Worker
	}

	targetSubnet := targetSubnetState(
		subnetName,
		"gardener-managed worker subnet",
		cidr,
		vpc.SelfLink,
		fctx.config.Networks.FlowLogs,
		fctx.config.Networks.DualStack,
	)

	subnet, err := fctx.computeClient.GetSubnet(ctx, region, subnetName)
	if err != nil {
		return err
	}

	if subnet == nil {
		subnet, err = fctx.computeClient.InsertSubnet(ctx, region, targetSubnet)
		if err != nil {
			return err
		}
	} else {
		subnet, err = fctx.updater.Subnet(ctx, fctx.infra.Spec.Region, targetSubnet, subnet)
		if err != nil {
			return err
		}
	}

	fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
	fctx.whiteboard.SetObject(ObjectKeyNodeSubnet, subnet)
	return nil
}

func (fctx *FlowContext) ensureInternalSubnet(ctx context.Context) error {
	var (
		region = fctx.infra.Spec.Region
	)

	if fctx.config.Networks.Internal == nil {
		return fctx.ensureInternalSubnetDeleted(ctx)
	}

	if err := fctx.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)

	subnetName := fctx.internalSubnetNameFromConfig()

	subnet, err := fctx.computeClient.GetSubnet(ctx, region, subnetName)
	if err != nil {
		return err
	}

	desired := targetSubnetState(
		subnetName,
		"gardener-managed internal subnet",
		*fctx.config.Networks.Internal,
		vpc.SelfLink,
		nil,
		fctx.config.Networks.DualStack,
	)
	if subnet == nil {
		subnet, err = fctx.computeClient.InsertSubnet(ctx, region, desired)
		if err != nil {
			return err
		}
	} else {
		subnet, err = fctx.updater.Subnet(ctx, fctx.infra.Spec.Region, desired, subnet)
		if err != nil {
			return err
		}
	}

	fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
	fctx.whiteboard.SetObject(ObjectKeyInternalSubnet, subnet)
	return nil
}

func (fctx *FlowContext) ensureCloudRouter(ctx context.Context) error {
	if fctx.config.Networks.VPC != nil && fctx.config.Networks.VPC.CloudRouter != nil {
		return fctx.ensureUserManagedCloudRouter(ctx)
	}

	log := shared.LogFromContext(ctx)

	if err := fctx.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)

	routerName := fctx.cloudRouterNameFromConfig()

	desired := targetRouterState(routerName, "gardener-managed router", vpc.SelfLink)
	router, err := fctx.computeClient.GetRouter(ctx, fctx.infra.Spec.Region, routerName)
	if err != nil {
		return err
	}

	if router == nil {
		log.Info("creating...")
		if router, err = fctx.computeClient.InsertRouter(ctx, fctx.infra.Spec.Region, desired); err != nil {
			return err
		}
	} else {
		if router, err = fctx.updater.Router(ctx, fctx.infra.Spec.Region, desired, router); err != nil {
			return err
		}
	}

	fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
	fctx.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (fctx *FlowContext) ensureUserManagedCloudRouter(ctx context.Context) error {
	var (
		log        = shared.LogFromContext(ctx)
		routerName = fctx.config.Networks.VPC.CloudRouter.Name
	)

	log.Info("ensuring user-managed router")

	router, err := fctx.computeClient.GetRouter(ctx, fctx.infra.Spec.Region, routerName)
	if err != nil {
		return err
	}
	if router == nil {
		return fmt.Errorf("failed to locate user-managed CloudRouter [Name=%s]: %v", routerName, err)
	}

	fctx.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (fctx *FlowContext) ensureAddresses(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	if fctx.config.Networks.CloudNAT == nil || len(fctx.config.Networks.CloudNAT.NatIPNames) == 0 {
		return nil
	}

	var addresses []*compute.Address
	for _, name := range fctx.config.Networks.CloudNAT.NatIPNames {
		ip, err := fctx.computeClient.GetAddress(ctx, fctx.infra.Spec.Region, name.Name)
		if err != nil {
			log.Error(err, "failed to locate user-managed IP address")
			return err
		}
		addresses = append(addresses, ip)
	}

	if len(addresses) > 0 {
		fctx.whiteboard.SetObject(ObjectKeyIPAddresses, addresses)
	}
	return nil
}

func (fctx *FlowContext) ensureCloudNAT(ctx context.Context) error {
	var err error
	if err := fctx.ensureObjectKeys(ObjectKeyRouter, ObjectKeyNodeSubnet); err != nil {
		return err
	}

	subnet := GetObject[*compute.Subnetwork](fctx.whiteboard, ObjectKeyNodeSubnet)
	router := GetObject[*compute.Router](fctx.whiteboard, ObjectKeyRouter)

	natName := fctx.cloudNatNameFromConfig()
	var (
		nat       *compute.RouterNat
		addresses []*compute.Address
	)

	if a := fctx.whiteboard.GetObject(ObjectKeyIPAddresses); a != nil {
		addresses = a.([]*compute.Address)
	}

	targetNat := targetNATState(natName, subnet.SelfLink, fctx.config.Networks.CloudNAT, addresses)
	router, nat, err = fctx.updater.NAT(ctx, fctx.infra.Spec.Region, *router, *targetNat)
	if err != nil {
		return err
	}

	fctx.whiteboard.SetObject(ObjectKeyRouter, router)
	fctx.whiteboard.SetObject(ObjectKeyNAT, nat)
	fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
	return nil
}

func (fctx *FlowContext) ensureFirewallRules(ctx context.Context) error {
	if err := fctx.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)

	cidrs := []*string{fctx.podCIDR, fctx.config.Networks.Internal, ptr.To(fctx.config.Networks.Workers), ptr.To(fctx.config.Networks.Worker)}
	rules := []*compute.Firewall{
		firewallRuleAllowInternal(firewallRuleAllowInternalName(fctx.clusterName), vpc.SelfLink, cidrs),
		firewallRuleAllowHealthChecks(firewallRuleAllowHealthChecksName(fctx.clusterName), vpc.SelfLink),
	}
	for _, rule := range rules {
		gcprule, err := fctx.computeClient.GetFirewallRule(ctx, rule.Name)
		if err != nil {
			return fmt.Errorf("failed to ensure firewall rule [name=%s]: %v", rule.Name, err)
		}
		if gcprule == nil {
			_, err := fctx.computeClient.InsertFirewallRule(ctx, rule)
			if err != nil {
				return fmt.Errorf("failed to create firewall rule [name=%s]: %v", rule.Name, err)
			}
		} else {
			if _, err = fctx.updater.Firewall(ctx, rule, gcprule); err != nil {
				return err
			}
		}
	}

	// delete unnecessary firewall rule.
	return fctx.computeClient.DeleteFirewallRule(ctx, firewallRuleAllowExternalName(fctx.clusterName))
}

func (fctx *FlowContext) ensureVPCDeleted(ctx context.Context) error {
	networkName := fctx.vpcNameFromConfig()
	err := fctx.computeClient.DeleteNetwork(ctx, networkName)
	if err != nil {
		return err
	}

	fctx.whiteboard.DeleteObject(ObjectKeyVPC)
	return nil
}

func (fctx *FlowContext) ensureSubnetDeleted(ctx context.Context) error {
	subnetName := fctx.subnetNameFromConfig()

	err := fctx.computeClient.DeleteSubnet(ctx, fctx.infra.Spec.Region, subnetName)
	if err != nil {
		return err
	}

	fctx.whiteboard.DeleteObject(ObjectKeyNodeSubnet)
	return nil
}

func (fctx *FlowContext) ensureInternalSubnetDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	subnetName := fctx.internalSubnetNameFromConfig()
	log.Info("deleting internal subnet")
	err := fctx.computeClient.DeleteSubnet(ctx, fctx.infra.Spec.Region, subnetName)
	if err != nil {
		return err
	}

	fctx.whiteboard.DeleteObject(ObjectKeyInternalSubnet)
	return nil
}

func (fctx *FlowContext) ensureServiceAccountDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	serviceAccountName := fctx.serviceAccountNameFromConfig()

	log.Info("deleting service account")
	if err := fctx.iamClient.DeleteServiceAccount(ctx, serviceAccountName); err != nil {
		return err
	}

	fctx.whiteboard.GetChild(ChildKeyIDs).Delete(KeyServiceAccountEmail)
	return nil
}

func (fctx *FlowContext) ensureCloudRouterDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	if fctx.config.Networks.VPC != nil && fctx.config.Networks.VPC.CloudRouter != nil {
		return nil
	}

	routerName := fctx.cloudRouterNameFromConfig()

	log.Info("deleting router")
	err := fctx.computeClient.DeleteRouter(ctx, fctx.infra.Spec.Region, routerName)
	if err != nil {
		return err
	}
	fctx.whiteboard.DeleteObject(ObjectKeyRouter)
	return nil
}

func (fctx *FlowContext) ensureCloudNATDeleted(ctx context.Context) error {
	// flow optimization: deletion can be omitted because we will delete gardener router
	// if fctx.config.Networks.VPC == nil || fctx.config.Networks.VPC.CloudRouter == nil {
	// 	log.Info("skipping nat deletion because router is gardener-managed and will be deleted.")
	// 	return nil
	// }

	routerName := fctx.cloudRouterNameFromConfig()
	natName := fctx.cloudNatNameFromConfig()

	router, err := fctx.updater.DeleteNAT(ctx, fctx.infra.Spec.Region, routerName, natName)
	if err != nil {
		return err
	}

	fctx.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (fctx *FlowContext) ensureFirewallRulesDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	vpcName := fctx.vpcNameFromConfig()

	fws, err := fctx.computeClient.ListFirewallRules(ctx, client.FirewallListOpts{
		Filter: fmt.Sprintf(`network eq ".*(%s).*"`, vpcName),
		ClientFilter: func(f *compute.Firewall) bool {
			if strings.HasPrefix(f.Name, infrastructure.KubernetesFirewallNamePrefix) {
				for _, targetTag := range f.TargetTags {
					if targetTag == fctx.clusterName {
						return true
					}
				}
			} else if strings.HasPrefix(f.Name, fctx.clusterName) {
				return true
			}

			return false
		},
	})
	if err != nil {
		return err
	}

	for _, fw := range fws {
		log.Info(fmt.Sprintf("destroying firewall rule [name=%s]", fw.Name))
		err := fctx.computeClient.DeleteFirewallRule(ctx, fw.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

func (fctx *FlowContext) ensureKubernetesRoutesDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	vpcName := fctx.vpcNameFromConfig()

	routes, err := fctx.computeClient.ListRoutes(ctx, client.RouteListOpts{
		Filter: fmt.Sprintf(`network eq ".*(%s).*"`, vpcName),
		ClientFilter: func(route *compute.Route) bool {
			if strings.HasPrefix(route.Name, infrastructure.ShootPrefix) && strings.HasSuffix(route.Network, vpcName) {
				urlParts := strings.Split(route.NextHopInstance, "/")
				if strings.HasPrefix(urlParts[len(urlParts)-1], fctx.clusterName) {
					return true
				}
			}
			return false
		},
	})
	if err != nil {
		return err
	}

	for _, route := range routes {
		log.Info(fmt.Sprintf("destroying route[name=%s]", route.Name))
		err := fctx.computeClient.DeleteRoute(ctx, route.Name)
		if err != nil {
			return err
		}
	}

	return nil
}
