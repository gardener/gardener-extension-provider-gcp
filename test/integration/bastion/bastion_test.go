// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gcpinstall "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
	gcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	bastionctrl "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/bastion"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var workersSubnetCIDR = "10.250.0.0/16"
var userDataConst = "IyEvYmluL2Jhc2ggLWV1CmlkIGdhcmRlbmVyIHx8IHVzZXJhZGQgZ2FyZGVuZXIgLW1VCm1rZGlyIC1wIC9ob21lL2dhcmRlbmVyLy5zc2gKZWNobyAic3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFDazYyeDZrN2orc0lkWG9TN25ITzRrRmM3R0wzU0E2UmtMNEt4VmE5MUQ5RmxhcmtoRzFpeU85WGNNQzZqYnh4SzN3aWt0M3kwVTBkR2h0cFl6Vjh3YmV3Z3RLMWJBWnl1QXJMaUhqbnJnTFVTRDBQazNvWGh6RkpKN0MvRkxNY0tJZFN5bG4vMENKVkVscENIZlU5Y3dqQlVUeHdVQ2pnVXRSYjdZWHN6N1Y5dllIVkdJKzRLaURCd3JzOWtVaTc3QWMyRHQ1UzBJcit5dGN4b0p0bU5tMWgxTjNnNzdlbU8rWXhtWEo4MzFXOThoVFVTeFljTjNXRkhZejR5MWhrRDB2WHE1R1ZXUUtUQ3NzRE1wcnJtN0FjQTBCcVRsQ0xWdWl3dXVmTEJLWGhuRHZRUEQrQ2Jhbk03bUZXRXdLV0xXelZHME45Z1VVMXE1T3hhMzhvODUgbWVAbWFjIiA+IC9ob21lL2dhcmRlbmVyLy5zc2gvYXV0aG9yaXplZF9rZXlzCmNob3duIGdhcmRlbmVyOmdhcmRlbmVyIC9ob21lL2dhcmRlbmVyLy5zc2gvYXV0aG9yaXplZF9rZXlzCmVjaG8gImdhcmRlbmVyIEFMTD0oQUxMKSBOT1BBU1NXRDpBTEwiID4vZXRjL3N1ZG9lcnMuZC85OS1nYXJkZW5lci11c2VyCg=="
var myPublicIP = ""

var (
	serviceAccount = flag.String("service-account", "", "Service account containing credentials for the GCP API")
	region         = flag.String("region", "", "GCP region")
)

func validateFlags() {
	if len(*serviceAccount) == 0 {
		panic("--service-account flag is not specified")
	}
	if len(*region) == 0 {
		panic("--region flag is not specified")
	}
}

var (
	ctx = context.Background()

	log            logr.Logger
	project        string
	computeService *compute.Service

	extensionscluster *extensionsv1alpha1.Cluster
	worker            *extensionsv1alpha1.Worker
	controllercluster *controller.Cluster

	secret    *corev1.Secret
	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	c         client.Client
	options   *bastionctrl.Options
	bastion   *extensionsv1alpha1.Bastion

	name       string
	vNetName   string
	routerName string
	subnetName string
)

var _ = BeforeSuite(func() {
	repoRoot := filepath.Join("..", "..", "..")

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))

	log = logf.Log.WithName("bastion-test")

	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("stopping test environment")
		Expect(testEnv.Stop()).To(Succeed())
	})

	By("generating randomized test resource identifiers")
	randString, err := randomString()
	Expect(err).NotTo(HaveOccurred())

	name = fmt.Sprintf("gcp-bastion-it--%s", randString)
	vNetName = name
	routerName = vNetName + "-cloud-router"
	subnetName = vNetName + "-nodes"

	myPublicIP, err = getMyPublicIPWithMask()
	Expect(err).ToNot(HaveOccurred())

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_bastions.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_workers.yaml"),
			},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	By("setup manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: server.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(gcpinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(bastionctrl.AddToManager(ctx, mgr)).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred())
	}()

	// test client should be uncached and independent from the tested manager
	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(c).NotTo(BeNil())

	flag.Parse()
	validateFlags()

	sa, err := gcp.GetServiceAccountFromJSON([]byte(*serviceAccount))
	project = sa.ProjectID
	Expect(err).NotTo(HaveOccurred())
	computeService, err = compute.NewService(ctx, option.WithCredentialsJSON([]byte(*serviceAccount)), option.WithScopes(compute.CloudPlatformScope))
	Expect(err).NotTo(HaveOccurred())

	extensionscluster, controllercluster = createClusters(name)
	worker = createWorker(name, vNetName, subnetName)
	bastion, options = createBastion(controllercluster, name, project, vNetName, subnetName)

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudprovider",
			Namespace: name,
		},
		Data: map[string][]byte{
			gcp.ServiceAccountJSONField: []byte(*serviceAccount),
		},
	}

})

