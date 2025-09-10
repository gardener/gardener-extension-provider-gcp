// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

// backupBucketValidator validates create and update operations on BackupBucket resources,
type backupBucketValidator struct {
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// NewBackupBucketValidator returns a new instance of backupBucket validator.
func NewBackupBucketValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &backupBucketValidator{
		decoder:        serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		lenientDecoder: serializer.NewCodecFactory(mgr.GetScheme()).UniversalDecoder(),
	}
}

// Validate validates the BackupBucket resource during create or update operations.
func (s *backupBucketValidator) Validate(_ context.Context, newObj, oldObj client.Object) error {
	newBackupBucket, ok := newObj.(*gardencore.BackupBucket)
	if !ok {
		return fmt.Errorf("wrong object type %T for new object", newObj)
	}

	if oldObj != nil {
		oldBackupBucket, ok := oldObj.(*gardencore.BackupBucket)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}
		return s.validateUpdate(oldBackupBucket, newBackupBucket).ToAggregate()
	}

	return s.validateCreate(newBackupBucket).ToAggregate()
}

// validateCreate validates the BackupBucket object upon creation.
func (b *backupBucketValidator) validateCreate(backupBucket *gardencore.BackupBucket) field.ErrorList {
	var (
		allErrs            = field.ErrorList{}
		providerConfigPath = field.NewPath("spec", "providerConfig")
	)

	config, err := admission.DecodeBackupBucketConfig(b.decoder, backupBucket.Spec.ProviderConfig)
	if err != nil {
		return append(allErrs, field.InternalError(providerConfigPath, fmt.Errorf("failed to decode provider config: %w", err)))
	}

	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketConfig(config, providerConfigPath)...)
	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketCredentialsRef(backupBucket.Spec.CredentialsRef, field.NewPath("spec", "credentialsRef"))...)

	return allErrs
}

// validateUpdate validates updates to the BackupBucket resource.
func (b *backupBucketValidator) validateUpdate(oldBackupBucket, backupBucket *gardencore.BackupBucket) field.ErrorList {
	var (
		allErrs            = field.ErrorList{}
		providerConfigPath = field.NewPath("spec", "providerConfig")
	)

	if oldBackupBucket.Spec.ProviderConfig == nil {
		return b.validateCreate(backupBucket)
	}

	oldConfig, err := admission.DecodeBackupBucketConfig(b.lenientDecoder, oldBackupBucket.Spec.ProviderConfig)
	if err != nil {
		return append(allErrs, field.InternalError(providerConfigPath, fmt.Errorf("failed to decode old provider config: %W", err)))
	}

	config, err := admission.DecodeBackupBucketConfig(b.decoder, backupBucket.Spec.ProviderConfig)
	if err != nil {
		return append(allErrs, field.InternalError(providerConfigPath, fmt.Errorf("failed to decode new provider config: %W", err)))
	}

	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketConfig(config, providerConfigPath)...)
	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketConfigUpdate(oldConfig, config, providerConfigPath)...)
	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketCredentialsRef(backupBucket.Spec.CredentialsRef, field.NewPath("spec", "credentialsRef"))...)

	return allErrs
}
