// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/gardener/gardener/extensions/pkg/terraformer"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal/imagevector"
)

const (
	// TerraformVarServiceAccount is the name of the terraform service account environment variable.
	TerraformVarServiceAccount = "TF_VAR_SERVICEACCOUNT"
)

// TerraformerVariablesEnvironmentFromServiceAccount computes the Terraformer variables environment from the
// given ServiceAccount.
func TerraformerVariablesEnvironmentFromServiceAccount(account *gcp.ServiceAccount) (map[string]string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, account.Raw); err != nil {
		return nil, err
	}

	return map[string]string{
		TerraformVarServiceAccount: buf.String(),
	}, nil
}

// NewTerraformer initializes a new Terraformer.
func NewTerraformer(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
) (terraformer.Terraformer, error) {
	tf, err := terraformer.NewForConfig(logger, restConfig, purpose, infra.Namespace, infra.Name, imagevector.TerraformerImage())
	if err != nil {
		return nil, err
	}

	return tf.
		UseV2(true).
		SetLogLevel("debug").
		SetTerminationGracePeriodSeconds(630).
		SetDeadlineCleaning(5 * time.Minute).
		SetDeadlinePod(15 * time.Minute), nil
}

// NewTerraformerWithAuth initializes a new Terraformer that has the ServiceAccount credentials.
func NewTerraformerWithAuth(
	logger logr.Logger,
	restConfig *rest.Config,
	purpose string,
	infra *extensionsv1alpha1.Infrastructure,
) (terraformer.Terraformer, error) {
	tf, err := NewTerraformer(logger, restConfig, purpose, infra)
	if err != nil {
		return nil, err
	}

	return SetTerraformerEnvVars(tf, infra.Spec.SecretRef)
}

// SetTerraformerEnvVars sets the environment variables based on the given secret reference.
func SetTerraformerEnvVars(tf terraformer.Terraformer, secretRef corev1.SecretReference) (terraformer.Terraformer, error) {
	return tf.SetEnvVars(corev1.EnvVar{
		Name: TerraformVarServiceAccount,
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: secretRef.Name,
			},
			Key: gcp.ServiceAccountJSONField,
		}},
	}), nil
}
