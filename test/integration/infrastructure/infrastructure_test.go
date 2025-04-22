// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	computev1 "google.golang.org/api/compute/v1"
	iamv1 "google.golang.org/api/iam/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	schemev1 "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gcpinstall "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
	gcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/features"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	. "github.com/gardener/gardener-extension-provider-gcp/test/integration/infrastructure"
)

const (
	workersSubnetCIDR  = "10.250.0.0/19"
	internalSubnetCIDR = "10.250.112.0/22"
	podCIDR            = "100.96.0.0/11"
	subnetCIDR         = "100.128.0.0/11"

	reconcilerUseTF     string = "tf"
	reconcilerMigrateTF string = "migrate"
	reconcilerUseFlow   string = "flow"
)

var (
	serviceAccount = flag.String("service-account", "", "Service account containing credentials for the GCP API")
	region         = flag.String("region", "", "GCP region")
	reconciler     = flag.String("reconciler", reconcilerUseTF, "Set annotation to use flow for reconciliation")
	testId         = string(uuid.NewUUID())
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

	log logr.Logger

	testEnv   *envtest.Environment
	mgrCancel context.CancelFunc
	c         client.Client

	project        string
	computeService *computev1.Service
	iamService     *iamv1.Service
)

var _ = BeforeSuite(func() {
	flag.Parse()
	validateFlags()

	repoRoot := filepath.Join("..", "..", "..")

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

	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName("infrastructure-test").WithValues("test-id", testId)

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_infrastructures.yaml"),
			},
		},
	}
	restConfig, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(restConfig).NotTo(BeNil())

	httpClient, err := rest.HTTPClientFor(restConfig)
	Expect(err).NotTo(HaveOccurred())
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig, httpClient)
	Expect(err).NotTo(HaveOccurred())

	scheme := runtime.NewScheme()
	Expect(schemev1.AddToScheme(scheme)).To(Succeed())
	Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
	Expect(gcpinstall.AddToScheme(scheme)).To(Succeed())

	By("setup manager")
	mgr, err := manager.New(restConfig, manager.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		Cache: cache.Options{
			Mapper: mapper,
			ByObject: map[client.Object]cache.ByObject{
				&extensionsv1alpha1.Infrastructure{}: {
					Label: labels.SelectorFromSet(labels.Set{"test-id": testId}),
				},
			},
		},
	})
	Expect(err).ToNot(HaveOccurred())

	Expect(infrastructure.AddToManagerWithOptions(ctx, mgr, infrastructure.AddOptions{
		// During testing in testmachinery cluster, there is no gardener-resource-manager to inject the volume mount.
		// Hence, we need to run without projected token mount.
		DisableProjectedTokenMount: true,
		Controller: controller.Options{
			MaxConcurrentReconciles: 5,
		},
	})).To(Succeed())

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	By("start manager")
	go func() {
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred())
	}()

	// test client should be uncached and independent of the tested manager.
	c, err = client.New(restConfig, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(c).NotTo(BeNil())

	sa, err := gcp.GetCredentialsConfigFromJSON([]byte(*serviceAccount))
	project = sa.ProjectID
	Expect(err).NotTo(HaveOccurred())
	computeService, err = computev1.NewService(ctx, option.WithCredentialsJSON([]byte(*serviceAccount)), option.WithScopes(computev1.CloudPlatformScope))
	Expect(err).NotTo(HaveOccurred())
	iamService, err = iamv1.NewService(ctx, option.WithCredentialsJSON([]byte(*serviceAccount)))
	Expect(err).NotTo(HaveOccurred())
})

