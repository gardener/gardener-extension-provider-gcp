package infraflow

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"google.golang.org/api/compute/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	gcpinternal "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

const (

	// CreatedServiceAccountKey marks whether we have created a service account for the shoot. If not we will skip reconciling the service accounts
	CreatedServiceAccountKey = "service_account_exist"
	// CreatedResourcesExistKey is a marker for the Terraform migration case. If the TF state is not empty
	// we inject this marker into the state to block the deletion without having first a successful reconciliation.
	CreatedResourcesExistKey = "resources_exist"
	// ChildKeyIDs is the prefix key for all ids.
	ChildKeyIDs = "ids"
	// NodesSubnetIPv6CIDR is the IPv6 CIDR block attached to the subnet
	NodesSubnetIPv6CIDR = "nodes-subnet-ipv6-cidr"
	// ServicesSubnetIPv6CIDR is the IPv6 CIDR block attached to the subnet
	ServicesSubnetIPv6CIDR = "services-subnet-ipv6-cidr"
	// KeyServiceAccountEmail is the key to store the service account object.
	KeyServiceAccountEmail = "service-account-email"
	// ObjectKeyVPC is the key to store the VPC object.
	ObjectKeyVPC = "vpc"
	// ObjectKeyNodeSubnet is the key to store the nodes subnet object.
	ObjectKeyNodeSubnet = "subnet-nodes"
	// ObjectKeyInternalSubnet is the key to store the internal subnet object.
	ObjectKeyInternalSubnet = "subnet-internal"
	// ObjectKeyServicesSubnet is the key to store the internal subnet object.
	ObjectKeyServicesSubnet = "subnet-services"
	// ObjectKeyRouter router is the key for the CloudRouter.
	ObjectKeyRouter = "router"
	// ObjectKeyNAT is the key for the .CloudNAT object.
	ObjectKeyNAT = "nat"
	// ObjectKeyIPAddresses is the key for the IP Address slice.
	ObjectKeyIPAddresses = "addresses/ip"
	// ObjectKeyRoutes is the key for the routes slice.
	ObjectKeyRoutes = "routes"
)

// DefaultUpdaterFunc is the default constructor used for an Updated used in the package.
var DefaultUpdaterFunc = gcpclient.NewUpdater

// FlowContext is capable of reconciling and deleting the infrastructure for a shoot.
type FlowContext struct {
	infra             *extensionsv1alpha1.Infrastructure
	config            *gcp.InfrastructureConfig
	state             *gcp.InfrastructureState
	updater           gcpclient.Updater
	credentialsConfig *gcpinternal.CredentialsConfig
	clusterName       string
	technicalID       string
	runtimeClient     client.Client
	networking        *gardencorev1beta1.Networking
	whiteboard        shared.Whiteboard
	log               logr.Logger
	shoot             *gardencorev1beta1.Shoot

	computeClient gcpclient.ComputeClient
	iamClient     gcpclient.IAMClient
	*shared.BasicFlowContext
}

// Opts contains the options to initialize a FlowContext.
type Opts struct {
	// Log is the logger using during the reconciliation.
	Log logr.Logger
	// Infra
	Infra   *extensionsv1alpha1.Infrastructure
	State   *gcp.InfrastructureState
	Cluster *controller.Cluster

	CredentialsConfig *gcpinternal.CredentialsConfig
	Factory           gcpclient.Factory
	Client            client.Client
}

