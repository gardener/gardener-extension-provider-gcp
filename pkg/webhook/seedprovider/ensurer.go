// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedprovider

import (
	"context"
	"fmt"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

// NewEnsurer creates a new seedprovider ensurer.
func NewEnsurer(etcdStorage *config.ETCDStorage, client client.Client, logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		etcdStorage: etcdStorage,
		logger:      logger.WithName("gcp-seedprovider-ensurer"),
		client:      client,
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	etcdStorage *config.ETCDStorage
	logger      logr.Logger
	client      client.Client
}

// EnsureETCD ensures that the etcd conform to the provider requirements.
func (e *ensurer) EnsureETCD(ctx context.Context, _ gcontext.GardenContext, newEtcd, _ *druidcorev1alpha1.Etcd) error {
	capacity := resource.MustParse("10Gi")
	class := ""

	if newEtcd.Name == v1beta1constants.ETCDMain && e.etcdStorage != nil {
		if e.etcdStorage.Capacity != nil {
			capacity = *e.etcdStorage.Capacity
		}
		if e.etcdStorage.ClassName != nil {
			class = *e.etcdStorage.ClassName
		}
	}

	newEtcd.Spec.StorageClass = &class
	newEtcd.Spec.StorageCapacity = &capacity

	if newEtcd.Spec.Backup.Store == nil || newEtcd.Spec.Backup.Store.Container == nil {
		return nil
	}

	backupBucket := &extensionsv1alpha1.BackupBucket{}
	if err := e.client.Get(ctx, client.ObjectKey{Name: *newEtcd.Spec.Backup.Store.Container}, backupBucket); err != nil {
		return fmt.Errorf("failed to fetch the seed's backupbucket to find the endpoint with error: %w", err)
	}

	backupBucketConfig, err := helper.BackupBucketConfigFromBackupBucket(backupBucket)
	if err != nil {
		return fmt.Errorf("failed to decode backupbucketconfig from backupbucket resource with error: %w", err)
	}

	newEtcd.Spec.Backup.Store.Endpoint = backupBucketConfig.Endpoint

	return nil
}
