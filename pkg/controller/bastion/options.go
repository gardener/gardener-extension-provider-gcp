// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"

	gcpapi "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// Maximum length for "base" name due to fact that we use this name to name other GCP resources,
// and it's required to fit 63 character length https://cloud.google.com/compute/docs/naming-resources
const maxLengthForBaseName = 33
const maxLengthForResource = 63

// Options contains provider-related information required for setting up
// a bastion instance. This struct combines precomputed values like the
// bastion instance name with the IDs of pre-existing cloud provider
// resources, like the Firewall name, subnet name etc.
type Options struct {
	Shoot               *gardencorev1beta1.Shoot
	BastionInstanceName string
	DiskName            string
	Zone                string
	Subnetwork          string
	ProjectID           string
	Network             string
	WorkersCIDR         string
}

type providerStatusRaw struct {
	Zone string `json:"zone"`
}

// DetermineOptions determines the required information that are required to reconcile a Bastion on GCP. This
// function does not create any IaaS resources.
func DetermineOptions(bastion *extensionsv1alpha1.Bastion, cluster *controller.Cluster, projectID, vNetworkName, subnetWork string) (*Options, error) {
	providerStatus, err := getProviderStatus(bastion)
	if err != nil {
		return nil, err
	}

	// Each resource name up to a maximum of 63 characters in GCP
	// https://cloud.google.com/compute/docs/naming-resources
	clusterName := cluster.ObjectMeta.Name
	baseResourceName, err := generateBastionBaseResourceName(clusterName, bastion.Name)
	if err != nil {
		return nil, err
	}

	workersCidr, err := getWorkersCIDR(cluster)
	if err != nil {
		return nil, err
	}

	region := cluster.Shoot.Spec.Region
	return &Options{
		Shoot:               cluster.Shoot,
		BastionInstanceName: baseResourceName,
		Zone:                getZone(cluster, region, providerStatus),
		DiskName:            DiskResourceName(baseResourceName),
		Subnetwork:          fmt.Sprintf("regions/%s/subnetworks/%s", region, subnetWork),
		ProjectID:           projectID,
		Network:             fmt.Sprintf("projects/%s/global/networks/%s", projectID, vNetworkName),
		WorkersCIDR:         workersCidr,
	}, nil
}

func getZone(cluster *extensions.Cluster, region string, providerStatus *providerStatusRaw) string {
	if providerStatus != nil {
		return providerStatus.Zone
	}

	for _, j := range cluster.CloudProfile.Spec.Regions {
		if j.Name == region {
			if len(j.Zones) > 0 {
				return j.Zones[0].Name
			}
		}
	}
	return ""
}

func getNetworkName(cluster *extensions.Cluster, projectID string, clusterName string) (string, error) {
	var networkName string
	infrastructureConfig := &gcpapi.InfrastructureConfig{}
	err := json.Unmarshal(cluster.Shoot.Spec.Provider.InfrastructureConfig.Raw, infrastructureConfig)
	if err != nil {
		return "", err
	}

	if infrastructureConfig.Networks.VPC != nil {
		networkName = fmt.Sprintf("projects/%s/global/networks/%s", projectID, infrastructureConfig.Networks.VPC.Name)
	} else {
		networkName = fmt.Sprintf("projects/%s/global/networks/%s", projectID, clusterName)
	}

	return networkName, nil
}

func ingressPermissions(bastion *extensionsv1alpha1.Bastion) ([]string, error) {
	var cidrs []string
	for _, ingress := range bastion.Spec.Ingress {
		cidr := ingress.IPBlock.CIDR
		ip, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid ingress CIDR %q: %w", cidr, err)
		}

		normalisedCIDR := ipNet.String()

		if ip.To4() != nil {
			cidrs = append(cidrs, normalisedCIDR)
		} else if ip.To16() != nil {
			// Only IPv4 is supported in sourceRanges[].
			// https://cloud.google.com/compute/docs/reference/rest/v1/firewalls/insert
			return nil, errors.New("IPv6 is currently not fully supported")
		}

	}

	return cidrs, nil
}

func generateBastionBaseResourceName(clusterName string, bastionName string) (string, error) {
	if clusterName == "" {
		return "", fmt.Errorf("clusterName can't be empty")
	}
	if bastionName == "" {
		return "", fmt.Errorf("bastionName can't be empty")
	}

	staticName := clusterName + "-" + bastionName
	h := sha256.New()
	_, err := h.Write([]byte(staticName))
	if err != nil {
		return "", err
	}
	hash := fmt.Sprintf("%x", h.Sum(nil))
	if len([]rune(staticName)) > maxLengthForBaseName {
		staticName = staticName[:maxLengthForBaseName]
	}
	return fmt.Sprintf("%s-bastion-%s", staticName, hash[:5]), nil
}

func getProviderStatus(bastion *extensionsv1alpha1.Bastion) (*providerStatusRaw, error) {
	if bastion.Status.ProviderStatus != nil && bastion.Status.ProviderStatus.Raw != nil {
		return unmarshalProviderStatus(bastion.Status.ProviderStatus.Raw)
	}
	return nil, nil
}

func marshalProviderStatus(zone string) ([]byte, error) {
	return json.Marshal(&providerStatusRaw{
		Zone: zone,
	})
}

func unmarshalProviderStatus(bytes []byte) (*providerStatusRaw, error) {
	info := &providerStatusRaw{}

	err := json.Unmarshal(bytes, info)
	if err != nil {
		return nil, fmt.Errorf("failed to parse json for status.ProviderStatus")
	}
	return info, nil
}

// DiskResourceName is Disk resource name
func DiskResourceName(baseName string) string {
	return fmt.Sprintf("%s-disk", baseName)
}

// FirewallIngressAllowSSHResourceName is Firewall ingress allow SSH rule resource name
func FirewallIngressAllowSSHResourceName(baseName string) string {
	return fmt.Sprintf("%s-allow-ssh", baseName)
}

// FirewallEgressAllowOnlyResourceName is Firewall egress allow only worker node rule resource name
func FirewallEgressAllowOnlyResourceName(baseName string) string {
	return fmt.Sprintf("%s-egress-worker", baseName)
}

// FirewallEgressDenyAllResourceName is Firewall egress deny all rule resource name
func FirewallEgressDenyAllResourceName(baseName string) string {
	return fmt.Sprintf("%s-deny-all", baseName)
}
