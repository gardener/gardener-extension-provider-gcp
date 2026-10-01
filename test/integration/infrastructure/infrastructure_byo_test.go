// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	computev1 "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	. "github.com/gardener/gardener-extension-provider-gcp/test/integration/infrastructure"
)

var _ = Describe("Infrastructure BYO subnet tests", func() {
	Context("with BYO subnet mode", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully reconcile and delete with BYO single-stack", func() {
			namespace, providerConfig := newProviderConfigBYO(false)
			err := runBYOTest(ctx, c, namespace, providerConfig, project, computeService, false)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully reconcile and delete with BYO dual-stack", func() {
			namespace, providerConfig := newProviderConfigBYODualStack()
			err := runBYOTest(ctx, c, namespace, providerConfig, project, computeService, true)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

const (
	byoWorkersSubnetCIDR  = "10.251.0.0/19"
	byoPodsCIDR           = "100.128.0.0/11"
	byoServicesSubnetCIDR = "192.168.255.0/29"
	byoPodRangeName       = "pods"
)

func newProviderConfigBYO(dualStack bool) (string, *gcpv1alpha1.InfrastructureConfig) {
	namespace, err := generateNamespaceName()
	Expect(err).NotTo(HaveOccurred())

	networkName := namespace
	workerSubnetName := namespace + "-nodes"

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		Expect(teardownBYONetwork(ctx, log, project, computeService, networkName, workerSubnetName, "")).NotTo(HaveOccurred())
		framework.RemoveCleanupAction(cleanupHandle)
	})

	Expect(prepareBYONetwork(ctx, log, project, computeService, networkName, workerSubnetName, "", dualStack)).To(Succeed())

	return namespace, &gcpv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Networks: gcpv1alpha1.NetworkConfig{
			VPC: &gcpv1alpha1.VPC{Name: networkName},
			SubnetWorkers: &gcpv1alpha1.SubnetReference{
				Name: workerSubnetName,
			},
		},
	}
}

func newProviderConfigBYODualStack() (string, *gcpv1alpha1.InfrastructureConfig) {
	namespace, err := generateNamespaceName()
	Expect(err).NotTo(HaveOccurred())

	networkName := namespace
	workerSubnetName := namespace + "-nodes"
	servicesSubnetName := namespace + "-services"

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		Expect(teardownBYONetwork(ctx, log, project, computeService, networkName, workerSubnetName, servicesSubnetName)).NotTo(HaveOccurred())
		framework.RemoveCleanupAction(cleanupHandle)
	})

	Expect(prepareBYONetwork(ctx, log, project, computeService, networkName, workerSubnetName, servicesSubnetName, true)).To(Succeed())

	return namespace, &gcpv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Networks: gcpv1alpha1.NetworkConfig{
			VPC: &gcpv1alpha1.VPC{Name: networkName},
			SubnetWorkers: &gcpv1alpha1.SubnetReference{
				Name:                  workerSubnetName,
				PodSecondaryRangeName: new(byoPodRangeName),
			},
			SubnetServices: &gcpv1alpha1.SubnetReference{
				Name: servicesSubnetName,
			},
		},
	}
}

func prepareBYONetwork(ctx context.Context, logger logr.Logger, project string, computeService *computev1.Service, networkName, workerSubnetName, servicesSubnetName string, dualStack bool) error {
	network := &computev1.Network{
		Name:                  networkName,
		AutoCreateSubnetworks: false,
		RoutingConfig:         &computev1.NetworkRoutingConfig{RoutingMode: "REGIONAL"},
		ForceSendFields:       []string{"AutoCreateSubnetworks"},
	}
	networkOp, err := computeService.Networks.Insert(project, network).Context(ctx).Do()
	if err != nil {
		return err
	}
	logger.Info("Waiting until BYO network is created", "network", networkName)
	if err := waitForOperation(ctx, project, computeService, networkOp); err != nil {
		return err
	}

	workerSubnet := &computev1.Subnetwork{
		Name:        workerSubnetName,
		Network:     networkOp.TargetLink,
		IpCidrRange: byoWorkersSubnetCIDR,
		Region:      *region,
	}
	if dualStack {
		workerSubnet.StackType = "IPV4_IPV6"
		workerSubnet.Ipv6AccessType = "EXTERNAL"
		workerSubnet.SecondaryIpRanges = []*computev1.SubnetworkSecondaryRange{
			{RangeName: byoPodRangeName, IpCidrRange: byoPodsCIDR},
		}
	}
	subnetOp, err := computeService.Subnetworks.Insert(project, *region, workerSubnet).Context(ctx).Do()
	if err != nil {
		return err
	}
	logger.Info("Waiting until BYO worker subnet is created", "subnet", workerSubnetName)
	if err := waitForOperation(ctx, project, computeService, subnetOp); err != nil {
		return err
	}

	if dualStack && servicesSubnetName != "" {
		svcSubnet := &computev1.Subnetwork{
			Name:           servicesSubnetName,
			Network:        networkOp.TargetLink,
			IpCidrRange:    byoServicesSubnetCIDR,
			Region:         *region,
			StackType:      "IPV4_IPV6",
			Ipv6AccessType: "EXTERNAL",
		}
		svcOp, err := computeService.Subnetworks.Insert(project, *region, svcSubnet).Context(ctx).Do()
		if err != nil {
			return err
		}
		logger.Info("Waiting until BYO services subnet is created", "subnet", servicesSubnetName)
		if err := waitForOperation(ctx, project, computeService, svcOp); err != nil {
			return err
		}
	}

	return nil
}