var _ = Describe("Bastion tests", func() {
	It("should successfully create and delete", func() {
		By("setup Infrastructure")
		err := prepareNewNetwork(ctx, log, project, computeService, vNetName, routerName, subnetName)
		Expect(err).NotTo(HaveOccurred())
		framework.AddCleanupAction(func() {
			err = teardownNetwork(ctx, log, project, computeService, vNetName, routerName, subnetName)
			Expect(err).NotTo(HaveOccurred())
		})

		By("create namespace for test execution")
		setupEnvironmentObjects(ctx, c, namespace(name), secret, extensionscluster, worker)
		framework.AddCleanupAction(func() {
			teardownShootEnvironment(ctx, c, namespace(name), secret, extensionscluster, worker)
		})

		By("setup bastion")
		err = c.Create(ctx, bastion)
		Expect(err).NotTo(HaveOccurred())

		framework.AddCleanupAction(func() {
			teardownBastion(ctx, log, c, bastion)

			By("verify bastion deletion")
			verifyDeletion(ctx, project, computeService, options)
		})

		By("wait until bastion is reconciled")
		Expect(extensions.WaitUntilExtensionObjectReady(
			ctx,
			c,
			log,
			bastion,
			extensionsv1alpha1.BastionResource,
			15*time.Second,
			60*time.Second,
			5*time.Minute,
			nil,
		)).To(Succeed())

		time.Sleep(60 * time.Second)
		verifyPort22IsOpen(ctx, c, bastion)
		verifyPort42IsClosed(ctx, c, bastion)

		By("verify cloud resources")
		verifyCreation(ctx, project, computeService, options)
	})
})

func verifyPort22IsOpen(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("check connection to port 22 open should not error")
	bastionUpdated := &extensionsv1alpha1.Bastion{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: bastion.Namespace, Name: bastion.Name}, bastionUpdated)).To(Succeed())

	ipAddress := bastionUpdated.Status.Ingress.IP
	address := net.JoinHostPort(ipAddress, "22")
	conn, err := net.DialTimeout("tcp", address, 60*time.Second)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(conn).NotTo(BeNil())
}

func verifyPort42IsClosed(ctx context.Context, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("check connection to port 42 which should fail")

	bastionUpdated := &extensionsv1alpha1.Bastion{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: bastion.Namespace, Name: bastion.Name}, bastionUpdated)).To(Succeed())

	ipAddress := bastionUpdated.Status.Ingress.IP
	address := net.JoinHostPort(ipAddress, "42")
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	Expect(err).Should(HaveOccurred())
	Expect(conn).To(BeNil())
}

func prepareNewNetwork(ctx context.Context, log logr.Logger, project string, computeService *compute.Service, networkName string, routerName string, subnetName string) error {

	network := &compute.Network{
		Name:                  networkName,
		AutoCreateSubnetworks: false,
		RoutingConfig: &compute.NetworkRoutingConfig{
			RoutingMode: "REGIONAL",
		},
		Subnetworks:     []string{subnetName},
		ForceSendFields: []string{"AutoCreateSubnetworks"},
	}
	networkOp, err := computeService.Networks.Insert(project, network).Context(ctx).Do()
	if err != nil {
		return err
	}
	log.Info("Waiting until network is created...", "network ", networkName)
	if err := waitForOperation(ctx, project, computeService, networkOp); err != nil {
		return err
	}

	subnet := &compute.Subnetwork{
		Name:           subnetName,
		Region:         *region,
		Network:        fmt.Sprintf("projects/%s/global/networks/%s", project, networkName),
		IpCidrRange:    "10.250.0.0/16",
		GatewayAddress: "10.250.0.1",
		EnableFlowLogs: false,
	}

	resubnetOp, err := computeService.Subnetworks.Insert(project, *region, subnet).Context(ctx).Do()
	log.Info("Waiting until subnet is created...", "subnet ", networkName+"-nodes")
	if err != nil {
		return err
	}
	if err := waitForOperation(ctx, project, computeService, resubnetOp); err != nil {
		return err
	}

	router := &compute.Router{
		Name:    routerName,
		Network: networkOp.TargetLink,
	}
	routerOp, err := computeService.Routers.Insert(project, *region, router).Context(ctx).Do()
	if err != nil {
		return err
	}
	log.Info("Waiting until router is created...", "router ", routerName)

	err = waitForOperation(ctx, project, computeService, routerOp)
	if err != nil {
		return err
	}

	return nil
}

