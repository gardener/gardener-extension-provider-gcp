// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apisgcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

var _ = Describe("BackupBucket Validator", func() {
	Describe("#Validate", func() {
		var (
			ctx                   context.Context
			credentialsRef        *corev1.ObjectReference
			ctrl                  *gomock.Controller
			mgr                   *mockmanager.MockManager
			scheme                *runtime.Scheme
			backupBucketValidator extensionswebhook.Validator
		)

		BeforeEach(func() {
			ctx = context.TODO()
			credentialsRef = &corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "backup-credentials",
				Namespace:  "garden",
			}

			ctrl = gomock.NewController(GinkgoT())
			scheme = runtime.NewScheme()
			Expect(gardencore.AddToScheme(scheme)).To(Succeed())
			Expect(apisgcpv1alpha1.AddToScheme(scheme)).To(Succeed())
			Expect(apisgcp.AddToScheme(scheme)).To(Succeed())

			mgr = mockmanager.NewMockManager(ctrl)
			mgr.EXPECT().GetScheme().Return(scheme).AnyTimes()

			backupBucketValidator = validator.NewBackupBucketValidator(mgr)
		})

		It("should return err when obj is not a gardencore.BackupBucket", func() {
			err := backupBucketValidator.Validate(ctx, &corev1.Secret{}, nil)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("wrong object type *v1.Secret for new object"))
		})

		It("should return err when oldObj is not a gardencore.BackupBucket", func() {
			err := backupBucketValidator.Validate(ctx, &gardencore.BackupBucket{}, &corev1.Secret{})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError("wrong object type *v1.Secret for old object"))
		})

		Context("Create", func() {
			It("should return error when BackupBucket provider config cannot be decoded", func() {
				backupBucket := &gardencore.BackupBucket{
					Spec: gardencore.BackupBucketSpec{
						CredentialsRef: credentialsRef,
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`invalid`),
						},
					},
				}

				err := backupBucketValidator.Validate(ctx, backupBucket, nil)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`failed to decode provider config:`)))
			})

			It("should succeed when BackupBucket is created with valid spec", func() {
				backupBucket := &gardencore.BackupBucket{
					Spec: gardencore.BackupBucketSpec{
						CredentialsRef: credentialsRef,
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
						},
					},
				}

				Expect(backupBucketValidator.Validate(ctx, backupBucket, nil)).To(Succeed())
			})
		})

		Context("Update", func() {
			It("should return error when BackupBucket is updated with invalid spec and old had unset providerConfig", func() {
				backupBucket := &gardencore.BackupBucket{
					Spec: gardencore.BackupBucketSpec{
						CredentialsRef: credentialsRef,
						ProviderConfig: nil,
					},
				}

				newBackupBucket := backupBucket.DeepCopy()
				newBackupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "invalid"}`),
				}

				err := backupBucketValidator.Validate(ctx, newBackupBucket, backupBucket)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring(`failed to decode provider config: no kind "invalid" is registered for version "gcp.provider.extensions.gardener.cloud/v1alpha1" in scheme`)))
			})

			It("should succeed when BackupBucket is updated with valid spec and old had unset providerConfig", func() {
				backupBucket := &gardencore.BackupBucket{
					Spec: gardencore.BackupBucketSpec{
						CredentialsRef: credentialsRef,
						ProviderConfig: nil,
					},
				}

				newBackupBucket := backupBucket.DeepCopy()
				newBackupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
				}

				Expect(backupBucketValidator.Validate(ctx, newBackupBucket, backupBucket)).To(Succeed())
			})

			It("should return error when BackupBucket is updated and old had invalid providerConfig set", func() {
				backupBucket := &gardencore.BackupBucket{
					Spec: gardencore.BackupBucketSpec{
						CredentialsRef: credentialsRef,
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "invalid"}`),
						},
					},
				}

				newBackupBucket := backupBucket.DeepCopy()
				newBackupBucket.Spec.CredentialsRef = nil

				err := backupBucketValidator.Validate(ctx, newBackupBucket, backupBucket)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to decode old provider config: no kind \"invalid\" is registered for version \"gcp.provider.extensions.gardener.cloud/v1alpha1\" in scheme")))
			})

			It("should return error when BackupBucket is updated with invalid providerConfig and old had valid providerConfig set", func() {
				backupBucket := &gardencore.BackupBucket{
					Spec: gardencore.BackupBucketSpec{
						CredentialsRef: credentialsRef,
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
						},
					},
				}

				newBackupBucket := backupBucket.DeepCopy()
				newBackupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "invalid"}`),
				}

				err := backupBucketValidator.Validate(ctx, newBackupBucket, backupBucket)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(ContainSubstring("failed to decode new provider config: no kind \"invalid\" is registered for version \"gcp.provider.extensions.gardener.cloud/v1alpha1\" in scheme")))
			})

			It("should succeed when BackupBucket is updated with valid spec and old had valid providerConfig set", func() {
				backupBucket := &gardencore.BackupBucket{
					Spec: gardencore.BackupBucketSpec{
						CredentialsRef: credentialsRef,
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig", "immutability": {"retentionPeriod": "96h", "retentionType": "bucket"}}`),
						},
					},
				}

				newBackupBucket := backupBucket.DeepCopy()
				newBackupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig"}`),
				}

				Expect(backupBucketValidator.Validate(ctx, newBackupBucket, backupBucket)).To(Succeed())
			})
		})
	})
})
