package infraflow

import (
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"google.golang.org/api/compute/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	ctclient "sigs.k8s.io/controller-runtime/pkg/client"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
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
	var err error

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

func (fctx *FlowContext) ensureIPv6CIDRs(ctx context.Context) error {
	nodeSubnet, ok := fctx.whiteboard.GetObject(ObjectKeyNodeSubnet).(*compute.Subnetwork)
	if !ok || nodeSubnet == nil {
		return fmt.Errorf("failed to get the subnet for nodes")
	}

	nodesIPv6Range, err := fctx.computeClient.WaitForIPv6Cidr(ctx, fctx.infra.Spec.Region, fmt.Sprintf("%d", nodeSubnet.Id))
	if err != nil {
		return err
	}
	fctx.whiteboard.Set(NodesSubnetIPv6CIDR, nodesIPv6Range)

	srvSubnet, ok := fctx.whiteboard.GetObject(ObjectKeyServicesSubnet).(*compute.Subnetwork)
	if !ok || srvSubnet == nil {
		return fmt.Errorf("failed to get the subnet for services")
	}

	srvIPv6Range, err := fctx.computeClient.WaitForIPv6Cidr(ctx, fctx.infra.Spec.Region, fmt.Sprintf("%d", srvSubnet.Id))
	if err != nil {
		return err
	}
	fctx.whiteboard.Set(ServicesSubnetIPv6CIDR, srvIPv6Range)

	return nil
}

func (fctx *FlowContext) ensureKubernetesRoutesCleanupForDualStackMigration(ctx context.Context) error {
	if err := fctx.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}

	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)

	cloudRoutes, err := infrastructure.ListKubernetesRoutes(ctx, fctx.computeClient, vpc.Name, fctx.clusterName)
	if err != nil {
		return err
	}

	if len(cloudRoutes) == 0 {
		// we don't need to do anything
		return nil
	}

	ccmDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: fctx.technicalID, Name: gcp.CloudControllerManagerName},
	}
	ccmScale := &autoscalingv1.Scale{}
	scaleClient := fctx.runtimeClient.SubResource("scale")
	if err := scaleClient.Get(ctx, ccmDeploy, ccmScale); err != nil {
		return err
	}

	scaleCCMto := func(scale int32) error {
		ccmScale.Spec.Replicas = scale
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return scaleClient.Update(ctx, ccmDeploy, ctclient.WithSubResourceBody(ccmScale))
		})
	}

	originalScale := ccmScale.Spec.Replicas

	// scale CCM to zero
	if err = scaleCCMto(0); err != nil {
		return err
	}

	routes := []v1alpha1.Route{}
	// Safely retrieve and cast the routes object
	if routesObj := fctx.whiteboard.GetObject(ObjectKeyRoutes); routesObj != nil {
		if castedRoutes, ok := routesObj.([]v1alpha1.Route); ok {
			routes = castedRoutes
		}
	}

	// Ensure all cloudRoutes are added to the whiteboard, avoiding duplicates
	for _, route := range cloudRoutes {
		instanceName, zone, err := extractInstanceAndZone(route.NextHopInstance)
		if err != nil {
			return err
		}

		// Check if the route already exists in the list
		exists := false
		for _, r := range routes {
			if r.InstanceName == instanceName && r.DestinationCIDR == route.DestRange && r.Zone == zone {
				exists = true
				break
			}
		}

		// Add the route if it doesn't already exist
		if !exists {
			routes = append(routes, v1alpha1.Route{
				InstanceName:    instanceName,
				DestinationCIDR: route.DestRange,
				Zone:            zone,
			})
		}
	}
	fctx.whiteboard.SetObject(ObjectKeyRoutes, routes)
	err = fctx.persistState(ctx)
	if err != nil {
		fctx.log.Error(err, "failed to persist state")
	}

	// Delete all cloudroutes
	for _, route := range cloudRoutes {
		fctx.log.Info("Deleting route", "name", route.Name, "destRange", route.DestRange)
		if err := fctx.computeClient.DeleteRoute(ctx, route.Name); err != nil {
			// scale CCM back to originalScale in case if there are errors from DeleteRoutes() call
			_ = scaleCCMto(originalScale)
			return err
		}
	}
	return nil
}

