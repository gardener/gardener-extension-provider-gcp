// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	"context"

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
	backupbucket.Actuator
	client           client.Client
	gcpClientFactory gcpclient.Factory
}

// NewActuator creates a new Actuator that manages BackupBucket resources.
func NewActuator(mgr manager.Manager, gcpClientFactory gcpclient.Factory) backupbucket.Actuator {
	return &actuator{
		client:           mgr.GetClient(),
		gcpClientFactory: gcpClientFactory,
	}
}

func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	storageClient, err := a.gcpClientFactory.Storage(ctx, a.client, bb.Spec.SecretRef)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	backupBucketConfig, err := admission.DecodeBackupBucketConfig(serializer.NewCodecFactory(a.client.Scheme(), serializer.EnableStrict).UniversalDecoder(), bb.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	attrs, err := storageClient.Attrs(ctx, bb.Name)

	if err != nil && err != storage.ErrBucketNotExist {
		return util.DetermineError(err, helper.KnownCodes)
	}

	if err != nil {
		if err == storage.ErrBucketNotExist {
			attrs = &storage.BucketAttrs{
				Location: bb.Spec.Region,
				UniformBucketLevelAccess: storage.UniformBucketLevelAccess{
					Enabled: true,
				},
				SoftDeletePolicy: &storage.SoftDeletePolicy{
					RetentionDuration: 0,
				},
			}
			err = storageClient.CreateBucket(ctx, attrs)
			if err != nil {
				return util.DetermineError(err, helper.KnownCodes)
			}

		}
		return err
	} else if isUpdateRequired(attrs, backupBucketConfig) {

		updateAttrs := storage.BucketAttrsToUpdate{
			RetentionPolicy: attrs.RetentionPolicy,
		}
		attrs, err = storageClient.UpdateBucket(ctx, bb.Name, updateAttrs)
		if err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	if attrs.RetentionPolicy != nil && backupBucketConfig.Immutability.Locked && !attrs.RetentionPolicy.IsLocked {
		if err := storageClient.LockBucket(ctx, bb.Name); err != nil {
			return util.DetermineError(err, helper.KnownCodes)
		}
	}

	return nil
}

func (a *actuator) Delete(ctx context.Context, _ logr.Logger, bb *extensionsv1alpha1.BackupBucket) error {
	storageClient, err := a.gcpClientFactory.Storage(ctx, a.client, bb.Spec.SecretRef)
	if err != nil {
		return util.DetermineError(err, helper.KnownCodes)
	}

	return util.DetermineError(storageClient.DeleteBucketIfExists(ctx, bb.Name), helper.KnownCodes)
}

func isUpdateRequired(attrs *storage.BucketAttrs, config *apisgcp.BackupBucketConfig) bool {
	var desiredRetentionPolicy *storage.RetentionPolicy
	if config != nil {
		desiredRetentionPolicy = &storage.RetentionPolicy{
			RetentionPeriod: config.Immutability.RetentionPeriod.Duration,
		}
	}

	if desiredRetentionPolicy == nil && attrs.RetentionPolicy == nil {
		return false
	}

	if desiredRetentionPolicy != nil && attrs.RetentionPolicy != nil && *desiredRetentionPolicy == *attrs.RetentionPolicy {
		return false
	}
	return true
}
