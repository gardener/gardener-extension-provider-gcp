// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure_test

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gcpinstall "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
	gcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	. "github.com/gardener/gardener-extension-provider-gcp/test/integration/infrastructure"
)

const (
	workersSubnetCIDR  = "10.250.0.0/19"
	internalSubnetCIDR = "10.250.112.0/22"
)

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

	log         logr.Logger
	gardenerlog *logrus.Entry

	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	c         client.Client

	project        string
	computeService *compute.Service
	iamService     *iam.Service

	internalChartsPath string
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateFlags()

	internalChartsPath = gcp.InternalChartsPath
	repoRoot := filepath.Join("..", "..", "..")
	gcp.InternalChartsPath = filepath.Join(repoRoot, gcp.InternalChartsPath)

	runtimelog.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	log = runtimelog.Log.WithName("infrastructure-test")

	gardenerlogger := logrus.New()
	gardenerlogger.SetOutput(GinkgoWriter)
	gardenerlog = logrus.NewEntry(gardenerlogger)

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: pointer.BoolPtr(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-cluster.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-infrastructure.yaml"),
			},
		},
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	By("setup manager")
	mgr, err := manager.New(cfg, manager.Options{
		MetricsBindAddress: "0",
	})
	Expect(err).NotTo(HaveOccurred())

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed())
	Expect(gcpinstall.AddToScheme(mgr.GetScheme())).To(Succeed())

	Expect(infrastructure.AddToManager(mgr)).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred())
	}()

	c = mgr.GetClient()
	Expect(c).NotTo(BeNil())

	project, err = gcp.ExtractServiceAccountProjectID([]byte(*serviceAccount))
	Expect(err).NotTo(HaveOccurred())
	computeService, err = compute.NewService(ctx, option.WithCredentialsJSON([]byte(*serviceAccount)), option.WithScopes(compute.CloudPlatformScope))
	Expect(err).NotTo(HaveOccurred())
	iamService, err = iam.NewService(ctx, option.WithCredentialsJSON([]byte(*serviceAccount)))
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	defer func() {
		By("stopping manager")
		mgrCancel()
	}()

	By("running cleanup actions")
	framework.RunCleanupActions()

	By("stopping test environment")
	Expect(testEnv.Stop()).To(Succeed())

	gcp.InternalChartsPath = internalChartsPath
})

