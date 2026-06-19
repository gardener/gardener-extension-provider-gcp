// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	gcpvalidation "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

type credentialsBinding struct {
	apiReader                                    client.Reader
	allowedTokenURLs                             []string
	allowedServiceAccountImpersonationURLRegExps []*regexp.Regexp
}

// NewCredentialsBindingValidator returns a new instance of a credentials binding validator.
func NewCredentialsBindingValidator(mgr manager.Manager, allowedTokenURLs []string, allowedServiceAccountImpersonationURLRegExps []*regexp.Regexp) extensionswebhook.Validator {
	return &credentialsBinding{
		apiReader:        mgr.GetAPIReader(),
		allowedTokenURLs: allowedTokenURLs,
		allowedServiceAccountImpersonationURLRegExps: allowedServiceAccountImpersonationURLRegExps,
	}
}

// Validate checks whether the given CredentialsBinding refers to valid GCP credentials.
func (cb *credentialsBinding) Validate(ctx context.Context, newObj, oldObj client.Object) error {
	credentialsBinding, ok := newObj.(*security.CredentialsBinding)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if oldObj != nil {
		_, ok := oldObj.(*security.CredentialsBinding)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}

		// The relevant fields of the credentials binding are immutable so we can exit early on update
		return nil
	}

	// Explicitly use the client.Reader to prevent controller-runtime to start Informer for Secrets/InternalSecrets/WorkloadIdentities
	// under the hood. The latter increases the memory usage of the component.
	credentials, err := kubernetes.GetCredentialsByObjectReference(ctx, cb.apiReader, credentialsBinding.CredentialsRef)
	if err != nil {
		return err
	}

	credentialsKey := client.ObjectKey{Namespace: credentialsBinding.CredentialsRef.Namespace, Name: credentialsBinding.CredentialsRef.Name}
	switch creds := credentials.(type) {
	case *corev1.Secret:
		return gcpvalidation.ValidateCloudProviderSecretData(creds.Data, field.NewPath("secret"), credentialsKey.String()).ToAggregate()
	case *gardencorev1beta1.InternalSecret:
		return gcpvalidation.ValidateCloudProviderSecretData(creds.Data, field.NewPath("secret"), credentialsKey.String()).ToAggregate()
	case *securityv1alpha1.WorkloadIdentity:
		if creds.Spec.TargetSystem.ProviderConfig == nil {
			return errors.New("the target system is missing configuration")
		}

		config, err := helper.WorkloadIdentityConfigFromRaw(creds.Spec.TargetSystem.ProviderConfig)
		if err != nil {
			return fmt.Errorf("target system's configuration is not valid: %w", err)
		}

		fieldPath := field.NewPath("spec", "targetSystem", "providerConfig")
		if errList := gcpvalidation.ValidateWorkloadIdentityConfig(config, fieldPath, cb.allowedTokenURLs, cb.allowedServiceAccountImpersonationURLRegExps); len(errList) > 0 {
			return fmt.Errorf("referenced workload identity %s is not valid: %w", credentialsKey, errList.ToAggregate())
		}
		return nil
	default:
		return fmt.Errorf("unsupported credentials reference: version %q, kind %q", credentialsBinding.CredentialsRef.APIVersion, credentialsBinding.CredentialsRef.Kind)
	}
}
