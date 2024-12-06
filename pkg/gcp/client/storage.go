// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

const (
	errCodeBucketAlreadyOwnedByYou = 409
)

// StorageClient is an interface which must be implemented by GCS clients.
type StorageClient interface {
	// GCS wrappers
	CreateOrUpdateBucket(ctx context.Context, bucketName, region string, config *apisgcp.BackupBucketConfig) error
	DeleteBucketIfExists(ctx context.Context, bucketName string) error
	DeleteObjectsWithPrefix(ctx context.Context, bucketName, prefix string) error
}

type storageClient struct {
	client         *storage.Client
	serviceAccount *gcp.ServiceAccount
}

// NewStorageClient creates a new storage client from the given  serviceAccount.
func NewStorageClient(ctx context.Context, serviceAccount *gcp.ServiceAccount) (StorageClient, error) {
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON(serviceAccount.Raw), option.WithScopes(storage.ScopeFullControl))
	if err != nil {
		return nil, err
	}
	return &storageClient{
		client:         client,
		serviceAccount: serviceAccount,
	}, nil
}

// NewStorageClientFromSecretRef creates a new storage client from the given <secretRef>.
func NewStorageClientFromSecretRef(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (StorageClient, error) {
	serviceAccount, err := gcp.GetServiceAccountFromSecretReference(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	return NewStorageClient(ctx, serviceAccount)
}

// CreateOrUpdateBucket ensures that a GCS bucket with the specified name exists in the given region,
// applying the provided configuration. If the bucket does not exist, it is created. If the bucket
// already exists and the immutability settings differ from the desired configuration, the retention
// policy is updated accordingly.
//
// The config parameter can include immutability settings, such as a retention period. If a retention
// policy is specified and not already locked, it will be locked to prevent further changes.
//
// Returns an error if the bucket creation or update fails.
func (s *storageClient) CreateOrUpdateBucket(ctx context.Context, bucketName, region string, config *apisgcp.BackupBucketConfig) error {
	bucket := s.client.Bucket(bucketName)
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		if err == storage.ErrBucketNotExist {
			return s.createBucket(ctx, bucket, region, config)
		}

		return fmt.Errorf("failed to get attributes for bucket %q: %w", bucket.BucketName(), err)
	}

	return s.updateBucketIfNeeded(ctx, bucket, attrs, config)
}

func (s *storageClient) createBucket(ctx context.Context, bucket *storage.BucketHandle, region string, config *apisgcp.BackupBucketConfig) error {
	var retentionPolicy *storage.RetentionPolicy
	if config != nil {
		retentionPolicy = &storage.RetentionPolicy{
			RetentionPeriod: config.Immutability.RetentionPeriod.Duration,
		}
	}

	bucketAttrs := &storage.BucketAttrs{
		Location:        region,
		RetentionPolicy: retentionPolicy,
		UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
			Enabled: true,
		},
		SoftDeletePolicy: &storage.SoftDeletePolicy{
			RetentionDuration: 0,
		},
	}

	if err := bucket.Create(ctx, s.serviceAccount.ProjectID, bucketAttrs); err != nil {
		return fmt.Errorf("failed to create bucket %q: %w", bucket.BucketName(), err)
	}

	// Lock the retention policy if specified
	if config != nil && config.Immutability.Locked {
		if err := s.lockBucketRetentionPolicy(ctx, bucket); err != nil {
			return fmt.Errorf("failed to lock retention policy for bucket %q: %w", bucket.BucketName(), err)
		}
	}

	return nil
}

func (s *storageClient) updateBucketIfNeeded(ctx context.Context, bucket *storage.BucketHandle, attrs *storage.BucketAttrs, config *apisgcp.BackupBucketConfig) error {
	var desiredRetentionPolicy *storage.RetentionPolicy
	if config != nil {
		desiredRetentionPolicy = &storage.RetentionPolicy{
			RetentionPeriod: config.Immutability.RetentionPeriod.Duration,
		}
	}

	// Determine if an update is required based on the desired and current retention policies.
	isUpdateRequired := true
	if desiredRetentionPolicy == nil && attrs.RetentionPolicy == nil {
		isUpdateRequired = false
	}

	if desiredRetentionPolicy != nil && attrs.RetentionPolicy != nil && *desiredRetentionPolicy == *attrs.RetentionPolicy {
		isUpdateRequired = false
	}

	// Perform the update if needed
	if isUpdateRequired {
		// If the desired retention policy is nil and the current retention policy is not nil,
		// it indicates that the retention policy needs to be removed. To achieve this, set
		// the RetentionPeriod to 0. This is required by the Google Cloud Storage API to
		// explicitly update and remove an existing retention policy.
		// For more details, refer to:
		// https://github.com/googleapis/google-cloud-go/blob/main/storage/bucket.go#L1172
		if desiredRetentionPolicy == nil && attrs.RetentionPolicy != nil {
			desiredRetentionPolicy = &storage.RetentionPolicy{}
		}
		bucketAttrsToUpdate := storage.BucketAttrsToUpdate{
			RetentionPolicy: desiredRetentionPolicy,
		}
		var err error
		attrs, err = bucket.Update(ctx, bucketAttrsToUpdate)
		if err != nil {
			return fmt.Errorf("failed to update retention policy for bucket %q: %w", bucket.BucketName(), err)
		}
	}

	// Lock the retention policy if specified and not already locked
	if config != nil && config.Immutability.Locked && !attrs.RetentionPolicy.IsLocked {
		if err := s.lockBucketRetentionPolicy(ctx, bucket); err != nil {
			return fmt.Errorf("failed to lock retention policy for bucket %q: %w", bucket.BucketName(), err)
		}
	}

	return nil
}

// lockBucketRetentionPolicy locks the retention policy of the specified bucket.
// It retrieves the bucket's attributes to obtain the current metageneration, which is required
// to lock the retention policy. If the bucket's retention policy is already locked, it returns nil.
//
// Returns an error if retrieving the bucket attributes or locking the retention policy fails.
func (s *storageClient) lockBucketRetentionPolicy(ctx context.Context, bucket *storage.BucketHandle) error {
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get attributes for bucket %q while attempting to lock retention policy: %w", bucket.BucketName(), err)
	}

	if err := bucket.If(storage.BucketConditions{MetagenerationMatch: attrs.MetaGeneration}).LockRetentionPolicy(ctx); err != nil {
		return fmt.Errorf("failed to lock retention policy for bucket %q: %w", bucket.BucketName(), err)
	}
	return nil
}

func (s *storageClient) DeleteBucketIfExists(ctx context.Context, bucketName string) error {
	err := s.client.Bucket(bucketName).Delete(ctx)
	return IgnoreNotFoundError(err)
}

func (s *storageClient) DeleteObjectsWithPrefix(ctx context.Context, bucketName, prefix string) error {
	bucketHandle := s.client.Bucket(bucketName)
	itr := bucketHandle.Objects(ctx, &storage.Query{Prefix: prefix})
	for {
		attr, err := itr.Next()
		if err != nil {
			if err == iterator.Done {
				return nil
			}
			return err
		}
		if err := bucketHandle.Object(attr.Name).Delete(ctx); err != nil && err != storage.ErrObjectNotExist {
			return err
		}
	}
}