var _ = Describe("Infrastructure tests", func() {
	Context("with infrastructure that requests new vpc", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			providerConfig := newProviderConfig(nil, nil)

			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			err = runTest(ctx, c, namespace, providerConfig, project, computeService, iamService, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with infrastructure that uses existing vpc, cloud router and cloud nat", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should successfully create and delete", func() {
			namespace, providerConfig := newProviderConfigForExistingVPC()
			err := runTest(ctx, c, namespace, providerConfig, project, computeService, iamService, false)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with invalid credentials", func() {
		Context("during create", func() {
			It("should successfully create and delete", func() {
				if *reconciler != reconcilerUseFlow {
					Skip("test is not working for terraform because the state is not exactly empty")
				}
				providerConfig := newProviderConfig(nil, nil)
				var (
					namespace *corev1.Namespace
					cluster   *extensionsv1alpha1.Cluster
					infra     *extensionsv1alpha1.Infrastructure
				)

				framework.AddCleanupAction(func() {
					By("cleaning up namespace and cluster")
					Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
					Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
				})

				defer func() {
					By("delete infrastructure")
					Expect(client.IgnoreNotFound(c.Delete(ctx, infra))).To(Succeed())

					By("wait until infrastructure is deleted")
					err := extensions.WaitUntilExtensionObjectDeleted(
						ctx,
						c,
						log,
						infra,
						extensionsv1alpha1.InfrastructureResource,
						10*time.Second,
						1*time.Minute,
					)
					Expect(err).ToNot(HaveOccurred())
				}()

				namespaceName, err := generateNamespaceName()
				Expect(err).NotTo(HaveOccurred())

				By("create namespace for test execution")
				namespace = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespaceName,
					},
				}
				Expect(c.Create(ctx, namespace)).To(Succeed())

				var sa map[string]interface{}
				Expect(json.Unmarshal([]byte(*serviceAccount), &sa)).NotTo(HaveOccurred())
				sa["private_key_id"] = "fake"
				sa["project_id"] = "fake"
				fake_sa, err := json.Marshal(sa)
				Expect(err).NotTo(HaveOccurred())

				By("deploy cloudprovider secret into namespace")
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cloudprovider",
						Namespace: namespaceName,
					},
					Data: map[string][]byte{
						gcp.ServiceAccountJSONField: fake_sa,
					},
				}
				if err := c.Create(ctx, secret); err != nil {
					Expect(err).ToNot(HaveOccurred())
				}

				cluster, err = newCluster(namespaceName)
				Expect(err).ToNot(HaveOccurred())
				Expect(c.Create(ctx, cluster)).To(Succeed())

				By("create infrastructure")
				infra, err = newInfrastructure(namespaceName, *reconciler, providerConfig)
				Expect(err).NotTo(HaveOccurred())
				Expect(c.Create(ctx, infra)).To(Succeed())

				By("wait until infrastructure creation has failed")
				err = extensions.WaitUntilExtensionObjectReady(
					ctx,
					c,
					log,
					infra,
					extensionsv1alpha1.InfrastructureResource,
					10*time.Second,
					30*time.Second,
					30*time.Second,
					nil,
				)
				var errorWithCode *gardencorev1beta1helper.ErrorWithCodes
				Expect(errors.As(err, &errorWithCode)).To(BeTrue())
				Expect(errorWithCode.Codes()).To(ContainElement(gardencorev1beta1.ErrorInfraUnauthorized))
			})
		})
	})

	Context("with dualstack enabled", func() {
		AfterEach(func() {
			framework.RunCleanupActions()
		})

		It("should create VPC and subnets with dualstack enabled", func() {
			if *reconciler != reconcilerUseFlow {
				Skip("dualstack support is not implemented for terraform")
			}
			providerConfig := newProviderConfig(nil, nil)

			namespace, err := generateNamespaceName()
			Expect(err).NotTo(HaveOccurred())

			err = runTest(ctx, c, namespace, providerConfig, project, computeService, iamService, true)
			Expect(err).NotTo(HaveOccurred())

		})

		It("dualstack enabled with infrastructure that uses existing vpc", func() {
			if *reconciler != reconcilerUseFlow {
				Skip("dualstack support is not implemented for terraform")
			}
			namespace, providerConfig := newProviderConfigForExistingVPC()
			err := runTest(ctx, c, namespace, providerConfig, project, computeService, iamService, true)
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
	computeService *computev1.Service,
	iamService *iamv1.Service,
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
			ctx,
			c,
			log,
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

	ipFamilies := []gardencorev1beta1.IPFamily{
		gardencorev1beta1.IPFamilyIPv4,
	}

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
				Pods:       ptr.To(podCIDR),
				Services:   ptr.To(subnetCIDR),
				IPFamilies: ipFamilies,
			},
		},
	}

	shootJSON, err := json.Marshal(shoot)
	if err != nil {
		return err
	}
	cluster = &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
			Seed:         runtime.RawExtension{Raw: []byte("{}")},
			Shoot:        runtime.RawExtension{Raw: shootJSON},
		},
	}
	if err := c.Create(ctx, cluster); err != nil {
		return err
	}
	priorityClass = &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.PriorityClassNameShootControlPlane300,
		},
		Description:   "PriorityClass for Shoot control plane components",
		GlobalDefault: false,
		Value:         999998300,
	}
	if err := c.Create(ctx, priorityClass); client.IgnoreAlreadyExists(err) != nil {
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
	infra, err = newInfrastructure(namespaceName, *reconciler, providerConfig)
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
		log,
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
	verifyCreation(ctx, project, computeService, iamService, infra, providerConfig, dualStack)

	if *reconciler == reconcilerMigrateTF {
		By("verifying terraform migration")
		infraCopy := infra.DeepCopy()
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, gcp.AnnotationKeyUseFlow, "true")
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		Expect(c.Patch(ctx, infra, client.MergeFrom(infraCopy))).To(Succeed())

		By("wait until infrastructure is reconciled")
		if err := extensions.WaitUntilExtensionObjectReady(
			ctx,
			c,
			log,
			infra,
			"Infrastructure",
			10*time.Second,
			30*time.Second,
			16*time.Minute,
			nil,
		); err != nil {
			return err
		}

		By("verify infrastructure creation after migration")
		verifyCreation(ctx, project, computeService, iamService, infra, providerConfig, dualStack)
	}

	return err
}

