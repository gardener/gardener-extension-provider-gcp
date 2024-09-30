// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"time"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener-extension-provider-gcp/imagevector"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

const (
	// TerraformVarServiceAccount is the name of the terraform service account environment variable.
	TerraformVarServiceAccount = "TF_VAR_SERVICEACCOUNT"
)

// NewTerraformer initializes a new Terraformer.
func NewTerraformer(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
	disableProjectedTokenMount bool,
) (
	terraformer.Terraformer,
	error,
) {
	tf, err := terraformer.NewForConfig(logger, restConfig, purpose, infra.Namespace, infra.Name, imagevector.TerraformerImage())
	if err != nil {
		return nil, err
	}

	owner := metav1.NewControllerRef(infra, extensionsv1alpha1.SchemeGroupVersion.WithKind(extensionsv1alpha1.InfrastructureResource))
	return tf.
		UseProjectedTokenMount(!disableProjectedTokenMount).
		SetTerminationGracePeriodSeconds(630).
		SetDeadlineCleaning(5 * time.Minute).
		SetDeadlinePod(15 * time.Minute).
		SetOwnerRef(owner), nil
}

// NewTerraformerWithAuth initializes a new Terraformer that has the ServiceAccount credentials.
func NewTerraformerWithAuth(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
	disableProjectedTokenMount bool,
	useWorkloadIdentityToken bool,
) (
	terraformer.Terraformer,
	error,
) {
	tf, err := NewTerraformer(logger, restConfig, purpose, infra, disableProjectedTokenMount)
	if err != nil {
		return nil, err
	}

	secretKey := gcp.ServiceAccountJSONField
	if useWorkloadIdentityToken {
		secretKey = gcp.CredentialsConfigField
	}
	return SetTerraformerEnvVars(tf, infra.Spec.SecretRef, secretKey)
}

// SetTerraformerEnvVars sets the environment variables based on the given secret reference.
func SetTerraformerEnvVars(tf terraformer.Terraformer, secretRef corev1.SecretReference, secretKey string) (terraformer.Terraformer, error) {
	return tf.SetEnvVars(corev1.EnvVar{
		Name: TerraformVarServiceAccount,
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretRef.Name,
			},
			Key: secretKey,
		}},
	}), nil
}