func teardownNetwork(ctx context.Context, log logr.Logger, project string, computeService *compute.Service, networkName string, routerName string, subnetName string) error {

	routerOp, err := computeService.Routers.Delete(project, *region, routerName).Context(ctx).Do()
	if err != nil {
		return err
	}

	log.Info("Waiting until router is deleted...", "router ", routerName)
	if err := waitForOperation(ctx, project, computeService, routerOp); err != nil {
		return err
	}

	subnetOp, err := computeService.Subnetworks.Delete(project, *region, subnetName).Context(ctx).Do()
	if err != nil {
		return err
	}

	log.Info("Waiting until subnet is deleted...", "subnet ", subnetName)
	if err := waitForOperation(ctx, project, computeService, subnetOp); err != nil {
		return err
	}

	networkOp, err := computeService.Networks.Delete(project, networkName).Context(ctx).Do()
	if err != nil {
		return err
	}

	log.Info("Waiting until network is deleted...", "network ", networkName)

	err = waitForOperation(ctx, project, computeService, networkOp)
	if err != nil {
		return err
	}

	return nil
}

func waitForOperation(ctx context.Context, project string, computeService *compute.Service, op *compute.Operation) error {
	return wait.PollUntilContextCancel(ctx, 5*time.Second, false, func(_ context.Context) (bool, error) {
		var (
			currentOp *compute.Operation
			err       error
		)

		if op.Region != "" {
			region := getResourceNameFromSelfLink(op.Region)
			currentOp, err = computeService.RegionOperations.Get(project, region, op.Name).Context(ctx).Do()
		} else {
			currentOp, err = computeService.GlobalOperations.Get(project, op.Name).Context(ctx).Do()
		}

		if err != nil {
			return false, err
		}
		return currentOp.Status == "DONE", nil
	})
}

func getResourceNameFromSelfLink(link string) string {
	parts := strings.Split(link, "/")
	return parts[len(parts)-1]
}

func createBastion(cluster *controller.Cluster, name, project, vNet, subnet string) (*extensionsv1alpha1.Bastion, *bastionctrl.Options) {
	bastion := &extensionsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-bastion",
			Namespace: name,
		},
		Spec: extensionsv1alpha1.BastionSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: gcp.Type,
			},
			UserData: []byte(userDataConst),
			Ingress: []extensionsv1alpha1.BastionIngressPolicy{
				{IPBlock: networkingv1.IPBlock{
					CIDR: myPublicIP,
				}},
			},
		},
	}

	options, err := bastionctrl.DetermineOptions(bastion, cluster, project, vNet, subnet)
	Expect(err).NotTo(HaveOccurred())

	return bastion, options
}

func createInfrastructureConfig() *gcpv1alpha1.InfrastructureConfig {
	return &gcpv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Networks: gcpv1alpha1.NetworkConfig{
			Workers: workersSubnetCIDR,
		},
	}
}

func createWorker(name, vNetName, subnetName string) *extensionsv1alpha1.Worker {
	infrastructureProviderStatus := createInfrastructureStatus(vNetName, subnetName)
	json, err := json.Marshal(infrastructureProviderStatus)
	Expect(err).NotTo(HaveOccurred())

	return &extensionsv1alpha1.Worker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
		Spec: extensionsv1alpha1.WorkerSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: gcp.Type,
			},
			InfrastructureProviderStatus: &runtime.RawExtension{
				Raw: json,
			},
			Pools: []extensionsv1alpha1.WorkerPool{},
			SecretRef: corev1.SecretReference{
				Name:      "secret",
				Namespace: name,
			},
			Region: *region,
		},
	}
}

func createInfrastructureStatus(vNetName, subnetName string) *gcpv1alpha1.InfrastructureStatus {
	return &gcpv1alpha1.InfrastructureStatus{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureStatus",
		},
		Networks: gcpv1alpha1.NetworkStatus{
			VPC: gcpv1alpha1.VPC{
				Name: vNetName,
			},
			Subnets: []gcpv1alpha1.Subnet{
				{
					Purpose: "fake",
					Name:    subnetName,
				},
				{
					Purpose: gcpv1alpha1.PurposeNodes,
					Name:    subnetName,
				},
			},
		},
	}
}

