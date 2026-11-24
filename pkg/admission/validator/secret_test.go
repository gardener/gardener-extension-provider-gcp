// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Secret validator", func() {
	Describe("#Validate", func() {
		const (
			namespace             = "garden-dev"
			name                  = "my-secret"
			shootProviderLabelKey = "provider.shoot.gardener.cloud/gcp"
		)

		var (
			secretValidator extensionswebhook.Validator
			ctx             = context.TODO()

			secret    *corev1.Secret
			oldSecret *corev1.Secret

			validServiceAccountJSON = []byte(`{
				"type":           "service_account",
				"project_id":     "my-project-123",
				"private_key_id": "1234567890abcdef1234567890abcdef12345678",
				"private_key":    "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
				"client_email":   "my-service-account@my-project-123.iam.gserviceaccount.com",
				"client_id":      "123456789012345678901",
				"token_uri":      "https://oauth2.googleapis.com/token"
			}`)
		)

		BeforeEach(func() {
			secretValidator = validator.NewSecretValidator()
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: make(map[string][]byte),
			}
			oldSecret = nil
		})

		It("should error if newObj is not a Secret", func() {
			err := secretValidator.Validate(ctx, &corev1.ConfigMap{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.ConfigMap"))
		})

		It("should error if oldObj is not a Secret", func() {
			err := secretValidator.Validate(ctx, &corev1.Secret{}, &corev1.ConfigMap{})
			Expect(err).To(MatchError("wrong object type *v1.ConfigMap for old object"))
		})

		It("should validate unchanged data when the provider label is introduced", func() {
			// Invalid data committed before the label was present — the race scenario.
			// When core adds the label via a metadata-only update, the equal-data path
			// must not short-circuit; validation must run and catch the bad bytes.
			secret.Data["unexpected-key"] = []byte("bad")
			oldSecret = secret.DeepCopy()
			secret.Labels = map[string]string{shootProviderLabelKey: "true"}

			Expect(secretValidator.Validate(ctx, secret, oldSecret)).To(HaveOccurred())
		})

		It("should skip validation for unchanged data that was previously validated", func() {
			// Label already present on the old object means the data passed validation
			// when it first entered the selector. A metadata-only update is safe to skip.
			secret.Labels = map[string]string{shootProviderLabelKey: "true"}
			secret.Data["unexpected-key"] = []byte("bad")
			oldSecret = secret.DeepCopy()

			Expect(secretValidator.Validate(ctx, secret, oldSecret)).To(Succeed())
		})

		It("should validate changed data even when the provider label was already present", func() {
			secret.Labels = map[string]string{shootProviderLabelKey: "true"}
			secret.Data[gcp.ServiceAccountJSONField] = validServiceAccountJSON
			oldSecret = secret.DeepCopy()
			secret.Data[gcp.ServiceAccountJSONField] = []byte(`{"type":"external_account"}`)

			Expect(secretValidator.Validate(ctx, secret, oldSecret)).To(HaveOccurred())
		})

		It("should succeed for a new valid secret without an old object", func() {
			secret.Data[gcp.ServiceAccountJSONField] = validServiceAccountJSON
			Expect(secretValidator.Validate(ctx, secret, nil)).To(Succeed())
		})

		It("should fail for a new secret missing the serviceaccount.json field", func() {
			Expect(secretValidator.Validate(ctx, secret, nil)).To(HaveOccurred())
		})
	})
})
