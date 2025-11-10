// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	googledns "google.golang.org/api/dns/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gcpinstall "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
	dnsrecordctrl "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var (
	serviceAccount = flag.String("service-account", "", "Service account containing credentials for the GCP API")
	logLevel       = flag.String("log-level", "", "Log level (debug, info, error)")
)

func validateFlags() {
	if len(*serviceAccount) == 0 {
		panic("--service-account flag is not specified")
	}
	if len(*logLevel) == 0 {
		logLevel = ptr.To(logger.DebugLevel)
	} else {
		if !slices.Contains(logger.AllLogLevels, *logLevel) {
			panic("invalid log level: " + *logLevel)
		}
	}
}

var (
	ctx = context.Background()

	log        logr.Logger
	dnsService *googledns.Service
	testEnv    *envtest.Environment
	mgrCancel  context.CancelFunc
	c          client.Client

	project     string
	testName    string
	zoneName    string
	zoneDnsName string
	zoneID      string

	namespace *corev1.Namespace
	secret    *corev1.Secret
	cluster   *extensionsv1alpha1.Cluster
)

var _ = BeforeSuite(func() {
	repoRoot := filepath.Join("..", "..", "..")

	// enable manager logs
	logf.SetLogger(logger.MustNewZapLogger(*logLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter)))
	log = logf.Log.WithName("dnsrecord-test")

	flag.Parse()
	validateFlags()

	var mgrContext context.Context
	mgrContext, mgrCancel = context.WithCancel(ctx)

	DeferCleanup(func() {
		defer func() {
			By("stopping manager")
			mgrCancel()
		}()

		By("running cleanup actions")
		framework.RunCleanupActions()

		By("deleting GCP DNS hosted zone")
		deleteDNSHostedZone(ctx, dnsService, zoneID)

		By("tearing down shoot environment")
		teardownShootEnvironment(ctx, c, namespace, secret, cluster)

		By("stopping test environment")
		if testEnv != nil {
			Expect(testEnv.Stop()).To(Succeed())
		}
	})

	By("generating randomized test resource identifiers")
	testName = fmt.Sprintf("gcp-dnsrecord-it--%s", randomString())
	zoneName = testName + "-gardener-cloud"
	zoneDnsName = testName + ".gardener.cloud."
	namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
	}
	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dnsrecord",
			Namespace: testName,
		},
		Data: map[string][]byte{
			gcp.ServiceAccountJSONField: []byte(*serviceAccount),
		},
	}
	cluster = &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			CloudProfile: runtime.RawExtension{Raw: []byte("{}")},
			Seed:         runtime.RawExtension{Raw: []byte("{}")},
			Shoot:        runtime.RawExtension{Raw: []byte("{}")},
		},
	}

	By("starting test environment")
	testEnv = &envtest.Environment{
		UseExistingCluster: ptr.To(true),
		CRDInstallOptions: envtest.CRDInstallOptions{
			Paths: []string{
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_dnsrecords.yaml"),
				filepath.Join(repoRoot, "example", "20-crd-extensions.gardener.cloud_clusters.yaml"),
			},
		},
		ControlPlaneStopTimeout: 2 * time.Minute,
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	By("setting up manager")
	mgr, err := manager.New(cfg, manager.Options{
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	})
	Expect(err).ToNot(HaveOccurred(), "Failed to create manager for the test environment")

	Expect(extensionsv1alpha1.AddToScheme(mgr.GetScheme())).To(Succeed(), "Failed to add extensionsv1alpha1 scheme to manager")
	Expect(gcpinstall.AddToScheme(mgr.GetScheme())).To(Succeed(), "Failed to add GCP scheme to manager")

	Expect(dnsrecordctrl.AddToManagerWithOptions(ctx, mgr, dnsrecordctrl.AddOptions{})).To(Succeed(), "Failed to add DNSRecord controller to manager")

	By("starting manager")
	go func() {
		defer GinkgoRecover()
		err := mgr.Start(mgrContext)
		Expect(err).NotTo(HaveOccurred(), "Failed to start the manager")
	}()

	By("getting client")
	c, err = client.New(cfg, client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	})
	Expect(err).NotTo(HaveOccurred(), "Failed to create client for the test environment")
	Expect(c).NotTo(BeNil(), "Client for the test environment is nil")

	fmt.Println("service account: ", *serviceAccount)
	credentialsConfig, err := gcp.GetCredentialsConfigFromJSON([]byte(*serviceAccount))
	Expect(err).NotTo(HaveOccurred())
	project = credentialsConfig.ProjectID

	dnsService, err = googledns.NewService(ctx, option.WithCredentialsJSON([]byte(*serviceAccount)), option.WithScopes(googledns.NdevClouddnsReadwriteScope))
	Expect(err).NotTo(HaveOccurred())

	By("setting up shoot environment")
	setupShootEnvironment(ctx, c, namespace, secret, cluster)

	By("creating GCP DNS hosted zone")
	zoneID = createDNSHostedZone(ctx, dnsService, zoneDnsName)
})

