// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider_test

import (
	"context"
	"slices"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/webhook/cloudprovider"
)

var _ = Describe("Ensurer", func() {
	var (
		logger = log.Log.WithName("gcp-cloudprovider-webhook-test")
		ctx    = context.TODO()

		ensurer cloudprovider.Ensurer

		secret *corev1.Secret

		expectedCredentialsConfig []byte

		ctrl *gomock.Controller
		mgr  *mockmanager.MockManager
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mgr = mockmanager.NewMockManager(ctrl)

		scheme := kubernetes.SeedScheme
		Expect(install.AddToScheme(scheme)).To(Succeed())
		mgr.EXPECT().GetScheme().Return(scheme)

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					"workloadidentity.security.gardener.cloud/provider": "gcp",
				},
			},
			Data: map[string][]byte{
				"config": []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: test-proj
credentialsConfig:
  unused_field: "foo"
  universe_domain: "googleapis.com"
  type: "external_account"
  audience: "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider"
  subject_token_type: "urn:ietf:params:oauth:token-type:jwt"
  token_url: "https://sts.googleapis.com/v1/token"
  service_account_impersonation_url: "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken"
  credential_source:
    file: "/abc/cloudprovider/xyz"
    abc: 
      foo: "text"
`),
			},
		}

		ensurer = NewEnsurer(mgr, logger)

		expectedCredentialsConfig = []byte(`{"audience":"//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider","credential_source":{"file":"/var/run/secrets/gardener.cloud/workload-identity/token","format":{"type":"text"}},"service_account_impersonation_url":"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/SERVICE_ACCOUNT_EMAIL:generateAccessToken","subject_token_type":"urn:ietf:params:oauth:token-type:jwt","token_url":"https://sts.googleapis.com/v1/token","type":"external_account","universe_domain":"googleapis.com"}`)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#EnsureCloudProviderSecret", func() {
		It("should overwrite the 'credential_source' property from config and remove obsolete fields", func() {
			cloned := slices.Clone(secret.Data["config"])
			Expect(ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)).To(Succeed())
			Expect(secret.Data["credentialsConfig"]).To(Equal(expectedCredentialsConfig))
			Expect(secret.Data["config"]).To(Equal(cloned))
			Expect(secret.Data["projectID"]).To(Equal([]byte("test-proj")))
		})

		It("should not modify the secret if it is not labeled correctly", func() {
			secret.Labels["workloadidentity.security.gardener.cloud/provider"] = "foo"
			expected := secret.DeepCopy()
			Expect(ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)).To(Succeed())
			Expect(secret).To(Equal(expected))
		})

		It("should error if cloudprovider secret does not contain config data key", func() {
			delete(secret.Data, "config")
			err := ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)
			Expect(err).To(HaveOccurred())

			Expect(err).To(MatchError("cloudprovider secret is missing a 'config' data key"))
		})

		It("should error if cloudprovider secret does not contain a valid providerConfig", func() {
			secret.Data["config"] = []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
projectID: test-proj
credentialsConfig: invalid
`)
			err := ensurer.EnsureCloudProviderSecret(ctx, nil, secret, nil)
			Expect(err).To(HaveOccurred())

			Expect(err.Error()).To(ContainSubstring("could not unmarshal credential config"))
		})
	})
})
