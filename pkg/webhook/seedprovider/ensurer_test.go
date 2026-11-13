// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seedprovider

import (
	"context"
	"testing"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apisgcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Seedprovider Webhook Suite")
}

var _ = Describe("Ensurer", func() {
	var (
		etcdStorage = &config.ETCDStorage{
			ClassName: ptr.To("gardener.cloud-fast"),
			Capacity:  ptr.To(resource.MustParse("25Gi")),
		}
		backupBucketName = "test-bb"

		ctx = context.TODO()

		dummyContext = gcontext.NewInternalGardenContext(&extensionscontroller.Cluster{
			Seed: &gardencorev1beta1.Seed{},
		})

		scheme *runtime.Scheme
	)

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcpv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcp.AddToScheme(scheme)).To(Succeed())
	})

	Describe("#EnsureETCD", func() {
		It("should add or modify elements to etcd-main statefulset", func() {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
			}

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, dummyContext, etcd, nil)).To(Succeed())
			checkETCDMainStorage(etcd)
		})

		It("should prefer the per-seed etcd storage class over the static default for etcd-main", func() {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
			}

			seedContext := gcontext.NewInternalGardenContext(&extensionscontroller.Cluster{
				Seed: &gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Volume: &gardencorev1beta1.SeedVolume{
							ETCDStorageClassName: ptr.To("gce-sc-hd-balanced"),
						},
					},
				},
			})

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, seedContext, etcd, nil)).To(Succeed())
			Expect(*etcd.Spec.StorageClass).To(Equal("gce-sc-hd-balanced"))
			Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("25Gi")))
		})

		It("should keep the static default for etcd-main when no per-seed etcd storage class is set", func() {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
			}

			seedContext := gcontext.NewInternalGardenContext(&extensionscontroller.Cluster{
				Seed: &gardencorev1beta1.Seed{
					Spec: gardencorev1beta1.SeedSpec{
						Volume: &gardencorev1beta1.SeedVolume{},
					},
				},
			})

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, seedContext, etcd, nil)).To(Succeed())
			checkETCDMainStorage(etcd)
		})

		It("should error when fetching backupbucket fails", func() {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
				Spec: druidcorev1alpha1.EtcdSpec{
					Backup: druidcorev1alpha1.BackupSpec{
						Store: &druidcorev1alpha1.StoreSpec{
							Container: ptr.To(backupBucketName),
						},
					},
				},
			}

			// BackupBucket not pre-populated → fake client returns not-found
			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, dummyContext, etcd, nil)).To(HaveOccurred())
		})

		It("should add or modify backup endpointOverride of the etcd spec", func() {
			backupBucket := &extensionsv1alpha1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: backupBucketName,
				},
				Spec: extensionsv1alpha1.BackupBucketSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig", "store": {"endpointOverride": "https://storage.me-central2.rep.googleapis.com"}}`),
						},
					},
				},
			}
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(backupBucket).Build()

			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
				Spec: druidcorev1alpha1.EtcdSpec{
					Backup: druidcorev1alpha1.BackupSpec{
						Store: &druidcorev1alpha1.StoreSpec{
							Container: ptr.To(backupBucketName),
						},
					},
				},
			}

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, dummyContext, etcd, nil)).To(Succeed())
			Expect(*etcd.Spec.Backup.Store.EndpointOverride).To(Equal("https://storage.me-central2.rep.googleapis.com"))
		})

		It("should modify existing elements of etcd-main statefulset", func() {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			r := resource.MustParse("10Gi")
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
				Spec: druidcorev1alpha1.EtcdSpec{
					StorageCapacity: &r,
				},
			}

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, dummyContext, etcd, nil)).To(Succeed())
			checkETCDMainStorage(etcd)
		})

		It("should modify existing backup endpointOverride of the etcd spec", func() {
			backupBucket := &extensionsv1alpha1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: backupBucketName,
				},
				Spec: extensionsv1alpha1.BackupBucketSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "BackupBucketConfig", "store": {"endpointOverride": "https://storage.me-central2.rep.googleapis.com"}}`),
						},
					},
				},
			}
			c := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(backupBucket).Build()

			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDMain},
				Spec: druidcorev1alpha1.EtcdSpec{
					Backup: druidcorev1alpha1.BackupSpec{
						Store: &druidcorev1alpha1.StoreSpec{
							Container:        ptr.To(backupBucketName),
							EndpointOverride: ptr.To("https://storage.me-central1.rep.googleapis.com"),
						},
					},
				},
			}

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, dummyContext, etcd, nil)).To(Succeed())
			Expect(*etcd.Spec.Backup.Store.EndpointOverride).To(Equal("https://storage.me-central2.rep.googleapis.com"))
		})

		It("should add or modify elements to etcd-events statefulset", func() {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
			}

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, dummyContext, etcd, nil)).To(Succeed())
			checkETCDEventsStorage(etcd)
		})

		It("should modify existing elements of etcd-events statefulset", func() {
			c := fakeclient.NewClientBuilder().WithScheme(scheme).Build()
			r := resource.MustParse("20Gi")
			etcd := &druidcorev1alpha1.Etcd{
				ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.ETCDEvents},
				Spec: druidcorev1alpha1.EtcdSpec{
					StorageCapacity: &r,
				},
			}

			ensurer := NewEnsurer(etcdStorage, c, logger)
			Expect(ensurer.EnsureETCD(ctx, dummyContext, etcd, nil)).To(Succeed())
			checkETCDEventsStorage(etcd)
		})
	})
})

func checkETCDMainStorage(etcd *druidcorev1alpha1.Etcd) {
	Expect(*etcd.Spec.StorageClass).To(Equal("gardener.cloud-fast"))
	Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("25Gi")))
}

func checkETCDEventsStorage(etcd *druidcorev1alpha1.Etcd) {
	Expect(*etcd.Spec.StorageClass).To(Equal(""))
	Expect(*etcd.Spec.StorageCapacity).To(Equal(resource.MustParse("10Gi")))
}
