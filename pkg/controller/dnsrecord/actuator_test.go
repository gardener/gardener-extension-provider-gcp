// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/dnsrecord"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/dnsrecord"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
)

const (
	name        = "dnsrecord-external"
	namespace   = "shoot--foobar--gcp"
	shootDomain = "shoot.example.com"
	domainName  = "api.gcp.foobar." + shootDomain
	zone        = "zone"
	address     = "1.2.3.4"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl             *gomock.Controller
		c                client.Client
		mgr              *test.FakeManager
		gcpClientFactory *mockgcpclient.MockFactory
		gcpDNSClient     *mockgcpclient.MockDNSClient
		ctx              context.Context
		logger           logr.Logger
		a                dnsrecord.Actuator
		dns              *extensionsv1alpha1.DNSRecord
		zones            map[string]string
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gcpClientFactory = mockgcpclient.NewMockFactory(ctrl)
		gcpDNSClient = mockgcpclient.NewMockDNSClient(ctrl)

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		scheme := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
		c = fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.DNSRecord{}).Build()
		mgr = &test.FakeManager{Client: c}

		a = NewActuator(mgr, gcpClientFactory)

		dns = &extensionsv1alpha1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: extensionsv1alpha1.DNSRecordSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: gcp.DNSType,
				},
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
				Name:       domainName,
				RecordType: extensionsv1alpha1.DNSRecordTypeA,
				Values:     []string{address},
			},
		}

		zones = map[string]string{
			shootDomain:   zone,
			"example.com": "zone2",
			"other.com":   "zone3",
		}
	})

	Describe("#Reconcile", func() {
		It("should reconcile the DNSRecord", func() {
			gcpClientFactory.EXPECT().DNS(ctx, c, dns.Spec.SecretRef).Return(gcpDNSClient, nil)
			gcpDNSClient.EXPECT().GetManagedZones(ctx).Return(zones, nil)
			gcpDNSClient.EXPECT().CreateOrUpdateRecordSet(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA), []string{address}, int64(120)).Return(nil)

			Expect(c.Create(ctx, dns)).To(Succeed())

			err := a.Reconcile(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(dns.Status.Zone).To(Equal(ptr.To(zone)))
		})
	})

	Describe("#Delete", func() {
		It("should delete the DNSRecord", func() {
			dns.Status.Zone = ptr.To(zone)
			gcpClientFactory.EXPECT().DNS(ctx, c, dns.Spec.SecretRef).Return(gcpDNSClient, nil)
			gcpDNSClient.EXPECT().DeleteRecordSet(ctx, zone, domainName, string(extensionsv1alpha1.DNSRecordTypeA)).Return(nil)

			err := a.Delete(ctx, logger, dns, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
