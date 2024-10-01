// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/go-logr/logr"
	"google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

// bastionEndpoints collects the endpoints the bastion host provides; the
// private endpoint is important for opening a port on the worker node
// ingress firewall rule to allow SSH from that node, the public endpoint is where
// the end user connects to establish the SSH connection.
type bastionEndpoints struct {
	private *corev1.LoadBalancerIngress
	public  *corev1.LoadBalancerIngress
}

// Ready returns true if both public and private interfaces each have either
// an IP or a hostname or both.
func (be *bastionEndpoints) Ready() bool {
	return be != nil && IngressReady(be.private) && IngressReady(be.public)
}

func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) error {
	credentialsConfig, err := getCredentialsConfig(ctx, a.client, bastion)
	if err != nil {
		return fmt.Errorf("failed to get service account: %w", err)
	}

	secretReference := corev1.SecretReference{
		Namespace: cluster.ObjectMeta.Name,
		Name:      v1beta1constants.SecretNameCloudProvider,
	}

	gcpClient, err := gcpclient.New(a.tokenMetadataURL, a.tokenMetadataClient).Compute(ctx, a.client, secretReference)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to create GCP client: %w", err), helper.KnownCodes)
	}

	infrastructureStatus, subnet, err := getInfrastructureStatus(ctx, a.client, cluster)
	if err != nil {
		return err
	}

	opt, err := DetermineOptions(bastion, cluster, credentialsConfig.ProjectID, infrastructureStatus.Networks.VPC.Name, subnet)
	if err != nil {
		return fmt.Errorf("failed to determine Options: %w", err)
	}

	if opt.Zone == "" {
		opt.Zone, err = getDefaultGCPZone(ctx, gcpClient, cluster.Shoot.Spec.Region)
		if err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	bytes, err := json.Marshal(&providerStatusRaw{Zone: opt.Zone})
	if err != nil {
		return err
	}

	patch := client.MergeFrom(bastion.DeepCopy())
	bastion.Status.ProviderStatus = &runtime.RawExtension{Raw: bytes}
	if err := a.client.Status().Patch(ctx, bastion, patch); err != nil {
		return fmt.Errorf("failed to store status.providerStatus for zone: %s", opt.Zone)
	}

	err = ensureFirewallRules(ctx, log, gcpClient, bastion, opt)
	if err != nil {
		return util.DetermineError(fmt.Errorf("failed to ensure firewall rule: %w", err), helper.KnownCodes)
	}

	instance, err := ensureComputeInstance(ctx, log, bastion, gcpClient, opt)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	// check if the instance already exists and has an IP
	endpoints, err := getInstanceEndpoints(instance)
	if err != nil {
		return err
	}

	if !endpoints.Ready() {
		return &reconcilerutils.RequeueAfterError{
			// requeue rather soon, so that the user (most likely gardenctl eventually)
			// doesn't have to wait too long for the public endpoint to become available
			RequeueAfter: 5 * time.Second,
			Cause:        fmt.Errorf("bastion instance has no public/private endpoints yet"),
		}
	}

	// once a public endpoint is available, publish the endpoint on the
	// Bastion resource to notify upstream about the ready instance
	patch = client.MergeFrom(bastion.DeepCopy())
	bastion.Status.Ingress = endpoints.public
	return a.client.Status().Patch(ctx, bastion, patch)
}

func ensureFirewallRules(ctx context.Context, log logr.Logger, client gcpclient.ComputeClient, bastion *extensionsv1alpha1.Bastion, opt *Options) error {
	cidrs, err := ingressPermissions(bastion)
	if err != nil {
		return err
	}

	firewallList := []*compute.Firewall{IngressAllowSSH(opt, cidrs), EgressDenyAll(opt), EgressAllowOnly(opt)}

	for _, item := range firewallList {
		if err := createFirewallRuleIfNotExist(ctx, log, client, item); err != nil {
			return err
		}
	}

	firewall, err := client.GetFirewallRule(ctx, IngressAllowSSH(opt, cidrs).Name)
	if err != nil || firewall == nil {
		return fmt.Errorf("could not get firewall rule: %w", err)
	}

	currentCIDRs := firewall.SourceRanges
	wantedCIDRs := cidrs

	if !reflect.DeepEqual(currentCIDRs, wantedCIDRs) {
		return patchFirewallRule(ctx, client, IngressAllowSSH(opt, cidrs).Name, cidrs)
	}

	return nil
}

