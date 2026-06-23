// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"
	"regexp"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apisgcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

var _ = Describe("Shoot validator", func() {
	Describe("#Validate", func() {
		const namespace = "garden-dev"

		var (
			shootValidator extensionswebhook.Validator

			scheme       *runtime.Scheme
			cloudProfile *gardencorev1beta1.CloudProfile
			shoot        *core.Shoot

			ctx = context.Background()
		)

		newValidator := func(clientObjs []client.Object, readerObjs []client.Object) {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(clientObjs...).Build()
			reader := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(readerObjs...).Build()
			mgr := &test.FakeManager{Scheme: scheme, Client: c, APIReader: reader}
			shootValidator = validator.NewShootValidator(
				mgr,
				[]string{"https://sts.googleapis.com/v1/token", "https://sts.googleapis.com/v1/token/new"},
				[]*regexp.Regexp{regexp.MustCompile(`^https://iamcredentials\.googleapis\.com/v1/projects/-/serviceAccounts/.+:generateAccessToken$`)},
			)
		}

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(apisgcp.AddToScheme(scheme)).To(Succeed())
			Expect(apisgcpv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
			Expect(corev1.AddToScheme(scheme)).To(Succeed())
			Expect(securityv1alpha1.AddToScheme(scheme)).To(Succeed())

			cloudProfile = &gardencorev1beta1.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gcp",
				},
				Spec: gardencorev1beta1.CloudProfileSpec{
					Regions: []gardencorev1beta1.Region{
						{
							Name:  "us-west",
							Zones: []gardencorev1beta1.AvailabilityZone{{Name: "zone1"}},
						},
					},
					ProviderConfig: &runtime.RawExtension{
						Raw: encode(&apisgcpv1alpha1.CloudProfileConfig{
							TypeMeta: metav1.TypeMeta{
								APIVersion: apisgcpv1alpha1.SchemeGroupVersion.String(),
								Kind:       "CloudProfileConfig",
							},
						}),
					},
				},
			}

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					CloudProfile: &core.CloudProfileReference{
						Name: "cloudProfile",
					},
					Provider: core.Provider{
						Type:    "gcp",
						Workers: []core.Worker{},
					},
					Region: "us-west",
					Networking: &core.Networking{
						Nodes: ptr.To("10.250.0.0/16"),
					},
					Kubernetes: core.Kubernetes{
						Version: "1.34.2",
					},
				},
			}

			newValidator([]client.Object{cloudProfile}, nil)
		})

		Context("Workerless Shoot", func() {
			BeforeEach(func() {
				shoot.Spec.Provider.Workers = nil
			})

			It("should not validate", func() {
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Shoot with workers", func() {
			BeforeEach(func() {
				shoot.Spec.CloudProfile = &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "gcp",
				}
				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
					Raw: encode(&apisgcpv1alpha1.InfrastructureConfig{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apisgcpv1alpha1.SchemeGroupVersion.String(),
							Kind:       "InfrastructureConfig",
						},
						Networks: apisgcpv1alpha1.NetworkConfig{
							Workers: "10.250.0.0/16",
						},
					}),
				}
				shoot.Spec.Provider.ControlPlaneConfig = &runtime.RawExtension{
					Raw: encode(&apisgcpv1alpha1.ControlPlaneConfig{
						TypeMeta: metav1.TypeMeta{
							APIVersion: apisgcpv1alpha1.SchemeGroupVersion.String(),
							Kind:       "ControlPlaneConfig",
						},
						Zone: "zone1",
					}),
				}
				shoot.Spec.Provider.Workers = []core.Worker{{
					Name: "worker-1",
					Volume: &core.Volume{
						VolumeSize: "50Gi",
						Type:       ptr.To("pd-standard"),
					},
					Zones: []string{"zone1"},
				}}
			})

			It("should return err when networking is invalid", func() {
				shoot.Spec.Networking.Nodes = nil
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}
				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.networking.nodes"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.networking.ipFamilies"),
					})),
				))
			})

			It("should return err with IPv6-only networking", func() {
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}
				shoot.Spec.Networking.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"overlay":{"enabled":false}}`),
				}

				err := shootValidator.Validate(ctx, shoot, nil)
				Expect(err).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.networking.ipFamilies"),
					})),
				))
			})

			Context("DNS provider (shoot.spec.dns.providers) credentials", func() {
				Context("#primaryProviders", func() {
					It("should skip validation for non google-clouddns providers", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{{
								Type:    ptr.To("aws-route53"),
								Primary: ptr.To(true),
								CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
									APIVersion: "v1",
									Kind:       "Secret",
									Name:       "other-secret",
								},
							}},
						}

						Expect(shootValidator.Validate(ctx, shoot, nil)).NotTo(HaveOccurred())
					})

					It("should validate only primary google-clouddns provider when multiple providers exist", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Primary: ptr.To(false),
									Type:    ptr.To("google-clouddns"),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "non-primary-secret",
									},
								},
								{
									Primary: ptr.To(true),
									Type:    ptr.To("google-clouddns"),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "primary-secret",
									},
								},
								{
									Type: ptr.To("google-clouddns"),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "another-non-primary",
									},
									Primary: nil, // nil means false
								},
							},
						}

						validSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "primary-secret", Namespace: namespace},
							Data: map[string][]byte{
								"serviceaccount.json": []byte(`{
				"type": "service_account",
				"project_id": "my-project-123",
				"private_key_id": "1234567890abcdef1234567890abcdef12345678",
				"private_key": "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
				"client_email": "my-sa@my-project-123.iam.gserviceaccount.com",
				"token_uri": "https://oauth2.googleapis.com/token"
			}`),
							},
						}

						newValidator([]client.Object{cloudProfile}, []client.Object{validSecret})
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})
				})

				Context("#credentialsRef", func() {
					It("should return error when google-clouddns provider has no credentialsRef", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"), // credentialsRef missing
									Primary: ptr.To(true)},
							},
						}

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("spec.dns.providers[0].credentialsRef"),
						}))))
					})

					It("should return error when credentialsRef points to non-existent Secret", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "missing-secret",
									},
								},
							},
						}

						// missing-secret not in reader → fake returns not-found
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotFound),
							"Field": Equal("spec.dns.providers[0].credentialsRef"),
						}))))
					})

					It("should return error when credentialsRef retrieval fails", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "dns-secret",
									},
								},
							},
						}

						// dns-secret not in reader → fake returns not-found
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotFound),
							"Field": Equal("spec.dns.providers[0].credentialsRef"),
						}))))
					})

					It("should return error when credentialsRef points to invalid Secret", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "invalid-secret",
									},
								},
							},
						}

						invalidSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "invalid-secret", Namespace: namespace},
							Data: map[string][]byte{
								"serviceaccount.json": []byte(`{
							"project_id": "my-project-123",
							"private_key_id": "1234567890abcdef1234567890abcdef12345678",
							"private_key": "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
							"client_email": "my-sa@my-project-123.iam.gserviceaccount.com",
							"token_uri": "https://oauth2.googleapis.com/token"
						}`),
							},
						}

						newValidator([]client.Object{cloudProfile}, []client.Object{invalidSecret})
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.dns.providers[0].credentialsRef"),
							"Detail": ContainSubstring("missing required field \"type\" in service account JSON"),
						}))))
					})

					It("should succeed with valid Secret referenced by credentialsRef", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "valid-secret",
									},
								},
							},
						}

						validSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "valid-secret", Namespace: namespace},
							Data: map[string][]byte{
								"serviceaccount.json": []byte(`{
							"type": "service_account",
							"project_id": "my-project-123",
							"private_key_id": "1234567890abcdef1234567890abcdef12345678",
							"private_key": "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
							"client_email": "my-sa@my-project-123.iam.gserviceaccount.com",
							"token_uri": "https://oauth2.googleapis.com/token"
						}`),
							},
						}

						newValidator([]client.Object{cloudProfile}, []client.Object{validSecret})
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should succeed with valid WorkloadIdentity referenced by credentialsRef", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "security.gardener.cloud/v1alpha1",
										Kind:       "WorkloadIdentity",
										Name:       "gcp-workload-identity",
									},
								},
							},
						}

						validWorkloadIdentity := &securityv1alpha1.WorkloadIdentity{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "gcp-workload-identity",
								Namespace: namespace,
							},
							Spec: securityv1alpha1.WorkloadIdentitySpec{
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
												"token_url": "https://sts.googleapis.com/v1/token",
												"service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/foo@bar.example:generateAccessToken"
											}
										}`),
									},
								},
								Audiences: []string{"https://kubernetes.cloud"},
							},
						}

						newValidator([]client.Object{cloudProfile}, []client.Object{validWorkloadIdentity})
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should return error when credentialsRef points to invalid WorkloadIdentity", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "security.gardener.cloud/v1alpha1",
										Kind:       "WorkloadIdentity",
										Name:       "invalid-workload-identity",
									},
								},
							},
						}

						invalidWorkloadIdentity := &securityv1alpha1.WorkloadIdentity{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "invalid-workload-identity",
								Namespace: namespace,
							},
							Spec: securityv1alpha1.WorkloadIdentitySpec{
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
												"token_url": "https://invalid.example.com/v1/token"
											}
										}`),
									},
								},
								Audiences: []string{"https://kubernetes.cloud"},
							},
						}

						newValidator([]client.Object{cloudProfile}, []client.Object{invalidWorkloadIdentity})
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.dns.providers[0].credentialsRef"),
							"Detail": ContainSubstring("spec.targetSystem.providerConfig.credentialsConfig.token_url: Forbidden: allowed values are [\"https://sts.googleapis.com/v1/token\" \"https://sts.googleapis.com/v1/token/new\"]"),
						}))))
					})

					It("should return error when credentialsRef points to unsupported resource type", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "core.gardener.cloud/v1beta1",
										Kind:       "InternalSecret",
										Name:       "some-internal-secret",
									},
								},
							},
						}

						internalSecret := &gardencorev1beta1.InternalSecret{
							ObjectMeta: metav1.ObjectMeta{Name: "some-internal-secret", Namespace: namespace},
							Data: map[string][]byte{
								"foo": []byte("bar"),
							},
						}

						newValidator([]client.Object{cloudProfile}, []client.Object{internalSecret})
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.dns.providers[0].credentialsRef"),
							"Detail": Equal("supported credentials types are Secret and WorkloadIdentity"),
						}))))
					})

					It("should skip non-primary google-clouddns providers with credentialsRef", func() {
						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(false),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "non-primary-secret",
									},
								},
							},
						}

						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})

					It("should validate multiple primary google-clouddns providers with credentialsRef", func() {
						validSecret := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{Name: "primary-secret", Namespace: namespace},
							Data: map[string][]byte{
								"serviceaccount.json": []byte(`{
							"type": "service_account",
							"project_id": "my-project-123",
							"private_key_id": "1234567890abcdef1234567890abcdef12345678",
							"private_key": "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
							"client_email": "my-sa@my-project-123.iam.gserviceaccount.com",
							"token_uri": "https://oauth2.googleapis.com/token"
						}`),
							},
						}

						validWorkloadIdentity := &securityv1alpha1.WorkloadIdentity{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "gcp-workload-identity",
								Namespace: namespace,
							},
							Spec: securityv1alpha1.WorkloadIdentitySpec{
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
												"token_url": "https://sts.googleapis.com/v1/token",
												"service_account_impersonation_url": "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/foo@bar.example:generateAccessToken"
											}
										}`),
									},
								},
								Audiences: []string{"https://kubernetes.cloud"},
							},
						}

						shoot.Spec.DNS = &core.DNS{
							Providers: []core.DNSProvider{
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "v1",
										Kind:       "Secret",
										Name:       "primary-secret",
									},
								},
								{
									Type:    ptr.To("google-clouddns"),
									Primary: ptr.To(true),
									CredentialsRef: &autoscalingv1.CrossVersionObjectReference{
										APIVersion: "security.gardener.cloud/v1alpha1",
										Kind:       "WorkloadIdentity",
										Name:       "gcp-workload-identity",
									},
								},
							},
						}

						newValidator([]client.Object{cloudProfile}, []client.Object{validSecret, validWorkloadIdentity})
						Expect(shootValidator.Validate(ctx, shoot, nil)).To(Succeed())
					})
				})
			})
		})
	})
})

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}
