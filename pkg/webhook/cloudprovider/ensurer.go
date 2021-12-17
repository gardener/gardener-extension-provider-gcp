// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cloudprovider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/utils"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	gcontext "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ensurer struct {
	logger logr.Logger
	client client.Client
}

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		logger: logger,
	}
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// InjectScheme injects the given scheme into the decoder of the ensurer.
func (e *ensurer) InjectScheme(_ *runtime.Scheme) error {
	return nil
}

// EnsureCloudProviderSecret ensures that cloudprovider secret contain a serviceaccount.json (if not present).
func (e *ensurer) EnsureCloudProviderSecret(ctx context.Context, _ gcontext.GardenContext, new, _ *corev1.Secret) error {
	if utils.HasSecretKey(new, gcp.ServiceAccountJSONField) {
		return nil
	}

	if !utils.HasSecretKey(new, gcp.ServiceAccountSecretFieldProjectID) || !utils.HasSecretKey(new, gcp.ServiceAccountSecretFieldOrganisationID) {
		return fmt.Errorf("could not assign a service account as either project id or org id is missing")
	}

	serviceAccountSecret, err := e.getManagedServiceAccountSecret(ctx, string(new.Data[gcp.ServiceAccountSecretFieldOrganisationID]))
	if err != nil {
		return err
	}

	serviceAccountData, err := generateServiceAccountData(serviceAccountSecret.Data[gcp.ServiceAccountJSONField], string(new.Data[gcp.ServiceAccountSecretFieldProjectID]))
	if err != nil {
		return err
	}
	new.Data[gcp.ServiceAccountJSONField] = serviceAccountData

	return nil
}

func (e *ensurer) getManagedServiceAccountSecret(ctx context.Context, orgID string) (*corev1.Secret, error) {
	var (
		serviceAccountSecretList = corev1.SecretList{}
		matchingSecrets          = []*corev1.Secret{}
		labelSelector            = client.MatchingLabels{gcp.ExtensionPurposeLabel: gcp.ExtensionPurposeServiceAccoutSecret}
	)

	if err := e.client.List(ctx, &serviceAccountSecretList, labelSelector); err != nil {
		return nil, err
	}

	for _, sec := range serviceAccountSecretList.Items {
		if !utils.HasSecretKey(&sec, gcp.ServiceAccountSecretFieldOrganisationID) {
			continue
		}

		if string(sec.Data[gcp.ServiceAccountSecretFieldOrganisationID]) == orgID {
			tmp := &sec
			matchingSecrets = append(matchingSecrets, tmp)
		}
	}

	if len(matchingSecrets) == 0 {
		return nil, fmt.Errorf("found no service account secret matching to org id %q", orgID)
	}

	if len(matchingSecrets) > 1 {
		return nil, fmt.Errorf("found more than one service account secret matching to org id %q", orgID)
	}

	if !utils.HasSecretKey(matchingSecrets[0], gcp.ServiceAccountJSONField) {
		return nil, fmt.Errorf("service account secret does not contain service account information")
	}

	return matchingSecrets[0], nil
}

func generateServiceAccountData(serviceAccountTemplate []byte, projectID string) ([]byte, error) {
	var servieAccountData map[string]string
	if err := json.Unmarshal(serviceAccountTemplate, &servieAccountData); err != nil {
		return nil, err
	}

	servieAccountData["project_id"] = projectID

	servieAccountDataRaw, err := json.Marshal(servieAccountData)
	if err != nil {
		return nil, err
	}
	return servieAccountDataRaw, nil
}
