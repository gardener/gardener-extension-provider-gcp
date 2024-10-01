// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"errors"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	gcpapi "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	gcpapiv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("CredentialsBinding validator", func() {
	Describe("#Validate", func() {
		const (
			namespace = "garden-dev"
			name      = "my-provider-account"
		)

		var (
			credentialsBindingValidator extensionswebhook.Validator

			ctrl      *gomock.Controller
			mgr       *mockmanager.MockManager
			apiReader *mockclient.MockReader

			ctx                                = context.TODO()
			credentialsBindingSecret           *security.CredentialsBinding
			credentialsBindingWorkloadIdentity *security.CredentialsBinding

			fakeErr = fmt.Errorf("fake err")
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			mgr = mockmanager.NewMockManager(ctrl)

			apiReader = mockclient.NewMockReader(ctrl)
			mgr.EXPECT().GetAPIReader().Return(apiReader)
			scheme := runtime.NewScheme()
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(gcpapi.AddToScheme(scheme)).To(Succeed())
			Expect(gcpapiv1alpha1.AddToScheme(scheme)).To(Succeed())

			mgr.EXPECT().GetScheme().Return(scheme)

			credentialsBindingValidator = validator.NewCredentialsBindingValidator(mgr)

			credentialsBindingSecret = &security.CredentialsBinding{
				CredentialsRef: corev1.ObjectReference{
					Name:       name,
					Namespace:  namespace,
					Kind:       "Secret",
					APIVersion: "v1",
				},
			}
			credentialsBindingWorkloadIdentity = &security.CredentialsBinding{
				CredentialsRef: corev1.ObjectReference{
					Name:       name,
					Namespace:  namespace,
					Kind:       "WorkloadIdentity",
					APIVersion: "security.gardener.cloud/v1alpha1",
				},
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return err when obj is not a CredentialsBinding", func() {
			err := credentialsBindingValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(MatchError("wrong object type *v1.Secret"))
		})

		It("should return err when oldObj is not a CredentialsBinding", func() {
			err := credentialsBindingValidator.Validate(ctx, &security.CredentialsBinding{}, &corev1.Secret{})
			Expect(err).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		It("should return err if the CredentialsBinding references unknown credentials type", func() {
			credentialsBindingSecret.CredentialsRef.APIVersion = "unknown"
			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(MatchError(errors.New(`unsupported credentials reference: version "unknown", kind "Secret"`)))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr)

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return err when the corresponding Secret does not contain a 'serviceaccount.json' field", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					secret := &corev1.Secret{Data: map[string][]byte{
						"foo": []byte("bar"),
					}}
					*obj = *secret
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(MatchError("referenced secret garden-dev/my-provider-account is not valid: missing \"serviceaccount.json\" field in secret"))
		})

		It("should return err when the corresponding Secret does not contain a valid 'serviceaccount.json' field", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					secret := &corev1.Secret{Data: map[string][]byte{
						gcp.ServiceAccountJSONField: []byte(``),
					}}
					*obj = *secret
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(MatchError("referenced secret garden-dev/my-provider-account is not valid: could not get service account from \"serviceaccount.json\" field: failed to unmarshal json object: unexpected end of JSON input"))
		})

		It("should succeed when the corresponding Secret is valid", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&corev1.Secret{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret, _ ...client.GetOption) error {
					secret := &corev1.Secret{Data: map[string][]byte{
						gcp.ServiceAccountJSONField: []byte(`{"project_id": "project", "type": "service_account"}`),
					}}
					*obj = *secret
					return nil
				})

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)).To(Succeed())
		})

		It("should return nil when the CredentialsBinding did not change", func() {
			old := credentialsBindingSecret.DeepCopy()

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, old)).To(Succeed())
		})

		It("should return err if it fails to get the corresponding WorkloadIdentity", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).Return(fakeErr)

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(MatchError(fakeErr))
		})

		It("should return err when the corresponding WorkloadIdentity is missing config for target system", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *securityv1alpha1.WorkloadIdentity, _ ...client.GetOption) error {
					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"foo"},
							TargetSystem: securityv1alpha1.TargetSystem{
								Type: "gcp",
							},
						},
					}
					*obj = *workloadIdentity
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(MatchError("the target system is missing configuration"))
		})

		It("should return err when the corresponding WorkloadIdentity has empty config for target system", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *securityv1alpha1.WorkloadIdentity, _ ...client.GetOption) error {
					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"foo"},
							TargetSystem: securityv1alpha1.TargetSystem{
								Type:           "gcp",
								ProviderConfig: &runtime.RawExtension{},
							},
						},
					}
					*obj = *workloadIdentity
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring("target system's configuration is not valid"))
		})

		It("should return err when the corresponding WorkloadIdentity has invalid target system configuration", func() {
			apiReader.EXPECT().Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&securityv1alpha1.WorkloadIdentity{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *securityv1alpha1.WorkloadIdentity, _ ...client.GetOption) error {
					workloadIdentity := &securityv1alpha1.WorkloadIdentity{
						Spec: securityv1alpha1.WorkloadIdentitySpec{
							Audiences: []string{"foo"},
							TargetSystem: securityv1alpha1.TargetSystem{
								Type: "gcp",
								ProviderConfig: &runtime.RawExtension{
									Raw: []byte(`
apiVersion: gcp.provider.extensions.gardener.cloud/v1alpha1
kind: WorkloadIdentityConfig
"projectID": "foo-"
"credentialsConfig": {
	"extra": "field",
	"type": "not_external_account",
	"audience": "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
	"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
	"token_url": "https://sts.googleapis.com/v1/token",
	"credential_source": {
		"file": "/abc/cloudprovider/xyz",
		"abc": {
			"foo": "text"
		}
	}
}
`),
								},
							},
						},
					}
					*obj = *workloadIdentity
					return nil
				})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring("referenced workload identity garden-dev/my-provider-account is not valid"))
		})
	})
})
