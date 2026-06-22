// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"regexp"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
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

			mgr *test.FakeManager

			ctx                                = context.TODO()
			credentialsBindingSecret           *security.CredentialsBinding
			credentialsBindingWorkloadIdentity *security.CredentialsBinding
			credentialsBindingInternalSecret   *security.CredentialsBinding

			scheme *runtime.Scheme
		)

		newValidator := func(objs ...client.Object) {
			builder := fakeclient.NewClientBuilder().WithScheme(scheme)
			for _, obj := range objs {
				builder = builder.WithObjects(obj)
			}
			apiReader := builder.Build()
			mgr = &test.FakeManager{APIReader: apiReader}
			credentialsBindingValidator = validator.NewCredentialsBindingValidator(
				mgr,
				[]string{"https://sts.googleapis.com/v1/token", "https://sts.googleapis.com/v1/token/new"},
				[]*regexp.Regexp{regexp.MustCompile(`^https://iamcredentials\.googleapis\.com/v1/projects/-/serviceAccounts/.+:generateAccessToken$`)},
			)
		}

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())

			newValidator()

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
			credentialsBindingInternalSecret = &security.CredentialsBinding{
				CredentialsRef: corev1.ObjectReference{
					Name:       name,
					Namespace:  namespace,
					Kind:       "InternalSecret",
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
				},
			}
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
			Expect(err).To(MatchError(ContainSubstring("unsupported credentials reference")))
		})

		It("should return err if it fails to get the corresponding Secret", func() {
			// Secret not pre-populated → fake client returns not-found
			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should return err when the corresponding Secret does not contain a 'serviceaccount.json' field", func() {
			newValidator(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data:       map[string][]byte{"foo": []byte("bar")},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("secret.data[serviceaccount.json]"),
				"Detail": Equal("missing required field \"serviceaccount.json\" in secret garden-dev/my-provider-account"),
			}))))
		})

		It("should return err when the corresponding Secret does not contain a valid 'serviceaccount.json' field", func() {
			newValidator(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Data:       map[string][]byte{gcp.ServiceAccountJSONField: []byte(``)},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("secret.data[serviceaccount.json]"),
				"Detail": Equal("field \"serviceaccount.json\" cannot be empty in secret garden-dev/my-provider-account"),
			}))))
		})

		It("should succeed when the corresponding Secret is valid", func() {
			newValidator(&corev1.Secret{
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
			})

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, nil)).To(Succeed())
		})

		It("should return nil when the CredentialsBinding did not change", func() {
			old := credentialsBindingSecret.DeepCopy()
			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingSecret, old)).To(Succeed())
		})

		It("should succeed when the corresponding WorkloadIdentity is valid", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "gcp",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1",
								"kind": "WorkloadIdentityConfig",
								"projectID": "foo-valid",
								"credentialsConfig": {
									"universe_domain": "googleapis.com",
									"type": "external_account",
									"audience": "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
									"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
									"token_url": "https://sts.googleapis.com/v1/token/new"
								}
							}`),
						},
					},
				},
			})

			Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)).To(Succeed())
		})

		It("should return err if it fails to get the corresponding WorkloadIdentity", func() {
			// WorkloadIdentity not pre-populated → fake client returns not-found
			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(HaveOccurred())
		})

		It("should return err when the corresponding WorkloadIdentity is missing config for target system", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "gcp",
					},
				},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err).To(MatchError("the target system is missing configuration"))
		})

		It("should return err when the corresponding WorkloadIdentity has invalid target system configuration", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "gcp",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1",
								"kind": "WorkloadIdentityConfig",
								"projectID": "foo-",
								"credentialsConfig": {
									"type": "not_external_account",
									"audience": "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
									"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
									"token_url": "https://sts.googleapis.com/v1/token",
									"credential_source": {"file": "/abc/cloudprovider/xyz"}
								}
							}`),
						},
					},
				},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring("referenced workload identity garden-dev/my-provider-account is not valid"))
		})

		It("should succeed when the corresponding WorkloadIdentity is valid", func() {
			newValidator(&securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: securityv1alpha1.WorkloadIdentitySpec{
					Audiences: []string{"foo"},
					TargetSystem: securityv1alpha1.TargetSystem{
						Type: "gcp",
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{
								"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1",
								"kind": "WorkloadIdentityConfig",
								"projectID": "foo-valid",
								"credentialsConfig": {
									"universe_domain": "googleapis.com",
									"type": "external_account",
									"audience": "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
									"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
									"token_url": "https://sts.googleapis.com/v1/token/forbidden"
								}
							}`),
						},
					},
				},
			})

			err := credentialsBindingValidator.Validate(ctx, credentialsBindingWorkloadIdentity, nil)
			Expect(err.Error()).To(ContainSubstring(`spec.targetSystem.providerConfig.credentialsConfig.token_url: Forbidden: allowed values are ["https://sts.googleapis.com/v1/token" "https://sts.googleapis.com/v1/token/new"]`))
		})

		Context("InternalSecret", func() {
			It("should return err if it fails to get the corresponding InternalSecret", func() {
				// InternalSecret not pre-populated → fake client returns not-found
				err := credentialsBindingValidator.Validate(ctx, credentialsBindingInternalSecret, nil)
				Expect(err).To(HaveOccurred())
			})

			It("should return err when the corresponding InternalSecret does not contain a valid 'serviceaccount.json' field", func() {
				newValidator(&gardencorev1beta1.InternalSecret{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data:       map[string][]byte{"foo": []byte("bar")},
				})

				err := credentialsBindingValidator.Validate(ctx, credentialsBindingInternalSecret, nil)
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("secret.data[serviceaccount.json]"),
					"Detail": ContainSubstring("missing required field \"serviceaccount.json\""),
				}))))
			})

			It("should succeed when the corresponding InternalSecret is valid", func() {
				newValidator(&gardencorev1beta1.InternalSecret{
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
				})

				Expect(credentialsBindingValidator.Validate(ctx, credentialsBindingInternalSecret, nil)).To(Succeed())
			})
		})
	})
})
