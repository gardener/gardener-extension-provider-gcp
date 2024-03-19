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

func (c *FlowContext) ensureServiceAccount(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	serviceAccountName := c.serviceAccountNameFromConfig()
	sa, err := c.iamClient.GetServiceAccount(ctx, serviceAccountName)
	if err != nil {
		return err
	}

	if sa == nil {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "creating service account", "name", serviceAccountName)
		sa, err = c.iamClient.CreateServiceAccount(ctx, serviceAccountName)
		if err != nil {
			return err
		}
		w.Done(err)
		c.whiteboard.GetChild(ChildKeyIDs).Set(KeyServiceAccountEmail, sa.Email)
	}

	return nil
}

func (c *FlowContext) ensureVPC(ctx context.Context) error {
	var (
		log = shared.LogFromContext(ctx)
		err error
	)

	if c.config.Networks.VPC != nil {
		return c.ensureUserManagedVPC(ctx)
	}

	vpcName := c.vpcNameFromConfig()

	current, err := c.computeClient.GetNetwork(ctx, vpcName)
	if err != nil {
		return err
	}

	targetVPC := targetNetwork(vpcName)
	if current == nil {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "creating vpc", "name", targetVPC.Name)
		current, err = c.computeClient.InsertNetwork(ctx, targetVPC)
		if err != nil {
			return err
		}
		w.Done(err)
	} else {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "updating vpc", "name", targetVPC.Name)
		current, err = c.updater.VPC(ctx, targetVPC, current)
		if err != nil {
			return err
		}
		w.Done(err)
	}

	c.whiteboard.Set(CreatedResourcesExistKey, "true")
	c.whiteboard.SetObject(ObjectKeyVPC, current)
	return nil
}

func (c *FlowContext) ensureUserManagedVPC(ctx context.Context) error {
	var (
		log     = shared.LogFromContext(ctx)
		vpcSpec = c.config.Networks.VPC
		vpcName = vpcSpec.Name
		err     error
	)

	vpc, err := c.computeClient.GetNetwork(ctx, vpcName)
	if err != nil {
		return err
	}
	if vpc == nil {
		log.Error(nil, fmt.Sprintf("failed to locate user-managed VPC [Name=%s]", vpcName))
		return fmt.Errorf("failed to locate user-managed VPC [Name=%s]", vpcName)
	}

	c.whiteboard.SetObject(ObjectKeyVPC, vpc)
	return nil
}

func (c *FlowContext) ensureSubnet(ctx context.Context) error {
	var (
		log    = shared.LogFromContext(ctx)
		region = c.infra.Spec.Region
	)

	if err := c.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](c.whiteboard, ObjectKeyVPC)

	subnetName := c.subnetNameFromConfig()
	cidr := c.config.Networks.Workers
	if len(cidr) == 0 {
		cidr = c.config.Networks.Worker
	}

	targetSubnet := targetSubnetState(
		subnetName,
		"gardener-managed worker subnet",
		cidr,
		vpc.SelfLink,
		c.config.Networks.FlowLogs,
	)

	subnet, err := c.computeClient.GetSubnet(ctx, region, subnetName)
	if err != nil {
		return err
	}

	if subnet == nil {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "creating subnet", "name", subnetName)
		subnet, err = c.computeClient.InsertSubnet(ctx, region, targetSubnet)
		if err != nil {
			return err
		}
		w.Done(err)
	} else {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "updating subnet", "name", subnetName)
		subnet, err = c.updater.Subnet(ctx, c.infra.Spec.Region, targetSubnet, subnet)
		if err != nil {
			return err
		}
		w.Done(err)
	}

	c.whiteboard.Set(CreatedResourcesExistKey, "true")
	c.whiteboard.SetObject(ObjectKeyNodeSubnet, subnet)
	return nil
}