var _ = Describe("Infrastructure tests", func() {
	Context("with infrastructure that requests new vpc", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			providerConfig := newProviderConfig(nil)

			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			err = runTest(ctx, c, namespace, providerConfig, project, computeService, iamService)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with infrastructure that uses existing vpc", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			networkName := namespace
			cloudRouterName := networkName + "-cloud-router"

			err = prepareNewNetwork(ctx, log, project, computeService, networkName, cloudRouterName)
			Expect(err).NotTo(HaveOccurred())

			var cleanupHandle framework.CleanupActionHandle
			cleanupHandle = framework.AddCleanupAction(func() {
				err := teardownNetwork(ctx, log, project, computeService, networkName, cloudRouterName)
				Expect(err).NotTo(HaveOccurred())

				framework.RemoveCleanupAction(cleanupHandle)
			})

			providerConfig := newProviderConfig(&gcpv1alpha1.VPC{
				Name: networkName,
				CloudRouter: &gcpv1alpha1.CloudRouter{
					Name: cloudRouterName,
				},
			})

			err = runTest(ctx, c, namespace, providerConfig, project, computeService, iamService)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

func runTest(
	ctx context.Context,
	c client.Client,
	namespaceName string,
	providerConfig *gcpv1alpha1.InfrastructureConfig,
	project string,
	computeService *compute.Service,
	iamService *iam.Service,
) error {
	var (
		namespace *corev1.Namespace
		cluster   *extensionsv1alpha1.Cluster
		infra     *extensionsv1alpha1.Infrastructure
	)

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		By("delete infrastructure")
		Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

		By("wait until infrastructure is deleted")
		err := extensions.WaitUntilExtensionObjectDeleted(
			ctx,
			c,
			gardenerlog,
			infra,
			"Infrastructure",
			10*time.Second,
			16*time.Minute,
		)
		Expect(err).NotTo(HaveOccurred())

		By("verify infrastructure deletion")
		verifyDeletion(ctx, project, computeService, iamService, infra, providerConfig)

		Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
		Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())

		framework.RemoveCleanupAction(cleanupHandle)
	})

	By("create namespace for test execution")
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	if err := c.Create(ctx, namespace); err != nil {
		return err
	}

	By("create cluster")
	cluster = &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	if err := c.Create(ctx, cluster); err != nil {
		return err
	}

	By("deploy cloudprovider secret into namespace")
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudprovider",
			Namespace: namespaceName,
		},
		Data: map[string][]byte{
			gcp.ServiceAccountJSONField: []byte(*serviceAccount),
		},
	}
	if err := c.Create(ctx, secret); err != nil {
		return err
	}

	By("create infrastructure")
	infra, err := newInfrastructure(namespaceName, providerConfig)
	if err != nil {
		return err
	}

	if err := c.Create(ctx, infra); err != nil {
		return err
	}

	By("wait until infrastructure is created")
	if err := extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		gardenerlog,
		infra,
		extensionsv1alpha1.InfrastructureResource,
		10*time.Second,
		30*time.Second,
		16*time.Minute,
		nil,
	); err != nil {
		return err
	}

	By("verify infrastructure creation")
	verifyCreation(ctx, project, computeService, iamService, infra, providerConfig)

	return nil
}

func newProviderConfig(vpc *gcpv1alpha1.VPC) *gcpv1alpha1.InfrastructureConfig {
	return &gcpv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Networks: gcpv1alpha1.NetworkConfig{
			VPC:      vpc,
			Workers:  workersSubnetCIDR,
			Internal: pointer.StringPtr(internalSubnetCIDR),
			FlowLogs: &gcpv1alpha1.FlowLogs{
				AggregationInterval: pointer.StringPtr("INTERVAL_5_SEC"),
				FlowSampling:        pointer.Float32Ptr(0.2),
				Metadata:            pointer.StringPtr("INCLUDE_ALL_METADATA"),
			},
		},
	}
}

func newInfrastructure(namespace string, providerConfig *gcpv1alpha1.InfrastructureConfig) (*extensionsv1alpha1.Infrastructure, error) {
	const sshPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== your_email@example.com"

	providerConfigJSON, err := json.Marshal(&providerConfig)
	if err != nil {
		return nil, err
	}

	return &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infrastructure",
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.InfrastructureSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: gcp.Type,
				ProviderConfig: &runtime.RawExtension{
					Raw: providerConfigJSON,
				},
			},
			SecretRef: corev1.SecretReference{
				Name:      "cloudprovider",
				Namespace: namespace,
			},
			Region:       *region,
			SSHPublicKey: []byte(sshPublicKey),
		},
	}, nil
}

func generateNamespaceName() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return "gcp-infrastructure-it--" + suffix, nil
}

