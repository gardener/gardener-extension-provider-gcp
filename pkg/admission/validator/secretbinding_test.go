// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("SecretBinding validator", func() {
	Describe("#Validate", func() {
		const (
			namespace = "garden-dev"
			name      = "my-provider-account"
		)

		var (
			secretBindingValidator extensionswebhook.Validator

			ctx = context.TODO()

			secretBinding = &core.SecretBinding{
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			}

			mgr *test.FakeManager
		)

		BeforeEach(func() {
			apiReader := fakeclient.NewClientBuilder().Build()
			mgr = &test.FakeManager{APIReader: apiReader}
			secretBindingValidator = validator.NewSecretBindingValidator(mgr)
		})

		It("should return err when obj is not a SecretBinding", func() {
			err := secretBindingValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.Secret"))
		})

		It("should return err when oldObj is not a SecretBinding", func() {
			err := secretBindingValidator.Validate(ctx, &core.SecretBinding{}, &corev1.Secret{})
			Expect(err).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			// Secret not pre-populated in fake client → not found error
			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should return err when the corresponding Secret does not contain a 'serviceaccount.json' field", func() {
			apiReader := fakeclient.NewClientBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data:       map[string][]byte{"foo": []byte("bar")},
				},
			).Build()
			mgr = &test.FakeManager{APIReader: apiReader}
			secretBindingValidator = validator.NewSecretBindingValidator(mgr)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("secret.data[serviceaccount.json]"),
				"Detail": Equal("missing required field \"serviceaccount.json\" in secret garden-dev/my-provider-account"),
			}))))
		})

		It("should return err when the corresponding Secret does not contain a valid 'serviceaccount.json' field", func() {
			apiReader := fakeclient.NewClientBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data:       map[string][]byte{gcp.ServiceAccountJSONField: []byte(``)},
				},
			).Build()
			mgr = &test.FakeManager{APIReader: apiReader}
			secretBindingValidator = validator.NewSecretBindingValidator(mgr)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("secret.data[serviceaccount.json]"),
				"Detail": Equal("field \"serviceaccount.json\" cannot be empty in secret garden-dev/my-provider-account"),
			}))))
		})

		It("should return nil when the corresponding Secret is valid", func() {
			apiReader := fakeclient.NewClientBuilder().WithObjects(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data: map[string][]byte{
						gcp.ServiceAccountJSONField: []byte(`{
						"type":                        "service_account",
						"project_id":                  "my-project-123",
						"private_key_id":              "1234567890abcdef1234567890abcdef12345678",
						"private_key":                 "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
						"client_email":                "my-service-account@my-project-123.iam.gserviceaccount.com",
						"client_id":                   "123456789012345678901",
						"token_uri":                   "https://oauth2.googleapis.com/token"
					}`),
					},
				},
			).Build()
			mgr = &test.FakeManager{APIReader: apiReader}
			secretBindingValidator = validator.NewSecretBindingValidator(mgr)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