func (c *FlowContext) ensureInternalSubnet(ctx context.Context) error {
	var (
		log    = shared.LogFromContext(ctx)
		region = c.infra.Spec.Region
	)

	if c.config.Networks.Internal == nil {
		return c.ensureInternalSubnetDeleted(ctx)
	}

	if err := c.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](c.whiteboard, ObjectKeyVPC)

	subnetName := c.internalSubnetNameFromConfig()

	subnet, err := c.computeClient.GetSubnet(ctx, region, subnetName)
	if err != nil {
		return err
	}

	desired := targetSubnetState(
		subnetName,
		"gardener-managed internal subnet",
		*c.config.Networks.Internal,
		vpc.SelfLink,
		nil,
	)
	if subnet == nil {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "creating subnet", "name", subnetName)
		subnet, err = c.computeClient.InsertSubnet(ctx, region, desired)
		if err != nil {
			return err
		}
		w.Done(err)
	} else {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "updating subnet", "name", subnetName)
		subnet, err = c.updater.Subnet(ctx, c.infra.Spec.Region, desired, subnet)
		if err != nil {
			return err
		}
		w.Done(err)
	}

	c.whiteboard.Set(CreatedResourcesExistKey, "true")
	c.whiteboard.SetObject(ObjectKeyInternalSubnet, subnet)
	return nil
}