func createShoot(infrastructureConfig []byte) *gardencorev1beta1.Shoot {
	return &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "core.gardener.cloud/v1beta1",
			Kind:       "Shoot",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},

		Spec: gardencorev1beta1.ShootSpec{
			Region: *region,
			Provider: gardencorev1beta1.Provider{
				InfrastructureConfig: &runtime.RawExtension{
					Raw: infrastructureConfig,
				}},
		},
	}
}

func createCloudProfile() *gardencorev1beta1.CloudProfile {
	cloudProfile := &gardencorev1beta1.CloudProfile{
		Spec: gardencorev1beta1.CloudProfileSpec{
			Regions: []gardencorev1beta1.Region{
				{Name: *region},
				{Name: *region, Zones: []gardencorev1beta1.AvailabilityZone{
					{Name: *region + "-b"},
					{Name: *region + "-c"},
				}},
			},
		},
	}
	return cloudProfile
}

func createClusters(name string) (*extensionsv1alpha1.Cluster, *controller.Cluster) {
	infrastructureConfig := createInfrastructureConfig()
	infrastructureConfigJSON, _ := json.Marshal(&infrastructureConfig)

	shoot := createShoot(infrastructureConfigJSON)
	shootJSON, _ := json.Marshal(shoot)

	cloudProfile := createCloudProfile()
	cloudProfileJSON, _ := json.Marshal(cloudProfile)

	extensionscluster := &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{
				Object: cloudProfile,
				Raw:    cloudProfileJSON,
			},
			Seed: runtime.RawExtension{
				Raw: []byte("{}"),
			},
			Shoot: runtime.RawExtension{
				Object: shoot,
				Raw:    shootJSON,
			},
		},
	}

	cluster := &controller.Cluster{
		ObjectMeta:   metav1.ObjectMeta{Name: name},
		Shoot:        shoot,
		CloudProfile: cloudProfile,
	}
	return extensionscluster, cluster
}

func teardownBastion(ctx context.Context, log logr.Logger, c client.Client, bastion *extensionsv1alpha1.Bastion) {
	By("delete bastion")
	Expect(client.IgnoreNotFound(c.Delete(ctx, bastion))).To(Succeed())

	By("wait until bastion is deleted")
	err := extensions.WaitUntilExtensionObjectDeleted(ctx, c, log, bastion, extensionsv1alpha1.BastionResource, 10*time.Second, 16*time.Minute)
	Expect(err).NotTo(HaveOccurred())
}

func verifyCreation(ctx context.Context, project string, computeService *compute.Service, options *bastionctrl.Options) {
	By("checkFirewallExists")
	// bastion firewall - Check Ingress / Egress firewalls created
	checkFirewallExists(ctx, project, computeService, bastionctrl.FirewallIngressAllowSSHResourceName(options.BastionInstanceName))
	checkFirewallExists(ctx, project, computeService, bastionctrl.FirewallEgressAllowOnlyResourceName(options.BastionInstanceName))
	checkFirewallExists(ctx, project, computeService, bastionctrl.FirewallEgressDenyAllResourceName(options.BastionInstanceName))

	By("checking Firewall-allow-ssh rule SSHPortOpen,Public Source Ranges")
	firewall, err := computeService.Firewalls.Get(project, bastionctrl.FirewallIngressAllowSSHResourceName(options.BastionInstanceName)).Context(ctx).Do()
	Expect(ignoreNotFoundError(err)).NotTo(HaveOccurred())
	Expect(firewall.Allowed[0].Ports[0]).To(Equal("22"))
	Expect(firewall.SourceRanges[0]).To(Equal(myPublicIP))

	By("checking Firewall-deny-all rule")
	firewall, err = computeService.Firewalls.Get(project, bastionctrl.FirewallEgressDenyAllResourceName(options.BastionInstanceName)).Context(ctx).Do()
	Expect(ignoreNotFoundError(err)).NotTo(HaveOccurred())
	Expect(firewall.Denied[0].IPProtocol).To(Equal("all"))
	Expect(firewall.DestinationRanges[0]).To(Equal("0.0.0.0/0"))

	By("checking Firewall-egress-worker rule")
	firewall, err = computeService.Firewalls.Get(project, bastionctrl.FirewallEgressAllowOnlyResourceName(options.BastionInstanceName)).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(firewall.DestinationRanges[0]).To(Equal(options.WorkersCIDR))

	By("checking bastion instance")
	// bastion instance
	createdInstance, err := computeService.Instances.Get(project, options.Zone, options.BastionInstanceName).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(createdInstance.Name).To(Equal(options.BastionInstanceName))

	By("checking bastion ingress IPs exist")
	// bastion ingress IPs exist
	networkInterfaces := createdInstance.NetworkInterfaces
	internalIP := &networkInterfaces[0].NetworkIP
	externalIP := &networkInterfaces[0].AccessConfigs[0].NatIP
	Expect(internalIP).NotTo(BeNil())
	Expect(externalIP).NotTo(BeNil())

	By("checking bastion disks exists")
	// bastion Disk exists
	createdDisk, err := computeService.Disks.Get(project, options.Zone, bastionctrl.DiskResourceName(options.BastionInstanceName)).Context(ctx).Do()
	Expect(ignoreNotFoundError(err)).NotTo(HaveOccurred())
	Expect(createdDisk.Name).To(Equal(bastionctrl.DiskResourceName(options.BastionInstanceName)))

	By("checking userData matches the constant")
	// userdata ssh-public-key validation
	Expect(*createdInstance.Metadata.Items[0].Value).To(Equal(userDataConst))
}