// NewFlowContext returns a new FlowContext.
func NewFlowContext(ctx context.Context, opts Opts) (*FlowContext, error) {
	wb := shared.NewWhiteboard()
	wb.ImportFromFlatMap(opts.State.Data)
	wb.SetObject(ObjectKeyRoutes, opts.State.Routes)
	config, err := helper.InfrastructureConfigFromInfrastructure(opts.Infra)
	if err != nil {
		return nil, err
	}

	com, err := opts.Factory.Compute(ctx, opts.Client, opts.Infra.Spec.SecretRef)
	if err != nil {
		return nil, err
	}
	iam, err := opts.Factory.IAM(ctx, opts.Client, opts.Infra.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	fr := &FlowContext{
		whiteboard:        wb,
		infra:             opts.Infra,
		credentialsConfig: opts.CredentialsConfig,
		config:            config,
		state:             opts.State,
		updater:           DefaultUpdaterFunc(opts.Log, com),
		clusterName:       opts.Cluster.ObjectMeta.Name,
		runtimeClient:     opts.Client,
		technicalID:       opts.Cluster.Shoot.Status.TechnicalID,
		log:               opts.Log,
		shoot:             opts.Cluster.Shoot,
		networking:        opts.Cluster.Shoot.Spec.Networking,
		computeClient:     com,
		iamClient:         iam,
	}

	return fr, nil
}

// Reconcile reconciles the infrastructure
func (fctx *FlowContext) Reconcile(ctx context.Context) error {
	g := fctx.buildReconcileGraph()
	f := g.Compile()
	fctx.log.Info("starting Flow Reconciliation")
	err := f.Run(ctx, flow.Opts{
		Log: fctx.log,
	})
	if err != nil {
		err = flow.Causes(err)
		fctx.log.Error(err, "flow reconciliation failed")
		return errors.Join(err, fctx.persistState(ctx))
	}

	status := fctx.getStatus()
	state := fctx.getCurrentState()
	nodesSubnetIPv6CIDR := fctx.whiteboard.Get(NodesSubnetIPv6CIDR)
	servicesSubnetIPv6CIDR := fctx.whiteboard.Get(ServicesSubnetIPv6CIDR)

	return PatchProviderStatusAndState(
		ctx,
		fctx.runtimeClient,
		fctx.infra,
		fctx.networking,
		status,
		state,
		nodesSubnetIPv6CIDR,
		servicesSubnetIPv6CIDR,
	)
}

// Delete is used to destroy the infrastructure.
func (fctx *FlowContext) Delete(ctx context.Context) error {
	if fctx.state.Data == nil || !strings.EqualFold(fctx.state.Data[CreatedResourcesExistKey], "true") {
		return nil
	}

	g := fctx.buildDeleteGraph()
	f := g.Compile()
	return f.Run(ctx, flow.Opts{Log: fctx.log})
}

func (fctx *FlowContext) getStatus() *v1alpha1.InfrastructureStatus {
	status := &v1alpha1.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureStatus",
		},
		Networks: v1alpha1.NetworkStatus{
			VPC:        v1alpha1.VPC{},
			Subnets:    []v1alpha1.Subnet{},
			NatIPs:     []v1alpha1.NatIP{},
			IPFamilies: fctx.networking.IPFamilies,
		},
	}

	if n := GetObject[*compute.Network](fctx.whiteboard, ObjectKeyVPC); n != nil {
		status.Networks.VPC.Name = n.Name
	}

	if s := GetObject[*compute.Subnetwork](fctx.whiteboard, ObjectKeyNodeSubnet); s != nil {
		status.Networks.Subnets = append(status.Networks.Subnets, v1alpha1.Subnet{
			Name:    s.Name,
			Purpose: v1alpha1.PurposeNodes,
		})
	}

	if s := GetObject[*compute.Subnetwork](fctx.whiteboard, ObjectKeyInternalSubnet); s != nil {
		status.Networks.Subnets = append(status.Networks.Subnets, v1alpha1.Subnet{
			Name:    s.Name,
			Purpose: v1alpha1.PurposeInternal,
		})
	}

	if s := GetObject[*compute.Subnetwork](fctx.whiteboard, ObjectKeyServicesSubnet); s != nil {
		status.Networks.Subnets = append(status.Networks.Subnets, v1alpha1.Subnet{
			Name:    s.Name,
			Purpose: v1alpha1.PurposeServices,
		})
	}

	if router := GetObject[*compute.Router](fctx.whiteboard, ObjectKeyRouter); router != nil {
		status.Networks.VPC.CloudRouter = &v1alpha1.CloudRouter{
			Name: router.Name,
		}
	}

	if ipAddresses := fctx.whiteboard.GetObject(ObjectKeyIPAddresses); ipAddresses != nil {
		for _, ip := range ipAddresses.([]*compute.Address) {
			status.Networks.NatIPs = append(status.Networks.NatIPs, v1alpha1.NatIP{
				IP: ip.Address,
			})
		}
	}

	status.ServiceAccountEmail = ptr.Deref(fctx.whiteboard.GetChild(ChildKeyIDs).Get(KeyServiceAccountEmail), "")
	return status
}

