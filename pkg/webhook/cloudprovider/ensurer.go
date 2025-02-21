// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var (
	usedCredentialsConfigFields = map[string]struct{}{
		"universe_domain":                   {},
		"type":                              {},
		"audience":                          {},
		"subject_token_type":                {},
		"token_url":                         {},
		"service_account_impersonation_url": {},
	}
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(mgr manager.Manager, logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		logger:  logger,
		decoder: serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
	}
}

type ensurer struct {
	logger  logr.Logger
	decoder runtime.Decoder
}

// EnsureCloudProviderSecret ensures that cloudprovider secret
// contains the proper credential source.
func (e *ensurer) EnsureCloudProviderSecret(_ context.Context, _ gcontext.GardenContext, newSecret, _ *corev1.Secret) error {
	if newSecret.ObjectMeta.Labels == nil || newSecret.ObjectMeta.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider] != "gcp" {
		return nil
	}

	if _, ok := newSecret.Data[securityv1alpha1constants.DataKeyConfig]; !ok {
		return errors.New("cloudprovider secret is missing a 'config' data key")
	}
	workloadIdentityConfig := &apisgcp.WorkloadIdentityConfig{}
	if err := util.Decode(e.decoder, newSecret.Data[securityv1alpha1constants.DataKeyConfig], workloadIdentityConfig); err != nil {
		return fmt.Errorf("could not decode 'config' as WorkloadIdentityConfig: %w", err)
	}

	config := map[string]any{}
	if err := json.Unmarshal(workloadIdentityConfig.CredentialsConfig.Raw, &config); err != nil {
		return fmt.Errorf("could not unmarshal credential config: %w", err)
	}

	maps.DeleteFunc(config, func(k string, _ any) bool {
		_, contain := usedCredentialsConfigFields[k]
		return !contain
	})

	config["credential_source"] = map[string]any{
		"file": gcp.WorkloadIdentityMountPath + "/token",
		"format": map[string]string{
			"type": "text",
		},
	}
	newConfig, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("could not marshal new config: %w", err)
	}
	newSecret.Data["credentialsConfig"] = newConfig
	newSecret.Data["projectID"] = []byte(workloadIdentityConfig.ProjectID)

	return nil
}