// verify that the subnets have IPv6 CIDR ranges, firewalls allow IPv6 traffic, etc.
func verifyDualStackSetup(ctx context.Context, project string, computeService *computev1.Service, namespace string) {
	subnetNodes, err := computeService.Subnetworks.Get(project, *region, namespace+"-nodes").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(subnetNodes.ExternalIpv6Prefix).ToNot(BeEmpty(), "Expected IPv6 CIDR to be set for nodes subnet")

	subnetServices, err := computeService.Subnetworks.Get(project, *region, namespace+"-services").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(subnetServices.ExternalIpv6Prefix).ToNot(BeEmpty(), "Expected IPv6 CIDR to be set for services subnet")

	fwRules, err := computeService.Firewalls.List(project).Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	firewallRules := map[string]bool{}
	for _, fw := range fwRules.Items {
		firewallRules[fw.Name] = true
	}

	Expect(
		firewallRules[infraflow.FirewallRuleAllowHealthChecksNameIPv6(namespace)],
	).To(BeTrue(), "Missing firewall rule to allow IPv6 health check")

	Expect(
		firewallRules[infraflow.FirewallRuleAllowInternalNameIPv6(namespace)],
	).To(BeTrue(), "Missing firewall rule to allow IPv6 internal access")
}

func newProviderConfig(vpc *gcpv1alpha1.VPC, cloudNAT *gcpv1alpha1.CloudNAT) *gcpv1alpha1.InfrastructureConfig {
	return &gcpv1alpha1.InfrastructureConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureConfig",
		},
		Networks: gcpv1alpha1.NetworkConfig{
			VPC:      vpc,
			CloudNAT: cloudNAT,
			Workers:  workersSubnetCIDR,
			Internal: ptr.To(internalSubnetCIDR),
			FlowLogs: &gcpv1alpha1.FlowLogs{
				AggregationInterval: ptr.To("INTERVAL_5_SEC"),
				FlowSampling:        ptr.To[float32](0.2),
				Metadata:            ptr.To("INCLUDE_ALL_METADATA"),
			},
		},
	}
}

