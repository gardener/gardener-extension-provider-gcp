package infraflow

import (
	"context"
	"strings"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
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
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

const (

	// CreatedServiceAccountKey marks whether we have created a service account for the shoot. If not we will skip reconciling the service accounts
	CreatedServiceAccountKey = "service_account_exist"
	// CreatedResourcesExistKey is a marker for the Terraform migration case. If the TF state is not empty
	// we inject this marker into the state to block the deletion without having first a successful reconciliation.
	CreatedResourcesExistKey = "resources_exist"
	// ChildKeyIDs is the prefix key for all ids.
	ChildKeyIDs = "ids"
	// KeyServiceAccountEmail is the key to store the service account object.
	KeyServiceAccountEmail = "service-account-email"
	// ObjectKeyVPC is the key to store the VPC object.
	ObjectKeyVPC = "vpc"
	// ObjectKeyNodeSubnet is the key to store the nodes subnet object.
	ObjectKeyNodeSubnet = "subnet-nodes"
	// ObjectKeyInternalSubnet is the key to store the internal subnet object.
	ObjectKeyInternalSubnet = "subnet-internal"
	// ObjectKeyRouter router is the key for the CloudRouter.
	ObjectKeyRouter = "router"
	// ObjectKeyNAT is the key for the .CloudNAT object.
	ObjectKeyNAT = "nat"
	// ObjectKeyIPAddress is the key for the IP Address slice.
	ObjectKeyIPAddress = "addresses/ip"

	defaultWaiterPeriod time.Duration = 5 * time.Second
)

var (
	// DefaultUpdaterFunc is the default constructor used for an Updated used in the package.
	DefaultUpdaterFunc = gcpclient.NewUpdater
)

// PersistStateFunc is a callback function that is used to persist the state during the reconciliation.
type PersistStateFunc func(ctx context.Context, state *runtime.RawExtension) error

// FlowContext is capable of reconciling and deleting the infrastructure for a shoot.
type FlowContext struct {
	bfg *shared.BasicFlowContext

	infra          *extensionsv1alpha1.Infrastructure
	config         *gcp.InfrastructureConfig
	state          *gcp.InfrastructureState
	updater        gcpclient.Updater
	serviceAccount *gcpinternal.ServiceAccount
	clusterName    string
	whiteboard     shared.Whiteboard
	podCIDR        *string
	persistFn      PersistStateFunc
	log            logr.Logger

	computeClient gcpclient.ComputeClient
	iamClient     gcpclient.IAMClient
	*shared.BasicFlowContext
}

// Opts contains the options to initialize a FlowContext.
type Opts struct {
	// Log is the logger using during the reconciliation.
	Log logr.Logger
	// Infra
	Infra          *extensionsv1alpha1.Infrastructure
	State          *gcp.InfrastructureState
	Cluster        *controller.Cluster
	ServiceAccount *gcpinternal.ServiceAccount
	Factory        gcpclient.Factory
	Client         client.Client
	PersistFunc    PersistStateFunc
}

// NewFlowContext returns a new FlowContext.
func NewFlowContext(ctx context.Context, opts Opts) (*FlowContext, error) {
	wb := shared.NewWhiteboard()
	wb.ImportFromFlatMap(opts.State.Data)
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
		whiteboard:     wb,
		infra:          opts.Infra,
		serviceAccount: opts.ServiceAccount,
		config:         config,
		state:          opts.State,
		updater:        DefaultUpdaterFunc(opts.Log, com),
		clusterName:    opts.Cluster.ObjectMeta.Name,
		podCIDR:        opts.Cluster.Shoot.Spec.Networking.Pods,
		persistFn:      opts.PersistFunc,
		log:            opts.Log,

		computeClient: com,
		iamClient:     iam,
	}

	return fr, nil
}

// Reconcile reconciles the infrastructure
func (fctx *FlowContext) Reconcile(ctx context.Context) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	g := fctx.buildReconcileGraph()
	f := g.Compile()
	fctx.log.Info("starting Flow Reconciliation")
	err := f.Run(ctx, flow.Opts{
		Log: fctx.log,
	})
	if err != nil {
		err = flow.Causes(err)
		fctx.log.Error(err, "flow reconciliation failed")
		return nil, fctx.getCurrentState(), err
	}

	status := fctx.getStatus()
	state := fctx.getCurrentState()
	return status, state, nil
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
		TypeMeta: infrastructure.StatusTypeMeta,
		Networks: v1alpha1.NetworkStatus{
			VPC:     v1alpha1.VPC{},
			Subnets: []v1alpha1.Subnet{},
			NatIPs:  []v1alpha1.NatIP{},
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

	if router := GetObject[*compute.Router](fctx.whiteboard, ObjectKeyRouter); router != nil {
		status.Networks.VPC.CloudRouter = &v1alpha1.CloudRouter{
			Name: router.Name,
		}
	}

	status.ServiceAccountEmail = ptr.Deref(fctx.whiteboard.GetChild(ChildKeyIDs).Get(KeyServiceAccountEmail), "")
	return status
}

func (fctx *FlowContext) getCurrentState() *runtime.RawExtension {
	state := &v1alpha1.InfrastructureState{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureState",
		},
		Data: fctx.whiteboard.ExportAsFlatMap(),
		// Data: map[string]string{
		//
		// 	CreatedResourcesExistKey: ptr.Deref(fctx.whiteboard.Get(CreatedResourcesExistKey), ""),
		// },
	}

	return &runtime.RawExtension{
		Object: state,
	}
}

func (fctx *FlowContext) persistState(ctx context.Context) error {
	if fctx.persistFn == nil {
		return nil
	}

	return fctx.persistFn(ctx, fctx.getCurrentState())
}

func (fctx *FlowContext) loadWhiteBoard() {
	if fctx.whiteboard == nil {
		return
	}
	if fctx.state == nil {
		return
	}

	fctx.whiteboard.Set(CreatedResourcesExistKey, "true")
}
