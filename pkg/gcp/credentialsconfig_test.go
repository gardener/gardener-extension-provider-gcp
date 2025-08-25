// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"
	"fmt"

	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Service Account", func() {
	var (
		projectID             string
		email                 string
		credentialsConfigData []byte
		credentialsConfig     *CredentialsConfig
		secret                *corev1.Secret
	)
	BeforeEach(func() {
		projectID = "project"
		email = "email"
		credentialsConfigData = []byte(fmt.Sprintf(`{"project_id": "%s", "client_email": "%s"}`, projectID, email))
		credentialsConfig = &CredentialsConfig{ProjectID: projectID, Email: email, Raw: credentialsConfigData}
		secret = &corev1.Secret{
			Data: map[string][]byte{
				ServiceAccountJSONField: credentialsConfigData,
			},
		}
	})

	var ctrl *gomock.Controller
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#ExtractServiceAccountProjectID", func() {
		It("should correctly extract the project ID", func() {
			sa, err := GetCredentialsConfigFromJSON(credentialsConfigData)
			Expect(err).NotTo(HaveOccurred())
			Expect(sa.ProjectID).To(Equal(projectID))
		})

		It("should error if the project ID is empty", func() {
			_, err := GetCredentialsConfigFromJSON([]byte(`{"type": "service_account","project_id": ""}`))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no project id specified"))
		})

		It("should error on malformed json", func() {
			_, err := GetCredentialsConfigFromJSON([]byte(`{"project_id": ""`))

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected end of JSON input"))
		})
	})

	Describe("#ExtractCredentialType", func() {
		It("should correctly extract the credential type", func() {
			sa, err := GetCredentialsConfigFromJSON([]byte(`{"type": "service_account","project_id": "foobar"}`))
			Expect(err).NotTo(HaveOccurred())
			Expect(sa.Type).To(Equal("service_account"))
			Expect(sa.ProjectID).To(Equal("foobar"))
		})
	})

	Describe("#ReadCredentialsConfigSecret", func() {
		It("should read the credentials config data from the secret service account field", func() {
			secret := &corev1.Secret{Data: map[string][]byte{
				ServiceAccountJSONField: credentialsConfigData,
			}}

			actual, err := getCredentialsConfigFromSecret(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual.Raw).To(Equal(credentialsConfigData))
		})

		It("should read the credentials config data from the secret credentials config field", func() {
			data := []byte(`{"audience":"//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider","credential_source":{"file":"/var/run/secrets/gardener.cloud/workload-identity/token","format":{"type":"text"}},"subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://sts.googleapis.com/v1/token","type":"external_account","universe_domain":"googleapis.com","service_account_impersonation_url":"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken"}`)
			secret := &corev1.Secret{Data: map[string][]byte{
				"credentialsConfig": data,
			}}

			actual, err := getCredentialsConfigFromSecret(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(&CredentialsConfig{
				Raw:                            data,
				Type:                           "external_account",
				TokenFilePath:                  "/var/run/secrets/gardener.cloud/workload-identity/token",
				Audience:                       "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
				UniverseDomain:                 "googleapis.com",
				SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:                       "https://sts.googleapis.com/v1/token",
				ServiceAccountImpersonationURL: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken",
			}))
		})

		It("should fall back and read the credentials config from the config field", func() {
			config := []byte(`apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
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
			secret := &corev1.Secret{Data: map[string][]byte{
				"config": config,
			}}
			expectedRawData := []byte(`{"audience":"//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider","credential_source":{"file":"/var/run/secrets/gardener.cloud/workload-identity/token","format":{"type":"text"}},"service_account_impersonation_url":"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://sts.googleapis.com/v1/token","type":"external_account","universe_domain":"googleapis.com"}`)

			actual, err := getCredentialsConfigFromSecret(secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(&CredentialsConfig{
				Raw:                            expectedRawData,
				Type:                           "external_account",
				TokenFilePath:                  "/var/run/secrets/gardener.cloud/workload-identity/token",
				Audience:                       "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
				UniverseDomain:                 "googleapis.com",
				SubjectTokenType:               "urn:ietf:params:oauth:token-type:jwt",
				TokenURL:                       "https://sts.googleapis.com/v1/token",
				ServiceAccountImpersonationURL: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken",
				ProjectID:                      "test-proj",
			}))
		})
	})

	Describe("#GetCredentialsConfigData", func() {
		It("should retrieve the service account data", func() {
			var (
				c         = mockclient.NewMockClient(ctrl)
				ctx       = context.TODO()
				namespace = "foo"
				name      = "bar"
				secretRef = corev1.SecretReference{
					Namespace: namespace,
					Name:      name,
				}
			)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
					*actual = *secret
					return nil
				})

			actual, err := GetCredentialsConfigFromSecretReference(ctx, c, secretRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual.Raw).To(Equal(credentialsConfigData))
		})
	})

	Describe("#GetCredentialsConfig", func() {
		It("should correctly retrieve the service account", func() {
			var (
				c         = mockclient.NewMockClient(ctrl)
				ctx       = context.TODO()
				namespace = "foo"
				name      = "bar"
				secretRef = corev1.SecretReference{
					Namespace: namespace,
					Name:      name,
				}
			)
			c.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, actual *corev1.Secret, _ ...client.GetOption) error {
					*actual = *secret
					return nil
				})

			actual, err := GetCredentialsConfigFromSecretReference(ctx, c, secretRef)

			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(credentialsConfig))
		})
	})
})