func newInfrastructure(namespace string, reconciler string, providerConfig *gcpv1alpha1.InfrastructureConfig) (*extensionsv1alpha1.Infrastructure, error) {
	const sshPublicKey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDcSZKq0lM9w+ElLp9I9jFvqEFbOV1+iOBX7WEe66GvPLOWl9ul03ecjhOf06+FhPsWFac1yaxo2xj+SJ+FVZ3DdSn4fjTpS9NGyQVPInSZveetRw0TV0rbYCFBTJuVqUFu6yPEgdcWq8dlUjLqnRNwlelHRcJeBfACBZDLNSxjj0oUz7ANRNCEne1ecySwuJUAz3IlNLPXFexRT0alV7Nl9hmJke3dD73nbeGbQtwvtu8GNFEoO4Eu3xOCKsLw6ILLo4FBiFcYQOZqvYZgCb4ncKM52bnABagG54upgBMZBRzOJvWp0ol+jK3Em7Vb6ufDTTVNiQY78U6BAlNZ8Xg+LUVeyk1C6vWjzAQf02eRvMdfnRCFvmwUpzbHWaVMsQm8gf3AgnTUuDR0ev1nQH/5892wZA86uLYW/wLiiSbvQsqtY1jSn9BAGFGdhXgWLAkGsd/E1vOT+vDcor6/6KjHBm0rG697A3TDBRkbXQ/1oFxcM9m17RteCaXuTiAYWMqGKDoJvTMDc4L+Uvy544pEfbOH39zfkIYE76WLAFPFsUWX6lXFjQrX3O7vEV73bCHoJnwzaNd03PSdJOw+LCzrTmxVezwli3F9wUDiBRB0HkQxIXQmncc1HSecCKALkogIK+1e1OumoWh6gPdkF4PlTMUxRitrwPWSaiUIlPfCpQ== your_email@example.com"

	providerConfigJSON, err := json.Marshal(&providerConfig)
	if err != nil {
		return nil, err
	}

	infra := &extensionsv1alpha1.Infrastructure{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "infrastructure",
			Namespace: namespace,
			Labels: map[string]string{
				"test-id": testId,
			},
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
	}

	switch reconciler {
	case reconcilerUseFlow:
		log.Info("creating infrastructure with flow annotation")
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, gcp.AnnotationKeyUseFlow, "true")
	case reconcilerUseTF:
		log.Info("creating infrastructure with terraform annotation")
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, gcp.AnnotationKeyUseFlow, "false")
	}
	return infra, nil
}

func generateNamespaceName() (string, error) {
	suffix, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	if err != nil {
		return "", err
	}

	return "gcp-infrastructure-it--" + suffix, nil
}