var runTest = func(dns *extensionsv1alpha1.DNSRecord, newValues []string, beforeCreate, beforeUpdate, beforeDelete func()) {
	if beforeCreate != nil {
		beforeCreate()
	}

	By("creating dnsrecord")
	createDNSRecord(ctx, c, dns)

	defer func() {
		if beforeDelete != nil {
			beforeDelete()
		}

		By("deleting dnsrecord")
		deleteDNSRecord(ctx, c, dns)

		By("waiting until dnsrecord is deleted")
		waitUntilDNSRecordDeleted(ctx, c, log, dns)

		By("verifying that the GCP DNS recordset does not exist")
		verifyDNSRecordSetDeleted(ctx, dnsService, dns)
	}()

	framework.AddCleanupAction(func() {
		By("deleting the GCP DNS recordset if it still exists")
		deleteDNSRecordSet(ctx, dnsService, dns)
	})

	By("waiting until dnsrecord is ready")
	waitUntilDNSRecordReady(ctx, c, log, dns)

	By("getting dnsrecord and verifying its status")
	getDNSRecordAndVerifyStatus(ctx, c, dns, zoneID)

	By("verifying that the GCP DNS recordset exists and matches dnsrecord")
	verifyDNSRecordSet(ctx, dnsService, dns)

	if len(newValues) > 0 {
		if beforeUpdate != nil {
			beforeUpdate()
		}

		dns.Spec.Values = newValues
		metav1.SetMetaDataAnnotation(&dns.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)

		By("updating dnsrecord")
		updateDNSRecord(ctx, c, dns)

		By("waiting until dnsrecord is ready")
		waitUntilDNSRecordReady(ctx, c, log, dns)

		By("getting dnsrecord and verifying its status")
		getDNSRecordAndVerifyStatus(ctx, c, dns, zoneID)

		By("verifying that the GCP DNS recordset exists and matches dnsrecord")
		verifyDNSRecordSet(ctx, dnsService, dns)
	}
}

