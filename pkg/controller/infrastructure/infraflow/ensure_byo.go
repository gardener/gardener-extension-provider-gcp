package infraflow

import (
	"context"
	"fmt"

	"google.golang.org/api/compute/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
)

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

// ensureUserManagedWorkersSubnet looks up the user-provided nodes subnet and stores it on the whiteboard.
// It does NOT create or modify the subnet — only reads it.
func (fctx *FlowContext) ensureUserManagedWorkersSubnet(ctx context.Context) error {
	subnetRef := fctx.config.Networks.SubnetWorkers
	if subnetRef == nil {
		return fmt.Errorf("subnetWorkers must not be nil in BYO mode")
	}

	subnet, err := fctx.computeClient.GetSubnet(ctx, fctx.infra.Spec.Region, subnetRef.Name)
	if err != nil {
		return err
	}
	if subnet == nil {
		return fmt.Errorf("user-managed nodes subnet %q not found in region %q", subnetRef.Name, fctx.infra.Spec.Region)
	}

	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)
	if subnet.Network != vpc.SelfLink {
		return fmt.Errorf("user-managed nodes subnet %q belongs to network %q, not to the configured VPC %q",
			subnetRef.Name, subnet.Network, fctx.config.Networks.VPC.Name)
	}

	if fctx.isDualStack() && subnetRef.PodSecondaryRangeName != nil {
		err = validatePodSecondaryRange(subnetRef.Name, *subnetRef.PodSecondaryRangeName, subnet.SecondaryIpRanges)
		if err != nil {
			return err
		}
	}

	// Mark that Gardener has reconciled resources it must clean up on deletion (kubernetes routes and CCM
	// firewall rules). Without this marker the delete flow short-circuits and those resources leak, because
	// BYO mode does not create the VPC/subnets that otherwise set it.
	fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
	fctx.whiteboard.SetObject(ObjectKeyNodeSubnet, subnet)
	return nil
}

// ensureUserManagedServicesSubnet looks up the user-provided services subnet and stores it on the whiteboard.
// It does NOT create or modify the subnet — only reads it.
func (fctx *FlowContext) ensureUserManagedServicesSubnet(ctx context.Context) error {
	subnetRef := fctx.config.Networks.SubnetServices
	if subnetRef == nil {
		return fmt.Errorf("subnetServices must not be nil for dual-stack BYO mode")
	}

	subnet, err := fctx.computeClient.GetSubnet(ctx, fctx.infra.Spec.Region, subnetRef.Name)
	if err != nil {
		return err
	}
	if subnet == nil {
		return fmt.Errorf("user-managed services subnet %q not found in region %q", subnetRef.Name, fctx.infra.Spec.Region)
	}

	vpc := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC)
	if subnet.Network != vpc.SelfLink {
		return fmt.Errorf("user-managed services subnet %q belongs to network %q, not to the configured VPC %q", subnetRef.Name, subnet.Network, fctx.config.Networks.VPC.Name)
	}

	fctx.whiteboard.SetObject(ObjectKeyServicesSubnet, subnet)
	return nil
}