func teardownBYONetwork(ctx context.Context, logger logr.Logger, project string, computeService *computev1.Service, networkName, workerSubnetName, servicesSubnetName string) error {
	for _, subnetName := range []string{workerSubnetName, servicesSubnetName} {
		if subnetName == "" {
			continue
		}
		op, err := computeService.Subnetworks.Delete(project, *region, subnetName).Context(ctx).Do()
		if err != nil {
			if !gcpclient.IsErrorCode(err, http.StatusNotFound) {
				return err
			}
			logger.Info("BYO subnet already gone", "subnet", subnetName)
			continue
		}
		logger.Info("Waiting until BYO subnet is deleted", "subnet", subnetName)
		if err := waitForOperation(ctx, project, computeService, op); err != nil {
			return err
		}
	}

	networkOp, err := computeService.Networks.Delete(project, networkName).Context(ctx).Do()
	if err != nil {
		if gcpclient.IsErrorCode(err, http.StatusNotFound) {
			logger.Info("BYO network already gone", "network", networkName)
			return nil
		}
		return err
	}
	logger.Info("Waiting until BYO network is deleted", "network", networkName)
	return waitForOperation(ctx, project, computeService, networkOp)
}

func runBYOTest(
	ctx context.Context,
	c client.Client,
	namespaceName string,
	providerConfig *gcpv1alpha1.InfrastructureConfig,
	project string,
	computeService *computev1.Service,
	dualStack bool,
) error {
	var (
		namespace     *corev1.Namespace
		priorityClass *schedulingv1.PriorityClass
		cluster       *extensionsv1alpha1.Cluster
		infra         *extensionsv1alpha1.Infrastructure
	)

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		By("delete infrastructure")
		Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

		By("wait until infrastructure is deleted")
		err := extensions.WaitUntilExtensionObjectDeleted(
			ctx, c, log, infra, "Infrastructure", 10*time.Second, 16*time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())

		By("verify BYO infrastructure deletion")
		verifyBYODeletion(ctx, project, computeService, providerConfig)

		Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
		Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())

		framework.RemoveCleanupAction(cleanupHandle)
	})

	By("create namespace for test execution")
	namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
	if err := c.Create(ctx, namespace); err != nil {
		return err
	}

	By("create cluster")
	ipFamilies := []gardencorev1beta1.IPFamily{gardencorev1beta1.IPFamilyIPv4}
	if dualStack {
		ipFamilies = append(ipFamilies, gardencorev1beta1.IPFamilyIPv6)
	}
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Shoot",
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		},
		Spec: gardencorev1beta1.ShootSpec{
			Networking: &gardencorev1beta1.Networking{
				Nodes:      ptr.To(byoWorkersSubnetCIDR),
				Pods:       ptr.To(byoPodsCIDR),
				Services:   ptr.To(byoServicesSubnetCIDR),
				IPFamilies: ipFamilies,
			},
		},
	}
	shootJSON, err := json.Marshal(shoot)
	if err != nil {
		return err
	}
	cluster = &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: namespaceName},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
			Seed:         &runtime.RawExtension{Raw: []byte("{}")},
			Shoot:        runtime.RawExtension{Raw: shootJSON},
		},
	}
	if err := c.Create(ctx, cluster); err != nil {
		return err
	}

	priorityClass = &schedulingv1.PriorityClass{
		ObjectMeta:    metav1.ObjectMeta{Name: v1beta1constants.PriorityClassNameShootControlPlane300},
		Description:   "PriorityClass for Shoot control plane components",
		GlobalDefault: false,
		Value:         999998300,
	}
	if err := c.Create(ctx, priorityClass); client.IgnoreAlreadyExists(err) != nil {
		return err
	}

	By("deploy cloudprovider secret into namespace")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cloudprovider", Namespace: namespaceName},
		Data:       map[string][]byte{gcp.ServiceAccountJSONField: []byte(*serviceAccount)},
	}
	if err := c.Create(ctx, secret); err != nil {
		return err
	}

	By("create infrastructure")
	infra, err = newInfrastructure(namespaceName, providerConfig)
	if err != nil {
		return err
	}
	if err := c.Create(ctx, infra); err != nil {
		return err
	}

	By("wait until infrastructure is reconciled")
	if err := extensions.WaitUntilExtensionObjectReady(
		ctx, c, log, infra, extensionsv1alpha1.InfrastructureResource,
		10*time.Second, 30*time.Second, 16*time.Minute, nil,
	); err != nil {
		return err
	}

	By("verify BYO infrastructure creation")
	verifyBYOCreation(ctx, project, computeService, infra, providerConfig, dualStack)
	return nil
}