func verifyDeletion(ctx context.Context, project string, computeService *compute.Service, options *bastionctrl.Options) {
	// bastion firewalls should be gone
	// Check Firewall for Ingress / Egress
	checkFirewallDoesNotExist(ctx, project, computeService, bastionctrl.FirewallIngressAllowSSHResourceName(options.BastionInstanceName))
	checkFirewallDoesNotExist(ctx, project, computeService, bastionctrl.FirewallEgressAllowOnlyResourceName(options.BastionInstanceName))
	checkFirewallDoesNotExist(ctx, project, computeService, bastionctrl.FirewallEgressDenyAllResourceName(options.BastionInstanceName))

	// instance should be terminated and not found
	_, err := computeService.Instances.Get(project, options.Zone, options.BastionInstanceName).Context(ctx).Do()
	Expect(ignoreNotFoundError(err)).NotTo(HaveOccurred())

	// Disk should be terminated and not found
	_, err = computeService.Disks.Get(project, options.Zone, bastionctrl.DiskResourceName(options.BastionInstanceName)).Context(ctx).Do()
	Expect(ignoreNotFoundError(err)).NotTo(HaveOccurred())
}

func checkFirewallDoesNotExist(ctx context.Context, project string, computeService *compute.Service, firewallName string) {
	_, err := computeService.Firewalls.Get(project, firewallName).Context(ctx).Do()
	Expect(ignoreNotFoundError(err)).NotTo(HaveOccurred())
}

func checkFirewallExists(ctx context.Context, project string, computeService *compute.Service, firewallName string) {
	firewall, err := computeService.Firewalls.Get(project, firewallName).Context(ctx).Do()
	Expect(ignoreNotFoundError(err)).NotTo(HaveOccurred())
	Expect(firewall.Name).To(Equal(firewallName))
}

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func setupEnvironmentObjects(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster, worker *extensionsv1alpha1.Worker) {
	Expect(c.Create(ctx, namespace)).To(Succeed())
	Expect(c.Create(ctx, cluster)).To(Succeed())
	Expect(c.Create(ctx, secret)).To(Succeed())
	Expect(c.Create(ctx, worker)).To(Succeed())
}

func teardownShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster, worker *extensionsv1alpha1.Worker) {
	workerCopy := worker.DeepCopy()
	metav1.SetMetaDataAnnotation(&worker.ObjectMeta, "confirmation.gardener.cloud/deletion", "true")
	Expect(c.Patch(ctx, worker, client.MergeFrom(workerCopy))).To(Succeed())

	Expect(client.IgnoreNotFound(c.Delete(ctx, worker))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
	Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
}

func getMyPublicIPWithMask() (string, error) {
	resp, err := http.Get("https://api.ipify.org")

	if err != nil {
		return "", err
	}

	defer func() {
		err := resp.Body.Close()
		if err != nil {
			Expect(err).NotTo(HaveOccurred())
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := net.ParseIP(string(body))
	var mask net.IPMask
	if ip.To4() != nil {
		mask = net.CIDRMask(24, 32) // use a /24 net for IPv4
	} else {
		return "", fmt.Errorf("not valid IPv4 address")
	}

	cidr := net.IPNet{
		IP:   ip,
		Mask: mask,
	}

	full := cidr.String()

	_, ipnet, _ := net.ParseCIDR(full)

	return ipnet.String(), nil
}

func randomString() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return suffix, nil
}

func ignoreNotFoundError(err error) error {
	if err == nil {
		return nil
	}
	if googleError, ok := err.(*googleapi.Error); ok && googleError.Code == http.StatusNotFound {
		return nil
	}
	return err
}