func prepareNewNetwork(ctx context.Context, logger logr.Logger, project string, computeService *compute.Service, networkName, routerName string) error {
	logger = logger.WithValues("project", project)

	network := &compute.Network{
		Name:                  networkName,
		AutoCreateSubnetworks: false,
		RoutingConfig: &compute.NetworkRoutingConfig{
			RoutingMode: "REGIONAL",
		},
		ForceSendFields: []string{"AutoCreateSubnetworks"},
	}
	networkOp, err := computeService.Networks.Insert(project, network).Context(ctx).Do()
	if err != nil {
		return err
	}
	logger.Info("Waiting until network is created...", "network", networkName)
	if err := waitForOperation(ctx, project, computeService, networkOp); err != nil {
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
	logger.Info("Waiting until router is created...", "router", routerName)
	if err := waitForOperation(ctx, project, computeService, routerOp); err != nil {
		return err
	}

	return nil
}

func teardownNetwork(ctx context.Context, logger logr.Logger, project string, computeService *compute.Service, networkName, routerName string) error {
	logger = logger.WithValues("project", project)

	routerOp, err := computeService.Routers.Delete(project, *region, routerName).Context(ctx).Do()
	if err != nil {
		return err
	}

	logger.Info("Waiting until router is deleted...", "router", routerName)
	if err := waitForOperation(ctx, project, computeService, routerOp); err != nil {
		return err
	}

	networkOp, err := computeService.Networks.Delete(project, networkName).Context(ctx).Do()
	if err != nil {
		return err
	}

	logger.Info("Waiting until network is deleted...", "network", networkName)
	if err := waitForOperation(ctx, project, computeService, networkOp); err != nil {
		return err
	}

	return nil
}

func waitForOperation(ctx context.Context, project string, computeService *compute.Service, op *compute.Operation) error {
	return wait.PollUntil(5*time.Second, func() (bool, error) {
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
	}, ctx.Done())
}

func getResourceNameFromSelfLink(link string) string {
	parts := strings.Split(link, "/")
	return parts[len(parts)-1]
}

func verifyCreation(
	ctx context.Context,
	project string,
	computeService *compute.Service,
	iamService *iam.Service,
	infra *extensionsv1alpha1.Infrastructure,
	providerConfig *gcpv1alpha1.InfrastructureConfig,
) {
	// service account

	serviceAccountName := getServiceAccountName(project, infra.Namespace)
	serviceAccount, err := iamService.Projects.ServiceAccounts.Get(serviceAccountName).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(serviceAccount.DisplayName).To(Equal(infra.Namespace))

	// network

	network, err := computeService.Networks.Get(project, infra.Namespace).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(network.AutoCreateSubnetworks).To(BeFalse())
	Expect(network.Subnetworks).To(HaveLen(2))

	// subnets

	subnetNodes, err := computeService.Subnetworks.Get(project, *region, infra.Namespace+"-nodes").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(subnetNodes.Network).To(Equal(network.SelfLink))
	Expect(subnetNodes.IpCidrRange).To(Equal(providerConfig.Networks.Workers))
	Expect(subnetNodes.LogConfig.Enable).To(BeTrue())
	Expect(subnetNodes.LogConfig.AggregationInterval).To(Equal("INTERVAL_5_SEC"))
	Expect(subnetNodes.LogConfig.FlowSampling).To(Equal(float64(0.2)))
	Expect(subnetNodes.LogConfig.Metadata).To(Equal("INCLUDE_ALL_METADATA"))

	subnetInternal, err := computeService.Subnetworks.Get(project, *region, infra.Namespace+"-internal").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(subnetInternal.Network).To(Equal(network.SelfLink))
	Expect(subnetInternal.IpCidrRange).To(Equal(internalSubnetCIDR))

	// router

	router, err := computeService.Routers.Get(project, *region, infra.Namespace+"-cloud-router").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(router.Network).To(Equal(network.SelfLink))
	Expect(router.Nats).To(HaveLen(1))

	routerNAT := router.Nats[0]
	Expect(routerNAT.Name).To(Equal(infra.Namespace + "-cloud-nat"))
	Expect(routerNAT.NatIpAllocateOption).To(Equal("AUTO_ONLY"))
	Expect(routerNAT.SourceSubnetworkIpRangesToNat).To(Equal("LIST_OF_SUBNETWORKS"))
	Expect(routerNAT.MinPortsPerVm).To(Equal(int64(2048)))
	Expect(routerNAT.LogConfig.Enable).To(BeTrue())
	Expect(routerNAT.LogConfig.Filter).To(Equal("ERRORS_ONLY"))
	Expect(routerNAT.Subnetworks).To(HaveLen(1))
	Expect(routerNAT.Subnetworks[0].Name).To(Equal(subnetNodes.SelfLink))
	Expect(routerNAT.Subnetworks[0].SourceIpRangesToNat).To(Equal([]string{"ALL_IP_RANGES"}))

	// firewalls

	allowInternalAccess, err := computeService.Firewalls.Get(project, infra.Namespace+"-allow-internal-access").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	Expect(allowInternalAccess.Network).To(Equal(network.SelfLink))
	Expect(allowInternalAccess.SourceRanges).To(HaveLen(2))
	Expect(allowInternalAccess.SourceRanges).To(ConsistOf(workersSubnetCIDR, internalSubnetCIDR))
	Expect(allowInternalAccess.Allowed).To(ConsistOf([]*compute.FirewallAllowed{
		{
			IPProtocol: "icmp",
		},
		{
			IPProtocol: "ipip",
		},
		{
			IPProtocol: "tcp",
			Ports:      []string{"1-65535"},
		},
		{
			IPProtocol: "udp",
			Ports:      []string{"1-65535"},
		},
	}))

	allowExternalAccess, err := computeService.Firewalls.Get(project, infra.Namespace+"-allow-external-access").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	Expect(allowExternalAccess.Network).To(Equal(network.SelfLink))
	Expect(allowExternalAccess.SourceRanges).To(Equal([]string{"0.0.0.0/0"}))
	Expect(allowExternalAccess.Allowed).To(ConsistOf([]*compute.FirewallAllowed{
		{
			IPProtocol: "tcp",
			Ports:      []string{"80", "443"},
		},
	}))

	allowHealthChecks, err := computeService.Firewalls.Get(project, infra.Namespace+"-allow-health-checks").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	Expect(allowHealthChecks.Network).To(Equal(network.SelfLink))
	Expect(allowHealthChecks.SourceRanges).To(ConsistOf([]string{
		"35.191.0.0/16",
		"209.85.204.0/22",
		"209.85.152.0/22",
		"130.211.0.0/22",
	}))
	Expect(allowHealthChecks.Allowed).To(ConsistOf([]*compute.FirewallAllowed{
		{
			IPProtocol: "tcp",
			Ports:      []string{"30000-32767"},
		},
		{
			IPProtocol: "udp",
			Ports:      []string{"30000-32767"},
		},
	}))
}

func verifyDeletion(
	ctx context.Context,
	project string,
	computeService *compute.Service,
	iamService *iam.Service,
	infra *extensionsv1alpha1.Infrastructure,
	providerConfig *gcpv1alpha1.InfrastructureConfig,
) {
	// service account

	serviceAccountName := getServiceAccountName(project, infra.Namespace)
	_, err := iamService.Projects.ServiceAccounts.Get(serviceAccountName).Context(ctx).Do()
	Expect(err).To(BeNotFoundError())

	// network

	if providerConfig.Networks.VPC == nil {
		_, err = computeService.Networks.Get(project, infra.Namespace).Context(ctx).Do()
		Expect(err).To(BeNotFoundError())
	}

	// subnets

	_, err = computeService.Subnetworks.Get(project, *region, infra.Namespace+"-nodes").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())

	_, err = computeService.Subnetworks.Get(project, *region, infra.Namespace+"-internal").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())

	// router

	if providerConfig.Networks.VPC == nil || providerConfig.Networks.VPC.CloudRouter == nil {
		_, err = computeService.Routers.Get(project, *region, infra.Namespace+"-cloud-router").Context(ctx).Do()
		Expect(err).To(BeNotFoundError())
	}

	// firewalls

	_, err = computeService.Firewalls.Get(project, infra.Namespace+"-allow-internal-access").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())

	_, err = computeService.Firewalls.Get(project, infra.Namespace+"-allow-external-access").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())

	_, err = computeService.Firewalls.Get(project, infra.Namespace+"-allow-health-checks").Context(ctx).Do()
	Expect(err).To(BeNotFoundError())
}

func getServiceAccountName(project, displayName string) string {
	return fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", project, displayName, project)
}