func ensureComputeInstance(ctx context.Context, logger logr.Logger, bastion *extensionsv1alpha1.Bastion, client gcpclient.ComputeClient, opt *Options) (*compute.Instance, error) {
	instance, err := getBastionInstance(ctx, client, opt)
	if instance != nil || err != nil {
		return instance, err
	}

	logger.Info("Creating new bastion compute instance")
	computeInstance := computeInstanceDefine(opt, bastion.Spec.UserData)
	_, err = client.InsertInstance(ctx, opt.Zone, computeInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to create bastion compute instance: %w", err)
	}

	instance, err = getBastionInstance(ctx, client, opt)
	if instance != nil || err != nil {
		return instance, err
	}

	return nil, fmt.Errorf("failed to get (create) bastion compute instance: %w", err)
}

func getInstanceEndpoints(instance *compute.Instance) (*bastionEndpoints, error) {
	if instance == nil {
		return nil, fmt.Errorf("compute instance can't be nil")
	}

	if instance.Status != "RUNNING" {
		return nil, fmt.Errorf("instance not running, status: %s", instance.Status)
	}

	endpoints := &bastionEndpoints{}

	networkInterfaces := instance.NetworkInterfaces

	if len(networkInterfaces) == 0 {
		return nil, fmt.Errorf("no network interfaces found: %s", instance.Name)
	}

	internalIP := &networkInterfaces[0].NetworkIP

	if len(networkInterfaces[0].AccessConfigs) == 0 {
		return nil, fmt.Errorf("no access config found for network interface: %s", instance.Name)
	}

	externalIP := &networkInterfaces[0].AccessConfigs[0].NatIP

	if ingress := addressToIngress(&instance.Name, internalIP); ingress != nil {
		endpoints.private = ingress
	}

	// GCP does not automatically assign a public dns name to the instance (in contrast to e.g. AWS).
	// As we provide an externalIP to connect to the bastion, having a public dns name would just be an alternative way to connect to the bastion.
	// Out of this reason, we spare the effort to create a PTR record (see https://cloud.google.com/compute/docs/instances/create-ptr-record#api) just for the sake of having it.
	if ingress := addressToIngress(nil, externalIP); ingress != nil {
		endpoints.public = ingress
	}

	return endpoints, nil
}

// IngressReady returns true if either an IP or a hostname or both are set.
func IngressReady(ingress *corev1.LoadBalancerIngress) bool {
	return ingress != nil && (ingress.Hostname != "" || ingress.IP != "")
}

// addressToIngress converts the IP address into a corev1.LoadBalancerIngress resource.
// If both arguments are nil, then nil is returned.
func addressToIngress(dnsName *string, ipAddress *string) *corev1.LoadBalancerIngress {
	var ingress *corev1.LoadBalancerIngress

	if ipAddress != nil || dnsName != nil {
		ingress = &corev1.LoadBalancerIngress{}
		if dnsName != nil {
			ingress.Hostname = *dnsName
		}

		if ipAddress != nil {
			ingress.IP = *ipAddress
		}
	}

	return ingress
}

func computeInstanceDefine(opt *Options, userData []byte) *compute.Instance {
	return &compute.Instance{
		Disks:              disksDefine(opt),
		DeletionProtection: false,
		Description:        "Bastion Instance",
		Name:               opt.BastionInstanceName,
		Zone:               opt.Zone,
		MachineType:        machineTypeDefine(opt),
		NetworkInterfaces:  networkInterfacesDefine(opt),
		Tags:               &compute.Tags{Items: []string{opt.BastionInstanceName}},
		Metadata:           &compute.Metadata{Items: metadataItemsDefine(userData)},
	}
}

func metadataItemsDefine(userData []byte) []*compute.MetadataItems {
	return []*compute.MetadataItems{
		{
			Key:   "startup-script",
			Value: ptr.To(string(userData)),
		},
		{
			Key:   "block-project-ssh-keys",
			Value: ptr.To("TRUE"),
		},
	}
}

func machineTypeDefine(opt *Options) string {
	return fmt.Sprintf("zones/%s/machineTypes/%s", opt.Zone, opt.MachineName)
}

func networkInterfacesDefine(opt *Options) []*compute.NetworkInterface {
	return []*compute.NetworkInterface{
		{
			Network:       opt.Network,
			Subnetwork:    opt.Subnetwork,
			AccessConfigs: []*compute.AccessConfig{{Name: "External NAT", Type: "ONE_TO_ONE_NAT"}},
		},
	}
}

func disksDefine(opt *Options) []*compute.AttachedDisk {
	return []*compute.AttachedDisk{
		{
			AutoDelete: true,
			Boot:       true,
			DiskSizeGb: 10,
			Mode:       "READ_WRITE",
			InitializeParams: &compute.AttachedDiskInitializeParams{
				DiskName:    opt.DiskName,
				Description: "Gardenctl Bastion disk",
				SourceImage: opt.ImagePath,
			},
		},
	}
}
