package infraflow

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow/shared"
	gcpinternal "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/infrastructure"
)

// FlowReconciler is capable of reconciling and deleting the infrastructure for a shoot.
type FlowReconciler struct {
	*shared.BasicFlowContext

	infra          *extensionsv1alpha1.Infrastructure
	config         *gcp.InfrastructureConfig
	updater        gcpclient.Updater
	serviceAccount *gcpinternal.ServiceAccount
	clusterName    string
	whiteboard     shared.Whiteboard
	podCIDR        *string

	computeClient gcpclient.ComputeClient
	iamClient     gcpclient.IAMClient
}

// NewFlowReconciler returns a new FlowReconciler.
func NewFlowReconciler(
	ctx context.Context,
	log logr.Logger,
	infra *extensionsv1alpha1.Infrastructure,
	cluster *controller.Cluster,
	c client.Client,
) (*FlowReconciler, error) {
	config, err := helper.InfrastructureConfigFromInfrastructure(infra)
	if err != nil {
		return nil, err
	}

	serviceAccount, err := gcpinternal.GetServiceAccountFromSecretReference(ctx, c, infra.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	gc := gcpclient.New()
	com, err := gc.Compute(ctx, c, infra.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	iam, err := gc.IAM(ctx, c, infra.Spec.SecretRef)
	if err != nil {
		return nil, err
	}

	wb := shared.NewWhiteboard()
	bfc := shared.NewBasicFlowContext(log, wb, nil)
	fr := &FlowReconciler{
		BasicFlowContext: bfc,
		whiteboard:       wb,
		infra:            infra,
		serviceAccount:   serviceAccount,
		config:           config,
		updater:          gcpclient.NewUpdater(gc, serviceAccount, log),
		clusterName:      cluster.ObjectMeta.Name,
		podCIDR:          cluster.Shoot.Spec.Networking.Pods,

		computeClient: com,
		iamClient:     iam,
	}

	return fr, nil
}

// Reconcile reconciles the infrastructure
func (c *FlowReconciler) Reconcile(ctx context.Context) (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	g := c.buildReconcileGraph()
	f := g.Compile()
	c.Log.Info("starting Flow Reconciliation")
	err := f.Run(ctx, flow.Opts{Log: c.Log})
	if err != nil {
		c.Log.Error(err, "flow reconciliation failed")
		status, state, inErr := c.getStatus()
		if inErr != nil {
			c.Log.Error(err, "flow reconciliation failed")
		}
		return status, state, err
	}

	return c.getStatus()
}

// Delete is used to destroy the infrastructure.
func (c *FlowReconciler) Delete(ctx context.Context) error {
	g := c.buildDeleteGraph()
	f := g.Compile()
	return f.Run(ctx, flow.Opts{Log: c.Log})
}

func (c *FlowReconciler) getStatus() (*v1alpha1.InfrastructureStatus, *runtime.RawExtension, error) {
	status := &v1alpha1.InfrastructureStatus{
		TypeMeta: infrastructure.StatusTypeMeta,
		Networks: v1alpha1.NetworkStatus{
			VPC:     v1alpha1.VPC{},
			Subnets: []v1alpha1.Subnet{},
			NatIPs:  []v1alpha1.NatIP{},
		},
	}

	if n := GetObject[*gcpclient.Network](c.whiteboard, ObjectKeyVPC); n != nil {
		status.Networks.VPC.Name = n.Name
	}

	if s := GetObject[*gcpclient.Subnetwork](c.whiteboard, ObjectKeyNodeSubnet); s != nil {
		status.Networks.Subnets = append(status.Networks.Subnets, v1alpha1.Subnet{
			Name:    s.Name,
			Purpose: v1alpha1.PurposeNodes,
		})
	}

	if s := GetObject[*gcpclient.Subnetwork](c.whiteboard, ObjectKeyInternalSubnet); s != nil {
		status.Networks.Subnets = append(status.Networks.Subnets, v1alpha1.Subnet{
			Name:    s.Name,
			Purpose: v1alpha1.PurposeInternal,
		})
	}

	if router := GetObject[*gcpclient.Router](c.whiteboard, ObjectKeyRouter); router != nil {
		status.Networks.VPC.CloudRouter = &v1alpha1.CloudRouter{
			Name: router.Name,
		}
	}

	if s := GetObject[*gcpclient.ServiceAccount](c.whiteboard, ObjectKeyServiceAccount); s != nil {
		status.ServiceAccountEmail = s.Email
	}

	bytes, err := NewFlowState().ToJSON()
	if err != nil {
		return nil, nil, err
	}

	state := &runtime.RawExtension{
		Raw: bytes,
	}
	return status, state, nil
}
