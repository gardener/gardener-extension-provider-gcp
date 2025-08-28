// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"
	"errors"
	"reflect"

	"cloud.google.com/go/storage"
	"github.com/gardener/gardener/extensions/pkg/controller/backupbucket"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

type actuator struct {
	client           client.Client
	gcpClientFactory gcpclient.Factory
}

var _ backupbucket.Actuator = (*actuator)(nil)

// NewActuator creates a new Actuator that manages BackupBucket resources.
func NewActuator(mgr manager.Manager, gcpClientFactory gcpclient.Factory) backupbucket.Actuator {
	return &actuator{
		client:           mgr.GetClient(),
		gcpClientFactory: gcpClientFactory,
	}
}

func (a *actuator) Reconcile(ctx context.Context, logger logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	logger.Info("Starting reconciliation for BackupBucket")

	storageClient, err := a.gcpClientFactory.Storage(ctx, a.client, bb.Spec.SecretRef)
	if err != nil {
		logger.Error(err, "Failed to create storage client")
		return util.DetermineError(err, helper.KnownCodes)
	}

	var backupBucketConfig *apisgcp.BackupBucketConfig
	if bb.Spec.ProviderConfig != nil {
		backupBucketConfig, err = admission.DecodeBackupBucketConfig(serializer.NewCodecFactory(a.client.Scheme(), serializer.EnableStrict).UniversalDecoder(), bb.Spec.ProviderConfig)
		if err != nil {
			logger.Error(err, "Failed to decode provider config")
			return err
		}
	}

	attrs, err := storageClient.Attrs(ctx, bb.Name)
	if err != nil && !errors.Is(err, storage.ErrBucketNotExist) {
		logger.Error(err, "Failed to fetch bucket attributes")
		return util.DetermineError(err, helper.KnownCodes)
	}

	if errors.Is(err, storage.ErrBucketNotExist) {
		attrs, err = createBucket(ctx, storageClient, bb, backupBucketConfig, logger)
		if err != nil {
			return err
		}
	} else if isUpdateRequired(attrs, backupBucketConfig, logger) {
		attrs, err = updateBucket(ctx, storageClient, bb.Name, backupBucketConfig, logger)
		if err != nil {
			return err
		}
	}

	if attrs.RetentionPolicy != nil && !attrs.RetentionPolicy.IsLocked &&
		backupBucketConfig != nil && backupBucketConfig.Immutability != nil &&
		backupBucketConfig.Immutability.Locked {
		err = lockBucket(ctx, storageClient, bb.Name, logger)
		if err != nil {
			return err
		}
	}

	logger.Info("Reconciliation completed successfully", "name", bb.Name)
	return nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	storageClient, err := a.gcpClientFactory.Storage(ctx, a.client, bb.Spec.SecretRef)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	return util.DetermineError(storageClient.DeleteBucketIfExists(ctx, bb.Name), helper.KnownCodes)
}

func createBucket(ctx context.Context, storageClient gcpclient.StorageClient, bb *extensionsv1alpha1.BackupBucket, config *apisgcp.BackupBucketConfig, logger logr.Logger) (*storage.BucketAttrs, error) {
	logger.Info("Bucket does not exist; creating", "name", bb.Name)
	attrs := &storage.BucketAttrs{
		Name:     bb.Name,
		Location: bb.Spec.Region,
		UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
			Enabled: true,
		},
		SoftDeletePolicy: &storage.SoftDeletePolicy{
			RetentionDuration: 0,
		},
		Lifecycle: storage.Lifecycle{
			Rules: []storage.LifecycleRule{
				{
					Action: storage.LifecycleAction{
						Type: storage.DeleteAction,
					},

					Condition: storage.LifecycleCondition{
						// DaysSinceCustomTime is the days elapsed since the CustomTime date of the object.
						// This condition can only be satisfied if CustomTime has been set.
						// Note: Using `0` as the value will be ignored by the library and not sent to the API.
						// That's why it is set to 1 day.
						DaysSinceCustomTime: 1,
					},
				},
			},
		},
	}

	if config != nil && config.Immutability != nil {
		attrs.RetentionPolicy = &storage.RetentionPolicy{
			RetentionPeriod: config.Immutability.RetentionPeriod.Duration,
		}
	}

	if err := storageClient.CreateBucket(ctx, attrs); err != nil {
		logger.Error(err, "Failed to create bucket", "name", bb.Name)
		return nil, util.DetermineError(err, helper.KnownCodes)
	}
	logger.Info("Bucket created successfully", "name", bb.Name)
	return attrs, nil
}

