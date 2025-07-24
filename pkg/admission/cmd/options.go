// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	webhookcmd "github.com/gardener/gardener/extensions/pkg/webhook/cmd"
	"github.com/spf13/pflag"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/mutator"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config"
)

// GardenWebhookSwitchOptions are the webhookcmd.SwitchOptions for the admission webhooks.
func GardenWebhookSwitchOptions() *webhookcmd.SwitchOptions {
	return webhookcmd.NewSwitchOptions(
		webhookcmd.Switch(validator.Name, validator.New),
		webhookcmd.Switch(validator.SecretsValidatorName, validator.NewSecretsWebhook),
		webhookcmd.Switch(mutator.Name, mutator.New),
	)
}

// ConfigOptions are command line options that can be set for admission webhooks.
type ConfigOptions struct {
	WorkloadIdentityOptions WorkloadIdentityOptions

	config *Config
}

// Config is a completed admission configuration.
type Config struct {
	WorkloadIdentity config.WorkloadIdentity
}

// WorkloadIdentityOptions are options that specify how workload identities should be validated.
type WorkloadIdentityOptions struct {
	// AllowedTokenURLs are the allowed token URLs.
	AllowedTokenURLs []string
	// AllowedServiceAccountImpersonationURLRegExps are the allowed service account impersonation URL regular expressions.
	AllowedServiceAccountImpersonationURLRegExps []string
}

// AddFlags implements Flagger.AddFlags.
func (w *WorkloadIdentityOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(
		&w.AllowedTokenURLs,
		"wi-allowed-token-url",
		[]string{"https://sts.googleapis.com/v1/token"},
		"Token URL that is allowed to be configured in workload identity configurations. Can be set multiple times.",
	)
	fs.StringSliceVar(
		&w.AllowedServiceAccountImpersonationURLRegExps,
		"wi-allowed-service-account-impersonation-url-regexp",
		[]string{`^https://iamcredentials\.googleapis\.com/v1/projects/-/serviceAccounts/.+:generateAccessToken$`},
		"Regular expression that is used to validate service account impersonation urls configured in workload identity configurations. Can be set multiple times.",
	)
}

// Complete implements RESTCompleter.Complete.
func (c *ConfigOptions) Complete() error {
	c.config = &Config{
		WorkloadIdentity: config.WorkloadIdentity{
			AllowedTokenURLs: c.WorkloadIdentityOptions.AllowedTokenURLs,
			AllowedServiceAccountImpersonationURLRegExps: c.WorkloadIdentityOptions.AllowedServiceAccountImpersonationURLRegExps,
		},
	}
	return nil
}

// Completed returns the completed Config. Only call this if `Complete` was successful.
func (c *ConfigOptions) Completed() *Config {
	return c.config
}

// AddFlags implements Flagger.AddFlags.
func (c *ConfigOptions) AddFlags(fs *pflag.FlagSet) {
	c.WorkloadIdentityOptions.AddFlags(fs)
}

// ApplyWorkloadIdentity sets the values of this Config in the given config.WorkloadIdentity.
func (c *Config) ApplyWorkloadIdentity(cfg *config.WorkloadIdentity) {
	*cfg = c.WorkloadIdentity
}
