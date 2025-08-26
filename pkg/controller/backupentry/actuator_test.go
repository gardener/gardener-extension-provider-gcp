// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/controller/backupentry/genericactuator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupentry"
)

const (
	entryName  = "test-entry"
	region     = "region-1"
	secretName = "secret-1"
	namespace  = "shoot--foo--bar"
)

var _ = Describe("Actuator", func() {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		secret      *corev1.Secret
		backupEntry *extensionsv1alpha1.BackupEntry
		actuator    genericactuator.BackupEntryDelegate
		ctx         context.Context
		log         logr.Logger
		validConfig []byte
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, zap.WriteTo(GinkgoWriter))

		fakeClient = fakeclient.NewClientBuilder().Build()
		fakeManager = &test.FakeManager{Client: fakeClient}
		actuator = backupentry.NewActuator(fakeManager)

		validConfig = []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: test-proj
credentialsConfig:
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token"
  service_account_impersonation_url: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken"
`)
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      secretName,
				Labels: map[string]string{
					"security.gardener.cloud/purpose":                   "workload-identity-token-requestor",
					"workloadidentity.security.gardener.cloud/provider": "gcp",
				},
			},
			Data: map[string][]byte{
				"config": validConfig,
			},
		}
		Expect(fakeClient.Create(ctx, secret)).To(Succeed())

		backupEntry = &extensionsv1alpha1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: entryName,
			},
			Spec: extensionsv1alpha1.BackupEntrySpec{
				Region: region,
				SecretRef: corev1.SecretReference{
					Namespace: namespace,
					Name:      secretName,
				},
			},
		}

	})

	var _ = Describe("#GetETCDSecretData", func() {
		It("should not inject any data because secret has no labels", func() {
			secret.Labels = nil
			Expect(fakeClient.Update(ctx, secret)).To(Succeed())
			data := map[string][]byte{}
			injectedData, err := actuator.GetETCDSecretData(ctx, log, backupEntry, data)
			Expect(err).ToNot(HaveOccurred())
			Expect(injectedData).To(Equal(data))
		})

		It("should not inject any data because secret's label 'security.gardener.cloud/purpose' has not the value 'workload-identity-token-requestor'", func() {
			secret.Labels["security.gardener.cloud/purpose"] = "foo"
			Expect(fakeClient.Update(ctx, secret)).To(Succeed())

			data := map[string][]byte{}
			injectedData, err := actuator.GetETCDSecretData(ctx, log, backupEntry, data)
			Expect(err).ToNot(HaveOccurred())
			Expect(injectedData).To(Equal(data))
		})

		It("should not inject any data because secret's label 'workloadidentity.security.gardener.cloud/provider' has not the value 'gcp'", func() {
			secret.Labels["workloadidentity.security.gardener.cloud/provider"] = "foo"
			Expect(fakeClient.Update(ctx, secret)).To(Succeed())

			data := map[string][]byte{}
			injectedData, err := actuator.GetETCDSecretData(ctx, log, backupEntry, data)
			Expect(err).ToNot(HaveOccurred())
			Expect(injectedData).To(Equal(data))
		})

		It("should fail to inject any data because secret has no data field 'config'", func() {
			delete(secret.Data, "config")
			Expect(fakeClient.Update(ctx, secret)).To(Succeed())

			data := map[string][]byte{}
			injectedData, err := actuator.GetETCDSecretData(ctx, log, backupEntry, data)
			Expect(err).To(MatchError(ContainSubstring("could not decode 'config' as WorkloadIdentityConfig")))
			Expect(injectedData).To(BeNil())
		})

		It("should fail to inject any data because secret does not exist", func() {
			Expect(fakeClient.Delete(ctx, secret)).To(Succeed())

			data := map[string][]byte{}
			injectedData, err := actuator.GetETCDSecretData(ctx, log, backupEntry, data)
			Expect(err).To(BeNotFoundError())
			Expect(injectedData).To(BeNil())
		})

		It("should fail to inject any data because secret data field 'config' is not valid", func() {
			secret.Data["config"] = []byte("invalid value")
			Expect(fakeClient.Update(ctx, secret)).To(Succeed())

			data := map[string][]byte{}
			injectedData, err := actuator.GetETCDSecretData(ctx, log, backupEntry, data)
			Expect(err).To(MatchError(ContainSubstring("json parse error: json: cannot unmarshal string into Go value of type struct")))
			Expect(injectedData).To(BeNil())
		})

		It("should successfully inject data", func() {
			data := map[string][]byte{
				"foo": []byte("bar"),
			}
			injectedData, err := actuator.GetETCDSecretData(ctx, log, backupEntry, data)
			Expect(err).ToNot(HaveOccurred())
			Expect(injectedData).To(HaveKeyWithValue("projectID", []byte("test-proj")))
			Expect(string(injectedData["credentialsConfig"])).To(Equal(`{"audience":"//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider","credential_source":{"file":"/var/.gcp/token","format":{"type":"text"}},"service_account_impersonation_url":"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://sts.googleapis.com/v1/token","type":"external_account","universe_domain":"googleapis.com"}`))
			Expect(string(injectedData["serviceaccount.json"])).To(Equal(`{"audience":"//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider","credential_source":{"file":"/var/.gcp/token","format":{"type":"text"}},"service_account_impersonation_url":"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://sts.googleapis.com/v1/token","type":"external_account","universe_domain":"googleapis.com"}`))
			Expect(injectedData).To(HaveKeyWithValue("foo", []byte("bar")))
		})
	})
})
