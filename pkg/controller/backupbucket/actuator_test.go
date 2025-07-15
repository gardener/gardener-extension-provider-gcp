// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket_test

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apisgcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	. "github.com/gardener/gardener-extension-provider-gcp/pkg/controller/backupbucket"
	mockgcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client/mock"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl             *gomock.Controller
		c                *mockclient.MockClient
		sw               *mockclient.MockStatusWriter
		gcpClientFactory *mockgcpclient.MockFactory
		gcpStorageClient *mockgcpclient.MockStorageClient
		ctx              context.Context
		logger           logr.Logger
		a                backupbucket.Actuator
		mgr              *mockmanager.MockManager

		secretRef             = corev1.SecretReference{Name: "backup-gcp-ha", Namespace: "garden"}
		bucketName            = "test-bucket"
		region                = "europe-west1"
		immutabilityRetention = 24 * time.Hour
		desiredLifecycle      = storage.Lifecycle{
			Rules: []storage.LifecycleRule{
				{
					Action: storage.LifecycleAction{
						Type: storage.DeleteAction,
					},
					Condition: storage.LifecycleCondition{
						DaysSinceCustomTime: 1,
					},
				},
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		scheme := runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcpv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcp.AddToScheme(scheme)).To(Succeed())

		c = mockclient.NewMockClient(ctrl)
		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetClient().Return(c).AnyTimes()
		c.EXPECT().Scheme().Return(scheme).MaxTimes(1)

		sw = mockclient.NewMockStatusWriter(ctrl)
		gcpClientFactory = mockgcpclient.NewMockFactory(ctrl)
		gcpStorageClient = mockgcpclient.NewMockStorageClient(ctrl)

		c.EXPECT().Status().Return(sw).AnyTimes()
		sw.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		ctx = context.TODO()
		logger = log.Log.WithName("test")

		a = NewActuator(mgr, gcpClientFactory)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		var backupBucket *extensionsv1alpha1.BackupBucket

		Context("when bucket does not exist", func() {
			BeforeEach(func() {
				backupBucket = &extensionsv1alpha1.BackupBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bucketName,
						Namespace: "garden",
					},
					Spec: extensionsv1alpha1.BackupBucketSpec{
						SecretRef: secretRef,
						Region:    region,
					},
				}
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h"}}`),
				}
			})

			It("should create the bucket", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(nil, storage.ErrBucketNotExist).MaxTimes(2)
				gcpStorageClient.EXPECT().CreateBucket(ctx, gomock.Any()).Return(nil)
				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return error if creating bucket fails", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(nil, storage.ErrBucketNotExist)
				gcpStorageClient.EXPECT().CreateBucket(ctx, gomock.Any()).Return(fmt.Errorf("creation error"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when bucket already exists", func() {
			BeforeEach(func() {
				backupBucket = &extensionsv1alpha1.BackupBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bucketName,
						Namespace: "garden",
					},
					Spec: extensionsv1alpha1.BackupBucketSpec{
						SecretRef: secretRef,
						Region:    region,
					},
				}
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h"}}`),
				}
			})

			It("should not make any changes if the bucket has the correct lifecycle and retention policies", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
						Enabled: true,
					},
					SoftDeletePolicy: &storage.SoftDeletePolicy{
						RetentionDuration: 0,
					},
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
					},
					Lifecycle: desiredLifecycle,
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the bucket if the lifecycle policy is different", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
						Enabled: true,
					},
					SoftDeletePolicy: &storage.SoftDeletePolicy{
						RetentionDuration: 0,
					},
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
					},
					Lifecycle: storage.Lifecycle{}, // Incorrect lifecycle
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().UpdateBucket(ctx, bucketName, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, updateAttrs storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error) {
					Expect(updateAttrs.Lifecycle).To(Equal(&desiredLifecycle))
					return &storage.BucketAttrs{
						Location:         region,
						Lifecycle:        desiredLifecycle,
						RetentionPolicy:  existingAttrs.RetentionPolicy,
						SoftDeletePolicy: existingAttrs.SoftDeletePolicy,
						UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
							Enabled: true,
						},
					}, nil
				})

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the bucket if the retention policy is different", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
						Enabled: true,
					},
					SoftDeletePolicy: &storage.SoftDeletePolicy{
						RetentionDuration: 0,
					},
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention + 1*time.Hour, // Different retention
					},
					Lifecycle: desiredLifecycle,
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().UpdateBucket(ctx, bucketName, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, updateAttrs storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error) {
					Expect(updateAttrs.RetentionPolicy).To(Equal(&storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
					}))
					return &storage.BucketAttrs{
						Location:         region,
						Lifecycle:        desiredLifecycle,
						RetentionPolicy:  &storage.RetentionPolicy{RetentionPeriod: immutabilityRetention},
						SoftDeletePolicy: existingAttrs.SoftDeletePolicy,
						UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
							Enabled: true,
						},
					}, nil
				})

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the bucket if both lifecycle and retention policies are different", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
						Enabled: true,
					},
					SoftDeletePolicy: &storage.SoftDeletePolicy{
						RetentionDuration: 0,
					},
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention + 1*time.Hour, // Different retention
					},
					Lifecycle: storage.Lifecycle{}, // Incorrect lifecycle
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().UpdateBucket(ctx, bucketName, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, updateAttrs storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error) {
					Expect(updateAttrs.Lifecycle).To(Equal(&desiredLifecycle))
					Expect(updateAttrs.RetentionPolicy).To(Equal(&storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
					}))
					return &storage.BucketAttrs{
						Location:         region,
						Lifecycle:        desiredLifecycle,
						RetentionPolicy:  &storage.RetentionPolicy{RetentionPeriod: immutabilityRetention},
						SoftDeletePolicy: existingAttrs.SoftDeletePolicy,
						UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
							Enabled: true,
						},
					}, nil
				})

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error if updating the bucket fails", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
						Enabled: true,
					},
					SoftDeletePolicy: &storage.SoftDeletePolicy{
						RetentionDuration: 0,
					},
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention + 1*time.Hour,
					},
					Lifecycle: storage.Lifecycle{},
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().UpdateBucket(ctx, bucketName, gomock.Any()).Return(nil, fmt.Errorf("update error"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).To(HaveOccurred())
			})

			It("should update the bucket to remove retention policy if ProviderConfig is nil", func() {

				backupBucket.Spec.ProviderConfig = nil

				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
						Enabled: true,
					},
					SoftDeletePolicy: &storage.SoftDeletePolicy{
						RetentionDuration: 0,
					},
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
					},
					Lifecycle: desiredLifecycle,
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().UpdateBucket(ctx, bucketName, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, updateAttrs storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error) {
					Expect(updateAttrs.RetentionPolicy).Should(BeEquivalentTo(&storage.RetentionPolicy{}))
					return &storage.BucketAttrs{
						Location:         region,
						Lifecycle:        desiredLifecycle,
						RetentionPolicy:  &storage.RetentionPolicy{},
						SoftDeletePolicy: existingAttrs.SoftDeletePolicy,
						UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
							Enabled: true,
						},
					}, nil
				})

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the bucket to remove retention policy when BackupBucketConfig does not include immutability settings", func() {

				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig"}`),
				}

				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
						Enabled: true,
					},
					SoftDeletePolicy: &storage.SoftDeletePolicy{
						RetentionDuration: 0,
					},
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
					},
					Lifecycle: desiredLifecycle,
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().UpdateBucket(ctx, bucketName, gomock.Any()).DoAndReturn(func(_ context.Context, _ string, updateAttrs storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error) {
					Expect(updateAttrs.RetentionPolicy).Should(BeEquivalentTo(&storage.RetentionPolicy{}))
					return &storage.BucketAttrs{
						Location:         region,
						Lifecycle:        desiredLifecycle,
						RetentionPolicy:  &storage.RetentionPolicy{},
						SoftDeletePolicy: existingAttrs.SoftDeletePolicy,
						UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
							Enabled: true,
						},
					}, nil
				})

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when bucket must be locked", func() {
			BeforeEach(func() {
				backupBucket = &extensionsv1alpha1.BackupBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bucketName,
						Namespace: "garden",
					},
					Spec: extensionsv1alpha1.BackupBucketSpec{
						SecretRef: secretRef,
						Region:    region,
					},
				}
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig","immutability":{"retentionType":"bucket","retentionPeriod":"24h","locked":true}}`),
				}
			})

			It("should lock the bucket if required", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
						IsLocked:        false,
					},
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{Enabled: true},
					SoftDeletePolicy:         &storage.SoftDeletePolicy{RetentionDuration: 0},
					Lifecycle:                desiredLifecycle,
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().LockBucket(ctx, bucketName).Return(nil)

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error if locking fails", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				existingAttrs := &storage.BucketAttrs{
					Location: region,
					RetentionPolicy: &storage.RetentionPolicy{
						RetentionPeriod: immutabilityRetention,
						IsLocked:        false,
					},
					UniformBucketLevelAccess: storage.UniformBucketLevelAccess{Enabled: true},
					SoftDeletePolicy:         &storage.SoftDeletePolicy{RetentionDuration: 0},
					Lifecycle:                desiredLifecycle,
				}
				gcpStorageClient.EXPECT().Attrs(ctx, bucketName).Return(existingAttrs, nil)
				gcpStorageClient.EXPECT().LockBucket(ctx, bucketName).Return(fmt.Errorf("lock error"))

				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when providerConfig cannot be decoded", func() {
			BeforeEach(func() {
				backupBucket = &extensionsv1alpha1.BackupBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bucketName,
						Namespace: "garden",
					},
					Spec: extensionsv1alpha1.BackupBucketSpec{
						SecretRef: secretRef,
						Region:    region,
					},
				}
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig","immutability": { "retentionPeriod": "abc" }}`),
				}
			})

			It("should return an error if decoding fails", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when creating storage client fails", func() {
			BeforeEach(func() {
				backupBucket = &extensionsv1alpha1.BackupBucket{
					ObjectMeta: metav1.ObjectMeta{
						Name:      bucketName,
						Namespace: "garden",
					},
					Spec: extensionsv1alpha1.BackupBucketSpec{
						SecretRef: secretRef,
						Region:    region,
					},
				}
				backupBucket.Spec.ProviderConfig = &runtime.RawExtension{
					Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig","immutability":{"retentionPeriod":"24h"}}`),
				}
			})

			It("should return an error if storage client creation fails", func() {
				gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(nil, fmt.Errorf("client error"))
				err := a.Reconcile(ctx, logger, backupBucket)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("#Delete", func() {
		var backupBucket *extensionsv1alpha1.BackupBucket

		BeforeEach(func() {
			backupBucket = &extensionsv1alpha1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bucketName,
					Namespace: "garden",
				},
				Spec: extensionsv1alpha1.BackupBucketSpec{
					SecretRef: secretRef,
					Region:    region,
				},
			}
		})

		It("should delete the bucket successfully", func() {
			gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
			gcpStorageClient.EXPECT().DeleteBucketIfExists(ctx, bucketName).Return(nil)

			err := a.Delete(ctx, logger, backupBucket)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if deleting bucket fails", func() {
			gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(gcpStorageClient, nil)
			gcpStorageClient.EXPECT().DeleteBucketIfExists(ctx, bucketName).Return(fmt.Errorf("deletion error"))

			err := a.Delete(ctx, logger, backupBucket)
			Expect(err).To(HaveOccurred())
		})

		It("should return error if storage client creation fails on delete", func() {
			gcpClientFactory.EXPECT().Storage(ctx, c, secretRef).Return(nil, fmt.Errorf("client error"))
			err := a.Delete(ctx, logger, backupBucket)
			Expect(err).To(HaveOccurred())
		})
	})
})