func prepareNewNetwork(ctx context.Context, logger logr.Logger, project string, computeService *computev1.Service, networkName, routerName string) error {
	logger = logger.WithValues("project", project)

	network := &computev1.Network{
		Name:                  networkName,
		AutoCreateSubnetworks: false,
		RoutingConfig: &computev1.NetworkRoutingConfig{
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

	router := &computev1.Router{
		Name:    routerName,
		Network: networkOp.TargetLink,
	}
	routerOp, err := computeService.Routers.Insert(project, *region, router).Context(ctx).Do()
	if err != nil {
		return err
	}
	logger.Info("Waiting until router is created...", "router", routerName)
	return waitForOperation(ctx, project, computeService, routerOp)
}

func teardownNetwork(ctx context.Context, logger logr.Logger, project string, computeService *computev1.Service, networkName, routerName string) error {
	logger = logger.WithValues("project", project)

	routerOp, err := computeService.Routers.Delete(project, *region, routerName).Context(ctx).Do()
	if err != nil {
		if !gcpclient.IsErrorCode(err, http.StatusNotFound) {
			return err
		}
		logger.Info("The router is gone", "router", routerName)
	} else {
		logger.Info("Waiting until router is deleted...", "router", routerName)
		if err = waitForOperation(ctx, project, computeService, routerOp); err != nil {
			return err
		}
	}

	networkOp, err := computeService.Networks.Delete(project, networkName).Context(ctx).Do()
	if err != nil {
		if gcpclient.IsErrorCode(err, http.StatusNotFound) {
			return err
		}
		logger.Info("The network is gone", "network", networkName)
	} else {
		logger.Info("Waiting until network is deleted...", "network", networkName)
		if err = waitForOperation(ctx, project, computeService, networkOp); err != nil {
			return err
		}
	}

	return nil
}

func prepareNewIPAddresses(ctx context.Context, logger logr.Logger, project string, computeService *computev1.Service, ipAddressNames []string) error {
	logger = logger.WithValues("project", project)
	for _, ipAddressName := range ipAddressNames {
		address := &computev1.Address{
			Name: ipAddressName,
		}
		insertAddressOp, err := computeService.Addresses.Insert(project, *region, address).Context(ctx).Do()
		if err != nil {
			return err
		}
		logger.Info("Waiting until ip address is reserved...", "address", ipAddressName)
		if err = waitForOperation(ctx, project, computeService, insertAddressOp); err != nil {
			return err
		}
	}
	return nil
}

func teardownIPAddresses(ctx context.Context, logger logr.Logger, project string, computeService *computev1.Service, ipAddressNames []string) error {
	logger = logger.WithValues("project", project)
	for _, ipAddressName := range ipAddressNames {
		deleteAddressOp, err := computeService.Addresses.Delete(project, *region, ipAddressName).Context(ctx).Do()
		if err != nil {
			if gcpclient.IsErrorCode(err, http.StatusNotFound) {
				logger.Info("The ip address is gone", "address", ipAddressName)
				continue
			}
			return err
		}
		logger.Info("Waiting until ip address is released...", "address", ipAddressName)
		if err = waitForOperation(ctx, project, computeService, deleteAddressOp); err != nil {
			return err
		}
	}
	return nil
}

func waitForOperation(ctx context.Context, project string, computeService *computev1.Service, op *computev1.Operation) error {
	return wait.PollUntilContextCancel(ctx, 5*time.Second, false, func(_ context.Context) (bool, error) {
		var (
			currentOp *computev1.Operation
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

func verifyCreation(
	ctx context.Context,
	project string,
	computeService *computev1.Service,
	iamService *iamv1.Service,
	infra *extensionsv1alpha1.Infrastructure,
	providerConfig *gcpv1alpha1.InfrastructureConfig,
	dualStack bool,
) {
	// service account
	if !features.ExtensionFeatureGate.Enabled(features.DisableGardenerServiceAccountCreation) {
		serviceAccountName := getServiceAccountName(project, infra.Namespace)
		serviceAccount, err := iamService.Projects.ServiceAccounts.Get(serviceAccountName).Context(ctx).Do()
		Expect(err).NotTo(HaveOccurred())
		Expect(serviceAccount.DisplayName).To(Equal(infra.Namespace))
	}

	// network

	network, err := computeService.Networks.Get(project, infra.Namespace).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(network.AutoCreateSubnetworks).To(BeFalse())
	if dualStack {
		// dual stack shoots also have the services subnet.
		Expect(network.Subnetworks).To(HaveLen(3))
	} else {
		Expect(network.Subnetworks).To(HaveLen(2))
	}

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
	Expect(routerNAT.SourceSubnetworkIpRangesToNat).To(Equal("LIST_OF_SUBNETWORKS"))
	Expect(routerNAT.LogConfig.Enable).To(BeTrue())
	Expect(routerNAT.LogConfig.Filter).To(Equal("ERRORS_ONLY"))
	Expect(routerNAT.Subnetworks).To(HaveLen(1))
	Expect(routerNAT.Subnetworks[0].Name).To(Equal(subnetNodes.SelfLink))
	Expect(routerNAT.Subnetworks[0].SourceIpRangesToNat).To(Equal([]string{"ALL_IP_RANGES"}))

	if cn := providerConfig.Networks.CloudNAT; cn != nil {
		Expect(routerNAT.EnableDynamicPortAllocation).To(Equal(cn.EnableDynamicPortAllocation))
		if cn.MinPortsPerVM != nil {
			Expect(routerNAT.MinPortsPerVm).To(Equal(int64(*cn.MinPortsPerVM)))
		} else {
			Expect(routerNAT.MinPortsPerVm).To(Equal(int64(2048)))
		}
		if cn.MaxPortsPerVM != nil {
			Expect(routerNAT.MaxPortsPerVm).To(Equal(int64(*cn.MaxPortsPerVM)))
		}
	}

	if providerConfig.Networks.CloudNAT != nil && len(providerConfig.Networks.CloudNAT.NatIPNames) > 0 {
		Expect(routerNAT.NatIpAllocateOption).To(Equal("MANUAL_ONLY"))
		Expect(routerNAT.NatIps).To(HaveLen(len(providerConfig.Networks.CloudNAT.NatIPNames)))

		// ip addresses
		ipAddresses := make(map[string]bool)
		for _, natIPName := range providerConfig.Networks.CloudNAT.NatIPNames {
			address, err := computeService.Addresses.Get(project, *region, natIPName.Name).Context(ctx).Do()
			Expect(err).NotTo(HaveOccurred())
			ipAddresses[address.SelfLink] = true
			// egress cidr
			ipCIDR := fmt.Sprintf("%s/32", address.Address)
			Expect(infra.Status.EgressCIDRs).Should(ContainElement(ipCIDR))
		}
		for _, natIP := range routerNAT.NatIps {
			Expect(ipAddresses).Should(HaveKey(natIP))
		}
	} else {
		Expect(routerNAT.NatIpAllocateOption).To(Equal("AUTO_ONLY"))
	}

	// firewalls

	allowInternalAccess, err := computeService.Firewalls.Get(project, infra.Namespace+"-allow-internal-access").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	Expect(allowInternalAccess.Network).To(Equal(network.SelfLink))
	Expect(allowInternalAccess.SourceRanges).To(HaveLen(3))
	Expect(allowInternalAccess.SourceRanges).To(ConsistOf(workersSubnetCIDR, internalSubnetCIDR, podCIDR))
	Expect(allowInternalAccess.Priority).To(Equal(int64(1000)))
	Expect(allowInternalAccess.Allowed).To(ConsistOf([]*computev1.FirewallAllowed{
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

	allowHealthChecks, err := computeService.Firewalls.Get(project, infra.Namespace+"-allow-health-checks").Context(ctx).Do()
	Expect(err).NotTo(HaveOccurred())

	Expect(allowHealthChecks.Network).To(Equal(network.SelfLink))
	Expect(allowHealthChecks.Priority).To(Equal(int64(1000)))
	Expect(allowHealthChecks.SourceRanges).To(ConsistOf([]string{
		"35.191.0.0/16",
		"209.85.204.0/22",
		"209.85.152.0/22",
		"130.211.0.0/22",
	}))
	Expect(allowHealthChecks.Allowed).To(ConsistOf([]*computev1.FirewallAllowed{
		{
			IPProtocol: "tcp",
			Ports:      []string{"30000-32767"},
		},
		{
			IPProtocol: "udp",
			Ports:      []string{"30000-32767"},
		},
	}))
	if dualStack {
		verifyDualStackSetup(ctx, project, computeService, infra.Namespace)
	}
}

func verifyDeletion(
	ctx context.Context,
	project string,
	computeService *computev1.Service,
	iamService *iamv1.Service,
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

func newCluster(name string) (*extensionsv1alpha1.Cluster, error) {
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Shoot",
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
		},
		Spec: gardencorev1beta1.ShootSpec{
			Networking: &gardencorev1beta1.Networking{
				Pods: ptr.To(podCIDR),
			},
		},
	}

	shootJSON, err := json.Marshal(shoot)
	if err != nil {
		return nil, err
	}
	cluster := &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
			Seed:         runtime.RawExtension{Raw: []byte("{}")},
			Shoot:        runtime.RawExtension{Raw: shootJSON},
		},
	}
	return cluster, nil
}

func newProviderConfigForExistingVPC() (string, *gcpv1alpha1.InfrastructureConfig) {
	namespace, err := generateNamespaceName()
	Expect(err).NotTo(HaveOccurred())

	networkName := namespace
	cloudRouterName := networkName + "-cloud-router"
	ipAddressNames := []string{networkName + "-manual-nat1", networkName + "-manual-nat2"}

	var cleanupHandle framework.CleanupActionHandle
	cleanupHandle = framework.AddCleanupAction(func() {
		err := teardownNetwork(ctx, log, project, computeService, networkName, cloudRouterName)
		Expect(err).NotTo(HaveOccurred())
		err = teardownIPAddresses(ctx, log, project, computeService, ipAddressNames)
		Expect(err).NotTo(HaveOccurred())

		framework.RemoveCleanupAction(cleanupHandle)
	})

	err = prepareNewNetwork(ctx, log, project, computeService, networkName, cloudRouterName)
	Expect(err).NotTo(HaveOccurred())

	err = prepareNewIPAddresses(ctx, log, project, computeService, ipAddressNames)
	Expect(err).NotTo(HaveOccurred())

	vpc := &gcpv1alpha1.VPC{
		Name: networkName,
		CloudRouter: &gcpv1alpha1.CloudRouter{
			Name: cloudRouterName,
		},
	}
	var natIPNames []gcpv1alpha1.NatIPName
	for _, ipAddressName := range ipAddressNames {
		natIPNames = append(natIPNames, gcpv1alpha1.NatIPName{Name: ipAddressName})
	}
	cloudNAT := &gcpv1alpha1.CloudNAT{
		MinPortsPerVM:               ptr.To[int32](1024),
		MaxPortsPerVM:               ptr.To[int32](2048),
		EnableDynamicPortAllocation: true,
		NatIPNames:                  natIPNames,
	}
	return namespace, newProviderConfig(vpc, cloudNAT)
}