var _ = Describe("DNSRecord tests", func() {
	Context("when a DNS recordset doesn't exist and is not changed or deleted before dnsrecord deletion", func() {
		It("should successfully create and delete a dnsrecord of type A", func() {
			dns := newDNSRecord(testName, zoneDnsName, zoneID, extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))
			runTest(dns, nil, nil, nil, nil)
		})

		It("should successfully create and delete a dnsrecord of type CNAME", func() {
			dns := newDNSRecord(testName, zoneDnsName, zoneID, extensionsv1alpha1.DNSRecordTypeCNAME, []string{"foo.example.com."}, ptr.To[int64](600))
			runTest(dns, nil, nil, nil, nil)
		})

		It("should successfully create and delete a dnsrecord of type TXT", func() {
			dns := newDNSRecord(testName, zoneDnsName, zoneID, extensionsv1alpha1.DNSRecordTypeTXT, []string{"foo", "bar"}, nil)
			runTest(dns, nil, nil, nil, nil)
		})
	})

	Context("when a DNS recordset exists and is changed before dnsrecord update and deletion", func() {
		It("should successfully create, update, and delete a dnsrecord", func() {
			dns := newDNSRecord(testName, zoneDnsName, zoneID, extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))

			updateDNS := func() {
				By("updating GCP DNS recordset")
				_, err := dnsService.Changes.Create(project, zoneName, &googledns.Change{
					Deletions: []*googledns.ResourceRecordSet{{
						Name:    dns.Spec.Name,
						Type:    string(dns.Spec.RecordType),
						Ttl:     ptr.Deref(dns.Spec.TTL, 120),
						Rrdatas: dns.Spec.Values,
					}},
					Additions: []*googledns.ResourceRecordSet{{
						Name:    dns.Spec.Name,
						Type:    string(dns.Spec.RecordType),
						Ttl:     ptr.Deref(dns.Spec.TTL, 120),
						Rrdatas: []string{"8.8.8.8"},
					}},
				}).Do()
				Expect(err).To(BeNil())
			}

			runTest(
				dns,
				[]string{"3.3.3.3", "1.1.1.1"},
				func() {
					By("creating GCP DNS recordset")
					_, err := dnsService.ResourceRecordSets.Create(project, zoneName, &googledns.ResourceRecordSet{
						Name:    dns.Spec.Name,
						Type:    string(dns.Spec.RecordType),
						Ttl:     ptr.Deref(dns.Spec.TTL, 120),
						Rrdatas: []string{"8.8.8.8"},
					}).Do()
					Expect(err).To(BeNil())
				},
				updateDNS,
				updateDNS,
			)
		})
	})

	Context("when a DNS recordset exists and is deleted before dnsrecord deletion", func() {
		It("should successfully create and delete a dnsrecord", func() {
			dns := newDNSRecord(testName, zoneDnsName, zoneID, extensionsv1alpha1.DNSRecordTypeA, []string{"1.1.1.1", "2.2.2.2"}, ptr.To[int64](300))

			runTest(
				dns,
				nil,
				func() {
					By("creating GCP DNS recordset")
					_, err := dnsService.ResourceRecordSets.Create(project, zoneName, &googledns.ResourceRecordSet{
						Name:    dns.Spec.Name,
						Type:    string(dns.Spec.RecordType),
						Ttl:     ptr.Deref(dns.Spec.TTL, 120),
						Rrdatas: []string{"8.8.8.8"},
					}).Do()
					Expect(err).To(BeNil())
				},
				nil,
				func() {
					By("deleting GCP DNS recordset")
					_, err := dnsService.ResourceRecordSets.Delete(
						project,
						zoneName,
						dns.Spec.Name,
						string(dns.Spec.RecordType)).Do()
					Expect(err).To(BeNil())
				},
			)
		})
	})
})

func setupShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster) {
	Expect(c.Create(ctx, namespace)).To(Succeed())
	Expect(c.Create(ctx, secret)).To(Succeed())
	Expect(c.Create(ctx, cluster)).To(Succeed())
}

func teardownShootEnvironment(ctx context.Context, c client.Client, namespace *corev1.Namespace, secret *corev1.Secret, cluster *extensionsv1alpha1.Cluster) {
	if c != nil && cluster != nil {
		Expect(client.IgnoreNotFound(c.Delete(ctx, cluster))).To(Succeed())
	}
	if c != nil && secret != nil {
		Expect(client.IgnoreNotFound(c.Delete(ctx, secret))).To(Succeed())
	}
	if c != nil && namespace != nil {
		Expect(client.IgnoreNotFound(c.Delete(ctx, namespace))).To(Succeed())
	}
}

func createDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(c.Create(ctx, dns)).To(Succeed())
}

func updateDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(c.Update(ctx, dns)).To(Succeed())
}

func deleteDNSRecord(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord) {
	Expect(client.IgnoreNotFound(c.Delete(ctx, dns))).To(Succeed())
}

func getDNSRecordAndVerifyStatus(ctx context.Context, c client.Client, dns *extensionsv1alpha1.DNSRecord, zoneID string) {
	projectZone := project + "/" + zoneID
	Expect(c.Get(ctx, client.ObjectKey{Namespace: dns.Namespace, Name: dns.Name}, dns)).To(Succeed())
	Expect(dns.Status.Zone).To(PointTo(Equal(projectZone)))
}

func waitUntilDNSRecordReady(ctx context.Context, c client.Client, log logr.Logger, dns *extensionsv1alpha1.DNSRecord) {
	Expect(extensions.WaitUntilExtensionObjectReady(
		ctx,
		c,
		log,
		dns,
		extensionsv1alpha1.DNSRecordResource,
		10*time.Second,
		30*time.Second,
		5*time.Minute,
		nil,
	)).To(Succeed())
}

