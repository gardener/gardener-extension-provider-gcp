// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"regexp"
	"slices"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/security"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var (
	// DefaultAddOptions are the default AddOptions for configuring the validator.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the GCP admission webhook to the manager.
type AddOptions struct {
	// WorkloadIdentity is the workload identity validation configuration.
	WorkloadIdentity config.WorkloadIdentity
}

const (
	// Name is a name for a validation webhook.
	Name = "validator"
	// SecretsValidatorName is the name of the secrets validator.
	SecretsValidatorName = "secrets." + Name
)

var logger = log.Log.WithName("gcp-validator-webhook")

// New creates a new validation webhook for `core.gardener.cloud` and `security.gardener.cloud` resources.
func New(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Setting up webhook", "name", Name)

	var (
		allowedTokenURLs                             = slices.Clone(DefaultAddOptions.WorkloadIdentity.AllowedTokenURLs)
		allowedServiceAccountImpersonationURLRegExps = make([]*regexp.Regexp, 0, len(DefaultAddOptions.WorkloadIdentity.AllowedServiceAccountImpersonationURLRegExps))
	)

	for _, regExp := range DefaultAddOptions.WorkloadIdentity.AllowedServiceAccountImpersonationURLRegExps {
		compiled, err := regexp.Compile(regExp)
		if err != nil {
			return nil, err
		}
		allowedServiceAccountImpersonationURLRegExps = append(allowedServiceAccountImpersonationURLRegExps, compiled)
	}

	logger.Info("Initializing workload identity validator config", "allowed_token_urls", allowedTokenURLs, "allowed_service_account_impersonation_url_regexps", allowedServiceAccountImpersonationURLRegExps)

	return extensionswebhook.New(mgr, extensionswebhook.Args{
		Provider: gcp.Type,
		Name:     Name,
		Path:     "/webhooks/validate",
		Validators: map[extensionswebhook.Validator][]extensionswebhook.Type{
			NewShootValidator(mgr):                  {{Obj: &core.Shoot{}}},
			NewCloudProfileValidator(mgr):           {{Obj: &core.CloudProfile{}}},
			NewNamespacedCloudProfileValidator(mgr): {{Obj: &core.NamespacedCloudProfile{}}},
			NewSecretBindingValidator(mgr):          {{Obj: &core.SecretBinding{}}},
			NewCredentialsBindingValidator(
				mgr,
				allowedTokenURLs,
				allowedServiceAccountImpersonationURLRegExps,
			): {{Obj: &security.CredentialsBinding{}}},
			NewSeedValidator(mgr): {{Obj: &core.Seed{}}},
			NewWorkloadIdentityValidator(
				allowedTokenURLs,
				allowedServiceAccountImpersonationURLRegExps,
			): {{Obj: &securityv1alpha1.WorkloadIdentity{}}},
			NewBackupBucketValidator(mgr): {{Obj: &core.BackupBucket{}}},
		},
		Target: extensionswebhook.TargetSeed,
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"provider.extensions.gardener.cloud/gcp": "true"},
		},
	})
}

// NewSecretsWebhook creates a new validation webhook for Secrets.
func NewSecretsWebhook(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	logger.Info("Setting up webhook", "name", SecretsValidatorName)

	return extensionswebhook.New(mgr, extensionswebhook.Args{
		Provider: gcp.Type,
		Name:     SecretsValidatorName,
		Path:     "/webhooks/validate/secrets",
		Validators: map[extensionswebhook.Validator][]extensionswebhook.Type{
			NewSecretValidator(): {{Obj: &corev1.Secret{}}},
		},
		Target: extensionswebhook.TargetSeed,
		ObjectSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"provider.shoot.gardener.cloud/gcp": "true"},
		},
	})
}