func (c *FlowContext) ensureCloudRouter(ctx context.Context) error {
	if c.config.Networks.VPC != nil && c.config.Networks.VPC.CloudRouter != nil {
		return c.ensureUserManagedCloudRouter(ctx)
	}

	log := shared.LogFromContext(ctx)

	if err := c.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](c.whiteboard, ObjectKeyVPC)

	routerName := c.cloudRouterNameFromConfig()

	desired := targetRouterState(routerName, "gardener-managed router", vpc.SelfLink)
	router, err := c.computeClient.GetRouter(ctx, c.infra.Spec.Region, routerName)
	if err != nil {
		return err
	}

	if router == nil {
		log.Info("creating...")
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "creating router", "name", routerName)
		if router, err = c.computeClient.InsertRouter(ctx, c.infra.Spec.Region, desired); err != nil {
			return err
		}
		w.Done(err)
	} else {
		w := shared.InformOnWaiting(log, defaultWaiterPeriod, "updating router", "name", routerName)
		if router, err = c.updater.Router(ctx, c.infra.Spec.Region, desired, router); err != nil {
			return err
		}
		w.Done(err)
	}

	c.whiteboard.Set(CreatedResourcesExistKey, "true")
	c.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (c *FlowContext) ensureUserManagedCloudRouter(ctx context.Context) error {
	var (
		log        = shared.LogFromContext(ctx)
		routerName = c.config.Networks.VPC.CloudRouter.Name
	)

	log.Info("ensuring user-managed router")

	router, err := c.computeClient.GetRouter(ctx, c.infra.Spec.Region, routerName)
	if err != nil {
		return err
	}
	if router == nil {
		return fmt.Errorf("failed to locate user-managed CloudRouter [Name=%s]: %v", routerName, err)
	}

	c.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (c *FlowContext) ensureAddresses(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	if c.config.Networks.CloudNAT == nil || len(c.config.Networks.CloudNAT.NatIPNames) == 0 {
		return nil
	}

	var addresses []string
	for _, name := range c.config.Networks.CloudNAT.NatIPNames {
		ip, err := c.computeClient.GetAddress(ctx, c.infra.Spec.Region, name.Name)
		if err != nil {
			log.Error(err, "failed to locate user-managed IP address")
			return err
		}
		addresses = append(addresses, ip.SelfLink)
	}

	if len(addresses) > 0 {
		c.whiteboard.SetObject(ObjectKeyIPAddress, addresses)
	}
	return nil
}

func (c *FlowContext) ensureCloudNAT(ctx context.Context) error {
	var (
		log = shared.LogFromContext(ctx)
		err error
	)

	log.Info("ensuring nat")

	if err := c.ensureObjectKeys(ObjectKeyRouter, ObjectKeyNodeSubnet); err != nil {
		return err
	}

	subnet := GetObject[*client.Subnetwork](c.whiteboard, ObjectKeyNodeSubnet)
	router := GetObject[*client.Router](c.whiteboard, ObjectKeyRouter)

	natName := c.cloudNatNameFromConfig()
	var (
		nat       *compute.RouterNat
		addresses []string
	)

	if a := c.whiteboard.GetObject(ObjectKeyIPAddress); a != nil {
		addresses = a.([]string)
	}

	targetNat := targetNATState(natName, subnet.SelfLink, c.config.Networks.CloudNAT, addresses)
	w := shared.InformOnWaiting(log, defaultWaiterPeriod, "ensuring cloudNAT", "name", targetNat.Name)
	router, nat, err = c.updater.NAT(ctx, c.infra.Spec.Region, router, targetNat)
	if err != nil {
		return err
	}
	w.Done(err)

	c.whiteboard.SetObject(ObjectKeyRouter, router)
	c.whiteboard.SetObject(ObjectKeyNAT, nat)
	return nil
}

func (c *FlowContext) ensureFirewallRules(ctx context.Context) error {
	var (
		log = shared.LogFromContext(ctx)
	)

	if err := c.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](c.whiteboard, ObjectKeyVPC)

	cidrs := []*string{c.podCIDR, c.config.Networks.Internal, ptr.To(c.config.Networks.Workers), ptr.To(c.config.Networks.Worker)}
	rules := []*compute.Firewall{
		firewallRuleAllowExternal(firewallRuleAllowExternalName(c.clusterName), vpc.SelfLink),
		firewallRuleAllowInternal(firewallRuleAllowInternalName(c.clusterName), vpc.SelfLink, cidrs),
		firewallRuleAllowHealthChecks(firewallRuleAllowHealthChecksName(c.clusterName), vpc.SelfLink),
	}

	for _, rule := range rules {
		gcpRule, err := c.computeClient.GetFirewallRule(ctx, rule.Name)
		if err != nil {
			log.Info(fmt.Sprintf("failed to create firewall %s rule: %v", rule.Name, err))
			return fmt.Errorf("failed to ensure firewall rule [name=%s]: %v", gcpRule.Name, err)
		}

		if gcpRule == nil {
			if _, err = c.computeClient.InsertFirewallRule(ctx, rule); err != nil {
				log.Info(fmt.Sprintf("failed to create firewall %s rule: %v", rule.Name, err))
				return err
			}
			continue
		}
		_, err = c.updater.Firewall(ctx, rule)
		if err != nil {
			log.Info(fmt.Sprintf("failed to update firewall %s rule: %v", rule.Name, err))
			return err
		}
	}
	return nil
}

func (c *FlowContext) ensureVPCDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	networkName := c.vpcNameFromConfig()
	w := shared.InformOnWaiting(log, defaultWaiterPeriod, "deleting vpc", "name", networkName)
	err := c.computeClient.DeleteNetwork(ctx, networkName)
	w.Done(err)
	if err != nil {
		return err
	}
	c.whiteboard.DeleteObject(ObjectKeyVPC)
	return nil
}

func (c *FlowContext) ensureSubnetDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	subnetName := c.subnetNameFromConfig()

	w := shared.InformOnWaiting(log, defaultWaiterPeriod, "deleting internal subnet", "name", subnetName)
	err := c.computeClient.DeleteSubnet(ctx, c.infra.Spec.Region, subnetName)
	w.Done(err)
	if err != nil {
		return err
	}

	c.whiteboard.DeleteObject(ObjectKeyNodeSubnet)
	return nil
}