func waitUntilDNSRecordDeleted(ctx context.Context, c client.Client, log logr.Logger, dns *extensionsv1alpha1.DNSRecord) {
	Expect(extensions.WaitUntilExtensionObjectDeleted(
		ctx,
		c,
		log,
		dns.DeepCopy(),
		extensionsv1alpha1.DNSRecordResource,
		10*time.Second,
		5*time.Minute,
	)).To(Succeed())
}

func newDNSRecord(namespace string, zoneDnsName string, zoneID string, recordType extensionsv1alpha1.DNSRecordType, values []string, ttl *int64) *extensionsv1alpha1.DNSRecord {
	name := "dnsrecord-" + randomString()
	projectZone := project + "/" + zoneID
	return &extensionsv1alpha1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: extensionsv1alpha1.DNSRecordSpec{
			DefaultSpec: extensionsv1alpha1.DefaultSpec{
				Type: gcp.DNSType,
			},
			SecretRef: corev1.SecretReference{
				Name:      "dnsrecord",
				Namespace: namespace,
			},
			Zone:       &projectZone,
			Name:       name + "." + zoneDnsName,
			RecordType: recordType,
			Values:     values,
			TTL:        ttl,
		},
	}
}

func createDNSHostedZone(_ context.Context, dnsService *googledns.Service, zoneDnsName string) string {
	zone, err := dnsService.ManagedZones.Create(project, &googledns.ManagedZone{
		Name:        zoneName,
		DnsName:     zoneDnsName,
		Description: "Test zone for test " + testName,
		Visibility:  "public",
	}).Do()
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("%d", zone.Id)
}

func deleteDNSHostedZone(_ context.Context, dnsService *googledns.Service, zoneID string) {
	if dnsService == nil {
		return
	}
	err := dnsService.ManagedZones.Delete(project, zoneID).Do()
	Expect(err).NotTo(HaveOccurred())
}

func verifyDNSRecordSet(_ context.Context, dnsService *googledns.Service, dns *extensionsv1alpha1.DNSRecord) {
	rrs, err := dnsService.ResourceRecordSets.Get(project, zoneName, dns.Spec.Name, string(dns.Spec.RecordType)).Do()
	Expect(err).NotTo(HaveOccurred())
	Expect(rrs).NotTo(BeNil())

	Expect(rrs.Name).To(Equal(ensureTrailingDot(dns.Spec.Name)))
	Expect(rrs.Type).To(Equal(string(dns.Spec.RecordType)))
	Expect(rrs.Ttl).To(Equal(ptr.Deref(dns.Spec.TTL, 120)))

	Expect(rrs.Rrdatas).To(WithTransform(func(in []string) []string {
		switch dns.Spec.RecordType {
		case extensionsv1alpha1.DNSRecordTypeTXT:
			out := make([]string, len(in))
			for i, v := range in {
				out[i] = strings.Trim(v, "\"")
			}
			return out
		default:
			return in
		}
	}, ConsistOf(dns.Spec.Values)))
}

func verifyDNSRecordSetDeleted(_ context.Context, dnsService *googledns.Service, dns *extensionsv1alpha1.DNSRecord) {
	_, err := dnsService.ResourceRecordSets.Get(project, zoneDnsName, dns.Spec.Name, string(dns.Spec.RecordType)).Do()
	var googleError *googleapi.Error
	ok := errors.As(err, &googleError)
	Expect(ok).To(BeTrue())
	Expect(googleError.Code).To(Equal(404))
}

func deleteDNSRecordSet(_ context.Context, dnsService *googledns.Service, dns *extensionsv1alpha1.DNSRecord) {
	response, err := dnsService.ResourceRecordSets.Delete(project, zoneName, dns.Spec.Name, string(dns.Spec.RecordType)).Do()
	if gcpclient.IsErrorCode(err, 404) {
		return
	}
	Expect(err).NotTo(HaveOccurred())
	Expect(response.HTTPStatusCode).To(Equal(204))
}

func ensureTrailingDot(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

func randomString() string {
	rs, err := gardenerutils.GenerateRandomStringFromCharset(5, "0123456789abcdefghijklmnopqrstuvwxyz")
	Expect(err).NotTo(HaveOccurred())
	return rs
}
