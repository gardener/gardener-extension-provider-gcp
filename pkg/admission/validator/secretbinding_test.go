// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

			ctrl      *gomock.Controller
			apiReader *mockclient.MockReader
			ctx       = context.TODO()

			secretBinding = &core.SecretBinding{
				SecretRef: corev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			}
			fakeErr = fmt.Errorf("fake err")

			mgr *mockmanager.MockManager
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			apiReader = mockclient.NewMockReader(ctrl)

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetAPIReader().Return(apiReader)

			secretBindingValidator = validator.NewSecretBindingValidator(mgr)
		})

		AfterEach(func() {
			ctrl.Finish()
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
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return err when the corresponding Secret does not contain a 'serviceaccount.json' field", func() {
			secret := &corev1.Secret{Data: map[string][]byte{
				"foo": []byte("bar"),
			}}
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				SetArg(2, *secret)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("secret.data[serviceaccount.json]"),
				"Detail": Equal("missing required field \"serviceaccount.json\" in secret /"),
			}))))
		})

		It("should return err when the corresponding Secret does not contain a valid 'serviceaccount.json' field", func() {
			secret := &corev1.Secret{Data: map[string][]byte{
				gcp.ServiceAccountJSONField: []byte(``),
			}}
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				SetArg(2, *secret)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("secret.data[serviceaccount.json]"),
				"Detail": Equal("field \"serviceaccount.json\" cannot be empty in secret /"),
			}))))
		})

		It("should return nil when the corresponding Secret is valid", func() {
			secret := &corev1.Secret{Data: map[string][]byte{
				gcp.ServiceAccountJSONField: []byte(`{
					"type":                        "service_account",
					"project_id":                  "my-project-123",
					"private_key_id":              "1234567890abcdef1234567890abcdef12345678",
					"private_key":                 "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
					"client_email":                "my-service-account@my-project-123.iam.gserviceaccount.com",
					"client_id":                   "123456789012345678901",
					"token_uri":                   "https://oauth2.googleapis.com/token"
				}`),
			}}
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				SetArg(2, *secret)

			err := secretBindingValidator.Validate(ctx, secretBinding, nil)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
