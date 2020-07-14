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

package gcp

import (
	"context"
	"encoding/json"
	"fmt"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceAccount represents a GCP service account.
type ServiceAccount struct {
	// Raw is the raw representation of the GCP service account.
	Raw []byte
	// ProjectID is the project id the service account is associated to.
	ProjectID string
}

// GetServiceAccount retrieves the ServiceAccount from the secret with the given secret reference.
func GetServiceAccount(ctx context.Context, c client.Client, secretRef corev1.SecretReference) (*ServiceAccount, error) {
	data, err := GetServiceAccountData(ctx, c, secretRef)
	if err != nil {
		return nil, err
	}

	projectID, err := ExtractServiceAccountProjectID(data)
	if err != nil {
		return nil, err
	}

	return &ServiceAccount{
		Raw:       data,
		ProjectID: projectID,
	}, nil
}

// GetServiceAccountData retrieves the service account specified by the secret reference.
func GetServiceAccountData(ctx context.Context, c client.Client, secretRef corev1.SecretReference) ([]byte, error) {
	secret, err := extensionscontroller.GetSecretByReference(ctx, c, &secretRef)
	if err != nil {
		return nil, err
	}

	return ReadServiceAccountSecret(secret)
}

// ReadServiceAccountSecret reads the ServiceAccount from the given secret.
func ReadServiceAccountSecret(secret *corev1.Secret) ([]byte, error) {
	data, ok := secret.Data[ServiceAccountJSONField]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s doesn't have a service account json", secret.Namespace, secret.Name)
	}

	return data, nil
}

// ExtractServiceAccountProjectID extracts the project id from the given service account JSON.
func ExtractServiceAccountProjectID(serviceAccountJSON []byte) (string, error) {
	var serviceAccount struct {
		ProjectID string `json:"project_id"`
	}

	if err := json.Unmarshal(serviceAccountJSON, &serviceAccount); err != nil {
		return "", err
	}
	if serviceAccount.ProjectID == "" {
		return "", fmt.Errorf("no service account specified")
	}

	return serviceAccount.ProjectID, nil
}