func verifyBYOCreation(
	ctx context.Context,
	project string,
	computeService *computev1.Service,
	infra *extensionsv1alpha1.Infrastructure,
	providerConfig *gcpv1alpha1.InfrastructureConfig,
	dualStack bool,
) {
	workerSubnetName := providerConfig.Networks.SubnetWorkers.Name

	// BYO VPC must still exist and must not have been modified
	_, err := computeService.Networks.Get(project, providerConfig.Networks.VPC.Name).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	// Worker subnet must still exist and must not have been modified
	workerSubnet, err := computeService.Subnetworks.Get(project, *region, workerSubnetName).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(workerSubnet.IpCidrRange).To(Equal(byoWorkersSubnetCIDR))

	// No managed router must have been created
	_, err = computeService.Routers.Get(project, *region, infra.Namespace+"-cloud-router").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())

	// No managed firewall rules must have been created
	_, err = computeService.Firewalls.Get(project, infra.Namespace+"-allow-internal-access").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())
	_, err = computeService.Firewalls.Get(project, infra.Namespace+"-allow-health-checks").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())

	// Infrastructure status must reference the BYO subnets
	Expect(infra.Status.EgressCIDRs).To(BeEmpty())

	if dualStack {
		Expect(infra.Status.Networking.Nodes).To(HaveLen(2))
		Expect(infra.Status.Networking.Services).To(HaveLen(2))

		// Services subnet must exist
		svcSubnetName := providerConfig.Networks.SubnetServices.Name
		svcSubnet, err := computeService.Subnetworks.Get(project, *region, svcSubnetName).Context(ctx).Do()
		Expect(err).NotTo(HaveOccurred())
		Expect(svcSubnet.ExternalIpv6Prefix).NotTo(BeEmpty())

		// Worker subnet must have an IPv6 prefix and the secondary pod range
		Expect(workerSubnet.ExternalIpv6Prefix).NotTo(BeEmpty())
		Expect(workerSubnet.SecondaryIpRanges).To(HaveLen(1))
		Expect(workerSubnet.SecondaryIpRanges[0].IpCidrRange).To(Equal(byoPodsCIDR))
	} else {
		Expect(infra.Status.Networking.Nodes).To(HaveLen(1))
	}
}

func verifyBYODeletion(
	ctx context.Context,
	project string,
	computeService *computev1.Service,
	providerConfig *gcpv1alpha1.InfrastructureConfig,
) {
	// BYO VPC and subnets must NOT have been deleted
	_, err := computeService.Networks.Get(project, providerConfig.Networks.VPC.Name).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	_, err = computeService.Subnetworks.Get(project, *region, providerConfig.Networks.SubnetWorkers.Name).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	if providerConfig.Networks.SubnetServices != nil {
		_, err = computeService.Subnetworks.Get(project, *region, providerConfig.Networks.SubnetServices.Name).Context(ctx).Do()
		Expect(err).NotTo(HaveOccurred())
	}
}
