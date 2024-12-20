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

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

const (
	errCodeBucketAlreadyOwnedByYou = 409
)

// StorageClient is an interface which must be implemented by GCS clients.
type StorageClient interface {
	// GCS wrappers
	Attrs(ctx context.Context, bucketName string) (*storage.BucketAttrs, error)
	CreateBucket(ctx context.Context, atts *storage.BucketAttrs) error
	UpdateBucket(ctx context.Context, bucketName string, bucketAttrsToUpdate storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error)
	LockBucket(ctx context.Context, bucketName string) error
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

// Attrs retrieves the attributes of the specified bucket.
// It returns a pointer to storage.BucketAttrs containing the bucket's attributes, or an error if the operation fails.
func (s *storageClient) Attrs(ctx context.Context, bucketName string) (*storage.BucketAttrs, error) {
	return s.client.Bucket(bucketName).Attrs(ctx)
}

// CreateBucket creates a new bucket with the specified attributes.
func (s *storageClient) CreateBucket(ctx context.Context, attrs *storage.BucketAttrs) error {
	return s.client.Bucket(attrs.Name).Create(ctx, s.serviceAccount.ProjectID, attrs)
}

// UpdateBucket updates the bucket with the specified attributes.
func (s *storageClient) UpdateBucket(ctx context.Context, bucketName string, bucketAttrsToUpdate storage.BucketAttrsToUpdate) (*storage.BucketAttrs, error) {
	return s.client.Bucket(bucketName).Update(ctx, bucketAttrsToUpdate)
}

// LockBucket locks the retention policy of the specified bucket.
func (s *storageClient) LockBucket(ctx context.Context, bucketName string) error {
	bucket := s.client.Bucket(bucketName)
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
