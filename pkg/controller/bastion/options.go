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

package bastion

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// Options contains provider-related information required for setting up
// a bastion instance. This struct combines precomputed values like the
// bastion instance name with the IDs of pre-existing cloud provider
// resources, like the VPC ID, subnet ID etc.
type Options struct {
	Shoot               *gardencorev1beta1.Shoot
	BastionInstanceName string
	FirewallName        string
	PrivateIP           string
	PublicIP            string
	DiskName            string
	UserData            string
	Zone                string
	Region              string
	Subnetwork          string
	ProjectID           string
}

// DetermineOptions determines the required information that are required to reconcile a Bastion on GCP. This
// function does not create any IaaS resources.
func DetermineOptions(ctx context.Context, bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster) (*Options, error) {
	name := cluster.ObjectMeta.Name
	bastionInstanceName := fmt.Sprintf("%s-%s-bastion", name, bastion.Name)
	firewallName := fmt.Sprintf("%s-allow-ssh-access", bastionInstanceName)
	diskName := fmt.Sprintf("%s-%s-disk", name, bastion.Name)
	publicIP := bastion.Spec.Ingress[0].IPBlock.CIDR
	userData := string(bastion.Spec.UserData)
	subnetwork := cluster.Shoot.Name + "-nodes"
	zone := strings.Join(cluster.Shoot.Spec.Provider.Workers[0].Zones, " ")

	secret := &corev1.Secret{}
	serviceAccountJSON, ok := secret.Data[gcp.ServiceAccountJSONField]
	if !ok {
		return nil, fmt.Errorf("missing %q field in secret", gcp.ServiceAccountJSONField)
	}

	projectID, err := gcp.ExtractServiceAccountProjectID(serviceAccountJSON)
	if err != nil {
		return nil, err
	}

	return &Options{
		Shoot:               cluster.Shoot,
		BastionInstanceName: bastionInstanceName,
		FirewallName:        firewallName,
		Zone:                zone,
		DiskName:            diskName,
		PublicIP:            publicIP,
		UserData:            userData,
		Subnetwork:          subnetwork,
		ProjectID:           projectID,
	}, nil
}