func (fctx *FlowContext) ensureNodesSubnet(ctx context.Context) error {
	region := fctx.infra.Spec.Region

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
		!gardencorev1beta1.IsIPv4SingleStack(fctx.networking.IPFamilies),
		fctx.networking.Pods,
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
	region := fctx.infra.Spec.Region

	if fctx.config.Networks.Internal == nil {
		return fctx.ensureSubnetDeletedFactory(fctx.internalSubnetNameFromConfig(), ObjectKeyInternalSubnet)(ctx)
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
		!gardencorev1beta1.IsIPv4SingleStack(fctx.networking.IPFamilies),
		nil,
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

func (fctx *FlowContext) ensureServicesSubnet(ctx context.Context) error {
	region := fctx.infra.Spec.Region

	if err := fctx.ensureObjectKeys(ObjectKeyVPC); err != nil {
		return err
	}
	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)

	subnetName := fctx.servicesSubnetNameFromConfig()

	subnet, err := fctx.computeClient.GetSubnet(ctx, region, subnetName)
	if err != nil {
		return err
	}

	desired := targetSubnetState(
		subnetName,
		"gardener-managed services subnet",
		*fctx.networking.Services,
		vpc.SelfLink,
		nil,
		!gardencorev1beta1.IsIPv4SingleStack(fctx.networking.IPFamilies),
		nil,
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
	fctx.whiteboard.SetObject(ObjectKeyServicesSubnet, subnet)
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

	cidrs := []*string{
		fctx.networking.Pods,
		fctx.config.Networks.Internal,
		ptr.To(fctx.config.Networks.Workers),
		ptr.To(fctx.config.Networks.Worker),
	}

	rules := []*compute.Firewall{
		firewallRuleAllowInternal(FirewallRuleAllowInternalName(fctx.clusterName), vpc.SelfLink, cidrs),
		firewallRuleAllowHealthChecks(FirewallRuleAllowHealthChecksName(fctx.clusterName), vpc.SelfLink, healthCheckSourceRangesIPv4),
	}

	cidrsIPv6 := []*string{}
	if nodesIPv6 := fctx.whiteboard.Get(NodesSubnetIPv6CIDR); ptr.Deref(nodesIPv6, "") != "" {
		cidrsIPv6 = append(cidrsIPv6, nodesIPv6)
	}
	if servicesIPv6 := fctx.whiteboard.Get(ServicesSubnetIPv6CIDR); ptr.Deref(servicesIPv6, "") != "" {
		cidrsIPv6 = append(cidrsIPv6, servicesIPv6)
	}

	if len(cidrsIPv6) > 0 {
		rules = append(rules,
			firewallRuleAllowInternalIPv6(FirewallRuleAllowInternalNameIPv6(fctx.clusterName), vpc.SelfLink, cidrsIPv6),
			firewallRuleAllowHealthChecks(FirewallRuleAllowHealthChecksNameIPv6(fctx.clusterName), vpc.SelfLink, healthCheckSourceRangesIPv6),
		)
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
	return fctx.computeClient.DeleteFirewallRule(ctx, FirewallRuleAllowExternalName(fctx.clusterName))
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

func (fctx *FlowContext) ensureSubnetDeletedFactory(subnetName string, whiteboardKey string) func(context.Context) error {
	return func(ctx context.Context) error {
		log := shared.LogFromContext(ctx)

		log.Info("deleting", "subnet", subnetName)
		err := fctx.computeClient.DeleteSubnet(ctx, fctx.infra.Spec.Region, subnetName)
		if err != nil {
			return err
		}

		fctx.whiteboard.DeleteObject(whiteboardKey)
		return nil
	}
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
			} else if sets.New(
				FirewallRuleAllowInternalName(fctx.clusterName),
				FirewallRuleAllowInternalNameIPv6(fctx.clusterName),
				FirewallRuleAllowHealthChecksNameIPv6(fctx.clusterName),
				FirewallRuleAllowHealthChecksName(fctx.clusterName),
				FirewallRuleAllowExternalName(fctx.clusterName),
			).Has(f.Name) {
				return true
			}
			return false
		},
	})
	if err != nil {
		return err
	}

	for _, fw := range fws {
		log.Info("destroying firewall rule", "name", fw.Name)
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
		log.Info("destroying route", "name", route.Name)
		err := fctx.computeClient.DeleteRoute(ctx, route.Name)
		if err != nil {
			return err
		}
	}

	return nil
}

func (fctx *FlowContext) ensureAliasIpRanges(ctx context.Context) error {
	log := shared.LogFromContext(ctx)

	// Retrieve and cast the routes object from the whiteboard
	routes, ok := fctx.whiteboard.GetObject(ObjectKeyRoutes).([]v1alpha1.Route)
	if !ok || routes == nil {
		log.Info("No routes found in the whiteboard")
		return nil
	}

	// Iterate over the routes and process each one
	// Adding an already existing alias IP route will not fail so we can persist the state at the end
	// of the loop
	for _, route := range routes {
		// Attempt to insert the alias IP route
		err := fctx.computeClient.InsertAliasIPRoute(ctx, apisgcp.Route{
			InstanceName:    route.InstanceName,
			DestinationCIDR: route.DestinationCIDR,
			Zone:            route.Zone,
		}, DefaultSecondarySubnetName)

		if err != nil {
			// Handle errors other than "notFound"
			if !strings.Contains(err.Error(), "notFound") {
				log.Error(err, "Failed to add alias IP", "route", route.DestinationCIDR, "instance", route.InstanceName)
				return err
			}

			// Log and skip if the machine is already gone
			log.Info("Machine not found, skipping route", "route", route.DestinationCIDR, "instance", route.InstanceName)
			continue
		}

		// Log success and remove the processed route from the list
		log.Info("Successfully added alias IP", "route", route.DestinationCIDR, "instance", route.InstanceName)
	}

	// All routes have been processed, so we can clear the whiteboard
	// and persist the state
	fctx.whiteboard.SetObject(ObjectKeyRoutes, []v1alpha1.Route{})
	err := fctx.persistState(ctx)
	if err != nil {
		fctx.log.Error(err, "failed to persist state")
	}
	return nil
}