func updateBucket(ctx context.Context, storageClient gcpclient.StorageClient, bucketName string, config *apisgcp.BackupBucketConfig, logger logr.Logger) (*storage.BucketAttrs, error) {
	logger.Info("Updating bucket attributes", "name", bucketName)
	updateAttrs := storage.BucketAttrsToUpdate{
		Lifecycle: &storage.Lifecycle{
			Rules: []storage.LifecycleRule{
				{
					Action: storage.LifecycleAction{
						Type: storage.DeleteAction,
					},
					Condition: storage.LifecycleCondition{
						// DaysSinceCustomTime is the days elapsed since the CustomTime date of the object.
						// This condition can only be satisfied if CustomTime has been set.
						// Note: Using `0` as the value will be ignored by the library and not sent to the API.
						// That's why it is set to 1 day.
						DaysSinceCustomTime: 1,
					},
				},
			},
		},
		RetentionPolicy: &storage.RetentionPolicy{},
	}

	if config != nil && config.Immutability != nil {
		updateAttrs.RetentionPolicy.RetentionPeriod = config.Immutability.RetentionPeriod.Duration
	}

	attrs, err := storageClient.UpdateBucket(ctx, bucketName, updateAttrs)
	if err != nil {
		logger.Error(err, "Failed to update bucket", "name", bucketName)
		return nil, util.DetermineError(err, helper.KnownCodes)
	}
	logger.Info("Bucket updated successfully", "name", bucketName)
	return attrs, nil
}

func lockBucket(ctx context.Context, storageClient gcpclient.StorageClient, bucketName string, logger logr.Logger) error {
	logger.Info("Locking bucket", "name", bucketName)
	if err := storageClient.LockBucket(ctx, bucketName); err != nil {
		logger.Error(err, "Failed to lock bucket", "name", bucketName)
		return util.DetermineError(err, helper.KnownCodes)
	}
	logger.Info("Bucket locked successfully", "name", bucketName)
	return nil
}

// isUpdateRequired determines if the bucket attributes need an update based on the desired config.
func isUpdateRequired(attrs *storage.BucketAttrs, config *apisgcp.BackupBucketConfig, logger logr.Logger) bool {
	desiredLifecycle := storage.Lifecycle{
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

	lifecycleNeedsUpdate := !reflect.DeepEqual(desiredLifecycle, attrs.Lifecycle)
	// Determine if the retention policy needs an update
	var retentionPolicyNeedsUpdate bool
	if config == nil || config.Immutability == nil {
		// If config or immutability is nil, remove any existing retention policy
		retentionPolicyNeedsUpdate = attrs.RetentionPolicy != nil
	} else {
		desiredRetentionPeriod := config.Immutability.RetentionPeriod.Duration
		if attrs.RetentionPolicy == nil {
			retentionPolicyNeedsUpdate = true
		} else {
			retentionPolicyNeedsUpdate = attrs.RetentionPolicy.RetentionPeriod != desiredRetentionPeriod
		}
	}

	updateRequired := lifecycleNeedsUpdate || retentionPolicyNeedsUpdate
	logger.Info("Determined update requirement for bucket",
		"lifecycleNeedsUpdate", lifecycleNeedsUpdate,
		"retentionPolicyNeedsUpdate", retentionPolicyNeedsUpdate,
		"updateRequired", updateRequired)

	return updateRequired
}