func (c *FlowContext) ensureInternalSubnetDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	subnetName := c.internalSubnetNameFromConfig()
	log.Info("deleting internal subnet")
	w := shared.InformOnWaiting(log, defaultWaiterPeriod, "deleting internal subnet", "name", subnetName)
	err := c.computeClient.DeleteSubnet(ctx, c.infra.Spec.Region, subnetName)
	w.Done(err)
	if err != nil {
		return err
	}

	c.whiteboard.DeleteObject(ObjectKeyInternalSubnet)
	return nil
}

func (c *FlowContext) ensureServiceAccountDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	serviceAccountName := c.serviceAccountNameFromConfig()

	log.Info("deleting service account")
	if err := c.iamClient.DeleteServiceAccount(ctx, serviceAccountName); err != nil {
		return err
	}

	c.whiteboard.GetChild(ChildKeyIDs).Delete(KeyServiceAccountEmail)
	return nil
}

func (c *FlowContext) ensureCloudRouterDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	if c.config.Networks.VPC != nil && c.config.Networks.VPC.CloudRouter != nil {
		return nil
	}

	routerName := c.cloudRouterNameFromConfig()

	log.Info("deleting router")
	w := shared.InformOnWaiting(log, defaultWaiterPeriod, "deleting router", "name", routerName)
	err := c.computeClient.DeleteRouter(ctx, c.infra.Spec.Region, routerName)
	w.Done(err)
	if err != nil {
		return err
	}
	c.whiteboard.DeleteObject(ObjectKeyRouter)
	return nil
}

func (c *FlowContext) ensureCloudNATDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	// flow optimization: deletion can be omitted because we will delete gardener router
	// if c.config.Networks.VPC == nil || c.config.Networks.VPC.CloudRouter == nil {
	// 	log.Info("skipping nat deletion because router is gardener-managed and will be deleted.")
	// 	return nil
	// }

	routerName := c.cloudRouterNameFromConfig()
	natName := c.cloudNatNameFromConfig()

	w := shared.InformOnWaiting(log, defaultWaiterPeriod, "deleting cloudNAT", "name", natName)
	router, err := c.updater.DeleteNAT(ctx, c.infra.Spec.Region, routerName, natName)
	if err != nil {
		return err
	}
	w.Done(err)

	c.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (c *FlowContext) ensureFirewallRulesDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	vpcName := c.vpcNameFromConfig()

	var names []string

	fws, err := c.computeClient.ListFirewallRules(ctx)
	if err != nil {
		return err
	}

	for _, firewall := range fws {
		if strings.HasSuffix(firewall.Network, vpcName) {
			if strings.HasPrefix(firewall.Name, infrastructure.KubernetesFirewallNamePrefix) {
				for _, targetTag := range firewall.TargetTags {
					if targetTag == c.clusterName {
						names = append(names, firewall.Name)
						break
					}
				}
			} else if strings.HasPrefix(firewall.Name, c.clusterName) {
				names = append(names, firewall.Name)
			}
		}
	}

	for _, name := range names {
		log.Info(fmt.Sprintf("destroying firewall rule [name=%s]", name))
		err := c.computeClient.DeleteFirewallRule(ctx, name)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *FlowContext) ensureKubernetesRoutesDeleted(ctx context.Context) error {
	log := shared.LogFromContext(ctx)
	vpcName := c.vpcNameFromConfig()

	var names []string
	routes, err := c.computeClient.ListRoutes(ctx)
	if err != nil {
		return err
	}

	for _, route := range routes {
		if strings.HasPrefix(route.Name, infrastructure.ShootPrefix) && strings.HasSuffix(route.Network, vpcName) {
			urlParts := strings.Split(route.NextHopInstance, "/")
			if strings.HasPrefix(urlParts[len(urlParts)-1], c.clusterName) {
				names = append(names, route.Name)
			}
		}
	}

	for _, name := range names {
		log.Info(fmt.Sprintf("destroying route[name=%s]", name))
		err := c.computeClient.DeleteRoute(ctx, name)
		if err != nil {
			return err
		}
	}

	return nil
}
