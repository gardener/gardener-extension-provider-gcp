// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission"
	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

// NewSeedValidator returns a new Validator for Seed resources,
// ensuring backup configuration immutability according to policy.
func NewSeedValidator(mgr manager.Manager) extensionswebhook.Validator {
	return &seedValidator{
		client:         mgr.GetClient(),
		decoder:        serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
		lenientDecoder: serializer.NewCodecFactory(mgr.GetScheme()).UniversalDecoder(),
	}
}

// seedValidator validates create and update operations on Seed resources,
// enforcing immutability of backup configurations.
type seedValidator struct {
	client         client.Client
	decoder        runtime.Decoder
	lenientDecoder runtime.Decoder
}

// Validate validates the Seed resource during create or update operations.
// It enforces immutability policies on backup configurations to prevent
// disabling immutable settings, reducing retention periods, or changing retention types.
func (s *seedValidator) Validate(_ context.Context, newObj, oldObj client.Object) error {
	newSeed, ok := newObj.(*core.Seed)
	if !ok {
		return fmt.Errorf("wrong object type %T for new object", newObj)
	}

	if oldObj != nil {
		oldSeed, ok := oldObj.(*core.Seed)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}
		return s.validateUpdate(oldSeed, newSeed).ToAggregate()
	}

	return s.validateCreate(newSeed).ToAggregate()
}

// validateCreate validates the Seed object upon creation.
// It checks if immutable settings are provided and validates them to ensure they meet the required criteria.
func (s *seedValidator) validateCreate(seed *core.Seed) field.ErrorList {
	allErrs := field.ErrorList{}

	if seed.Spec.Backup != nil {
		backupPath := field.NewPath("spec", "backup")
		allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketCredentialsRef(seed.Spec.Backup.CredentialsRef, backupPath.Child("credentialsRef"))...)

		if seed.Spec.Backup.ProviderConfig != nil {
			providerConfigPath := backupPath.Child("providerConfig")
			backupBucketConfig, err := admission.DecodeBackupBucketConfig(s.decoder, seed.Spec.Backup.ProviderConfig)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(providerConfigPath, seed.Spec.Backup.ProviderConfig, fmt.Errorf("failed to decode provider config: %w", err).Error()))
			} else {
				allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketConfig(backupBucketConfig, providerConfigPath)...)
			}
		}
	}

	return allErrs
}

// validateUpdate validates updates to the Seed resource, ensuring that immutability settings for backup buckets
// are correctly managed. It enforces constraints such as preventing the unlocking of retention policies,
// disabling immutability once locked, and reduction of retention periods when policies are locked.
func (s *seedValidator) validateUpdate(oldSeed, newSeed *core.Seed) field.ErrorList {
	if oldSeed.Spec.Backup == nil || oldSeed.Spec.Backup.ProviderConfig == nil {
		return s.validateCreate(newSeed)
	}

	var (
		allErrs            = field.ErrorList{}
		backupPath         = field.NewPath("spec", "backup")
		providerConfigPath = backupPath.Child("providerConfig")
	)

	oldBackupBucketConfig, err := admission.DecodeBackupBucketConfig(s.lenientDecoder, oldSeed.Spec.Backup.ProviderConfig)
	if err != nil {
		return append(allErrs, field.InternalError(providerConfigPath, fmt.Errorf("failed to decode old provider config: %w", err)))
	}

	newBackupBucketConfig, err := admission.DecodeBackupBucketConfig(s.decoder, newSeed.Spec.Backup.ProviderConfig)
	if err != nil {
		return append(allErrs, field.InternalError(providerConfigPath, fmt.Errorf("failed to decode new provider config: %w", err)))
	}

	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketConfig(newBackupBucketConfig, providerConfigPath)...)
	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketConfigUpdate(oldBackupBucketConfig, newBackupBucketConfig, providerConfigPath)...)
	allErrs = append(allErrs, gcpvalidation.ValidateBackupBucketCredentialsRef(newSeed.Spec.Backup.CredentialsRef, backupPath.Child("credentialsRef"))...)

	return allErrs
}
