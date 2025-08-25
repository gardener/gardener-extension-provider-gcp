// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cloudprovider

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{logger: logger}
}

type ensurer struct {
	logger logr.Logger
}

// EnsureCloudProviderSecret ensures that cloudprovider secret
// contains the proper credential source.
func (e *ensurer) EnsureCloudProviderSecret(_ context.Context, _ gcontext.GardenContext, newSecret, _ *corev1.Secret) error {
	if newSecret.Labels == nil || newSecret.Labels[securityv1alpha1constants.LabelWorkloadIdentityProvider] != "gcp" {
		return nil
	}

	if err := gcp.SetWorkloadIdentityFeatures(newSecret.Data, gcp.WorkloadIdentityMountPath); err != nil {
		return fmt.Errorf("failed to set WorkloadIdentity features in the cloudprovider secret: %w", err)
	}

	return nil
}
