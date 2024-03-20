package infraflow

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/compute/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

func (c *FlowReconciler) ensureServiceAccount(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	serviceAccountName := c.serviceAccountNameFromConfig()
	sa, err := c.iamClient.GetServiceAccount(ctx, serviceAccountName)
	if err != nil {
		return err
	}

	if sa == nil {
		if features.ExtensionFeatureGate.Enabled(features.DisableGardenerServiceAccountCreation) {
			c.Log.Info(fmt.Sprintf("feature gate %s is enabled. Skipping service account creation", features.DisableGardenerServiceAccountCreation))
			return nil
		}

		log.Info("creating service account", "name", serviceAccountName)
		sa, err = c.iamClient.CreateServiceAccount(ctx, serviceAccountName)
		if err != nil {
			return err
		}
		c.whiteboard.SetObject(ObjectKeyServiceAccount, sa)
	}

	return nil
}

func (c *FlowReconciler) ensureVPC(ctx context.Context) error {
	var (
		log = c.LogFromContext(ctx)
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
		log.Info("creating...")
		current, err = c.computeClient.InsertNetwork(ctx, targetVPC)
		if err != nil {
			return err
		}
	} else {
		log.Info("vpc already exists")
		current, err = c.updater.VPC(ctx, c.computeClient, targetVPC, current)
		if err != nil {
			return err
		}
	}

	c.whiteboard.SetObject(ObjectKeyVPC, current)
	return nil
}

func (c *FlowReconciler) ensureUserManagedVPC(ctx context.Context) error {
	var (
		log     = c.LogFromContext(ctx)
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

func (c *FlowReconciler) ensureSubnet(ctx context.Context) error {
	var (
		log    = c.LogFromContext(ctx)
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
		log.Info("creating...")
		subnet, err = c.computeClient.InsertSubnet(ctx, region, targetSubnet)
		if err != nil {
			return err
		}
	} else {
		log.Info("subnet already exists")
		subnet, err = c.updater.Subnet(ctx, c.computeClient, c.infra.Spec.Region, targetSubnet, subnet)
		if err != nil {
			return err
		}
	}

	c.whiteboard.SetObject(ObjectKeyNodeSubnet, subnet)
	return nil
}

func (c *FlowReconciler) ensureInternalSubnet(ctx context.Context) error {
	var (
		log    = c.LogFromContext(ctx)
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
		log.Info("creating...")
		subnet, err = c.computeClient.InsertSubnet(ctx, region, desired)
		if err != nil {
			return err
		}
	} else {
		log.Info("internal subnet already exists")
		subnet, err = c.updater.Subnet(ctx, c.computeClient, c.infra.Spec.Region, desired, subnet)
		if err != nil {
			return err
		}
	}

	c.whiteboard.SetObject(ObjectKeyInternalSubnet, subnet)
	return nil
}

func (c *FlowReconciler) ensureCloudRouter(ctx context.Context) error {
	if c.config.Networks.VPC != nil && c.config.Networks.VPC.CloudRouter != nil {
		return c.ensureUserManagedCloudRouter(ctx)
	}

	log := c.LogFromContext(ctx)

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
		if router, err = c.computeClient.InsertRouter(ctx, c.infra.Spec.Region, desired); err != nil {
			return err
		}
	} else {
		log.Info("router already exists")
		if router, err = c.updater.Router(ctx, c.computeClient, c.infra.Spec.Region, desired, router); err != nil {
			return err
		}
	}

	c.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (c *FlowReconciler) ensureUserManagedCloudRouter(ctx context.Context) error {
	var (
		log        = c.LogFromContext(ctx)
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

func (c *FlowReconciler) ensureAddresses(ctx context.Context) error {
	log := c.LogFromContext(ctx)
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

func (c *FlowReconciler) ensureCloudNAT(ctx context.Context) error {
	var (
		log = c.LogFromContext(ctx)
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
	router, nat, err = c.updater.NAT(ctx, c.computeClient, c.infra.Spec.Region, router, targetNat)
	if err != nil {
		return err
	}

	c.whiteboard.SetObject(ObjectKeyRouter, router)
	c.whiteboard.SetObject(ObjectKeyNAT, nat)
	return nil
}

func (c *FlowReconciler) ensureFirewallRules(ctx context.Context) error {
	var (
		log = c.LogFromContext(ctx)
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
		_, err = c.updater.Firewall(ctx, c.computeClient, rule)
		if err != nil {
			log.Info(fmt.Sprintf("failed to update firewall %s rule: %v", rule.Name, err))
			return err
		}
	}
	return nil
}

func (c *FlowReconciler) ensureVPCDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	networkName := c.vpcNameFromConfig()

	log.Info("deleting vpc")
	if err := c.computeClient.DeleteNetwork(ctx, networkName); err != nil {
		return err
	}
	c.whiteboard.DeleteObject(ObjectKeyVPC)
	return nil
}

func (c *FlowReconciler) ensureSubnetDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	subnetName := c.subnetNameFromConfig()

	log.Info("deleting subnet")
	if err := c.computeClient.DeleteSubnet(ctx, c.infra.Spec.Region, subnetName); err != nil {
		return err
	}

	c.whiteboard.DeleteObject(ObjectKeyNodeSubnet)
	return nil
}

func (c *FlowReconciler) ensureInternalSubnetDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	subnetName := c.internalSubnetNameFromConfig()
	log.Info("deleting internal subnet")
	if err := c.computeClient.DeleteSubnet(ctx, c.infra.Spec.Region, subnetName); err != nil {
		return err
	}

	c.whiteboard.DeleteObject(ObjectKeyInternalSubnet)
	return nil
}

func (c *FlowReconciler) ensureServiceAccountDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	serviceAccountName := c.serviceAccountNameFromConfig()
	log.Info("deleting service account")
	if err := c.iamClient.DeleteServiceAccount(ctx, serviceAccountName); err != nil {
		return err
	}

	c.whiteboard.DeleteObject(ObjectKeyServiceAccount)
	return nil
}

func (c *FlowReconciler) ensureCloudRouterDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	if c.config.Networks.VPC != nil && c.config.Networks.VPC.CloudRouter != nil {
		return nil
	}

	routerName := c.cloudRouterNameFromConfig()

	log.Info("deleting router")
	if err := c.computeClient.DeleteRouter(ctx, c.infra.Spec.Region, routerName); err != nil {
		return err
	}
	c.whiteboard.DeleteObject(ObjectKeyRouter)
	return nil
}

func (c *FlowReconciler) ensureCloudNATDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)

	// flow optimization: deletion can be omitted because we will delete gardener router
	// if c.config.Networks.VPC == nil || c.config.Networks.VPC.CloudRouter == nil {
	// 	log.Info("skipping nat deletion because router is gardener-managed and will be deleted.")
	// 	return nil
	// }

	routerName := c.cloudRouterNameFromConfig()
	natName := c.cloudNatNameFromConfig()

	log.Info("deleting nat")
	router, err := c.updater.DeleteNAT(ctx, c.computeClient, c.infra.Spec.Region, routerName, natName)
	if err != nil {
		return err
	}
	log.Info("nat deleted successfully")
	c.whiteboard.SetObject(ObjectKeyRouter, router)
	return nil
}

func (c *FlowReconciler) ensureFirewallRulesDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)

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

func (c *FlowReconciler) ensureKubernetesRoutesDeleted(ctx context.Context) error {
	log := c.LogFromContext(ctx)
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