func (fctx *FlowContext) getCurrentState() *runtime.RawExtension {
	routes := []v1alpha1.Route{}

	// Safely retrieve and cast the routes object
	if routesObj := fctx.whiteboard.GetObject(ObjectKeyRoutes); routesObj != nil {
		if castedRoutes, ok := routesObj.([]v1alpha1.Route); ok {
			routes = castedRoutes
		}
	}

	state := &v1alpha1.InfrastructureState{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureState",
		},
		Data:   fctx.whiteboard.ExportAsFlatMap(),
		Routes: routes, // Use the safely casted routes
	}

	return &runtime.RawExtension{
		Object: state,
	}
}

func (fctx *FlowContext) persistState(ctx context.Context) error {
	nodesSubnetIPv6CIDR := fctx.whiteboard.Get(NodesSubnetIPv6CIDR)
	servicesSubnetIPv6CIDR := fctx.whiteboard.Get(ServicesSubnetIPv6CIDR)

	return PatchProviderStatusAndState(
		ctx,
		fctx.runtimeClient,
		fctx.infra,
		fctx.networking,
		nil,
		fctx.getCurrentState(),
		nodesSubnetIPv6CIDR,
		servicesSubnetIPv6CIDR,
	)
}

// PatchProviderStatusAndState computes and persists the infrastructure state
func PatchProviderStatusAndState(
	ctx context.Context,
	runtimeClient client.Client,
	infra *extensionsv1alpha1.Infrastructure,
	networking *gardencorev1beta1.Networking,
	status *v1alpha1.InfrastructureStatus,
	state *runtime.RawExtension,
	nodesSubnetIPv6CIDR *string,
	servicesSubnetIPv6CIDR *string,
) error {
	patch := client.MergeFrom(infra.DeepCopy())

	if status != nil {
		infra.Status.ProviderStatus = &runtime.RawExtension{Object: status}
		infra.Status.EgressCIDRs = make([]string, len(status.Networks.NatIPs))
		for i, natIP := range status.Networks.NatIPs {
			infra.Status.EgressCIDRs[i] = fmt.Sprintf("%s/32", natIP.IP)
		}

		infra.Status.Networking = &extensionsv1alpha1.InfrastructureStatusNetworking{}

		if networking != nil {
			if networking.Nodes != nil {
				infra.Status.Networking.Nodes = append(infra.Status.Networking.Nodes, *networking.Nodes)
			}
			if networking.Pods != nil {
				infra.Status.Networking.Pods = append(infra.Status.Networking.Pods, *networking.Pods)
			}
			if networking.Services != nil {
				infra.Status.Networking.Services = append(infra.Status.Networking.Services, *networking.Services)
			}
		}

		if nodesSubnetIPv6CIDR != nil {
			infra.Status.Networking.Nodes = append(infra.Status.Networking.Nodes, *nodesSubnetIPv6CIDR)
			infra.Status.Networking.Pods = append(infra.Status.Networking.Pods, *nodesSubnetIPv6CIDR)
			// Only add the IPv6 egress CIDR if we already have static IPv4 egress CIDRs.
			// While the IPv6 egress CIDR is always known for dual-stack clusters the IPv4 NAT IPs may be dynamic.
			// We try to be consistent and avoid the case where the IPv6 egress CIDR is present but the IPv4 NAT IPs are not.
			// Therefore, we do not add the IPv6 egress CIDR if the IPv4 NAT IPs are not present.
			if len(infra.Status.EgressCIDRs) > 0 {
				infra.Status.EgressCIDRs = append(infra.Status.EgressCIDRs, *nodesSubnetIPv6CIDR)
			}
		}

		if servicesSubnetIPv6CIDR != nil {
			infra.Status.Networking.Services = append(infra.Status.Networking.Services, getFirstSubnet(*servicesSubnetIPv6CIDR, 108))
		}
	}

	if state != nil {
		infra.Status.State = state
	}

	if data, err := patch.Data(infra); err != nil {
		return fmt.Errorf("failed getting patch data for infra %s: %w", infra.Name, err)
	} else if string(data) == `{}` {
		return nil
	}

	return runtimeClient.Status().Patch(ctx, infra, patch)
}

// getFirstSubnet extracts the first subnet of a specified size from a given CIDR block
// This is used to extract the /108 subnet from the /64 block as the /64 block is too large for --service-cluster-ip-range in cloud controller manager and kube-apiserver
// Remark: This is a workaround until the cloud controller manager and kube-apiserver supports /64 blocks
// or GCP supports Subnet CIDR reservations
func getFirstSubnet(cidr string, ones int) string {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return cidr
	}

	ipnet.Mask = net.CIDRMask(ones, len(ipnet.IP)*8)

	return ipnet.String()
}
