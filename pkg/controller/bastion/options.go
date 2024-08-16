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
	"slices"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"

	gcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
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
	ImagePath           string
	MachineName         string
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

	bastionVmDetails, err := DetermineVmDetails(cluster.CloudProfile.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to determine VM details for bastion host: %w", err)
	}

	cloudProfileConfig, err := getCloudProfileConfig(cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to extract cloud provider config from cluster: %w", err)
	}
	image, err := getProviderSpecificImage(cloudProfileConfig.MachineImages, bastionVmDetails)
	if err != nil {
		return nil, fmt.Errorf("failed to extract image from provider config: %w", err)
	}

	return &Options{
		Shoot:               cluster.Shoot,
		BastionInstanceName: baseResourceName,
		Zone:                getZone(cluster, region, providerStatus),
		DiskName:            DiskResourceName(baseResourceName),
		Subnetwork:          fmt.Sprintf("regions/%s/subnetworks/%s", region, subnetWork),
		ProjectID:           projectID,
		Network:             fmt.Sprintf("projects/%s/global/networks/%s", projectID, vNetworkName),
		WorkersCIDR:         workersCidr,
		MachineName:         bastionVmDetails.MachineName,
		ImagePath:           image.Image,
	}, nil
}

func getZone(cluster *extensions.Cluster, region string, providerStatus *providerStatusRaw) string {
	if providerStatus != nil {
		return providerStatus.Zone
	}

	for _, j := range cluster.CloudProfile.Spec.Regions {
		if j.Name == region && len(j.Zones) > 0 {
			return j.Zones[0].Name
		}
	}
	return ""
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

func getCloudProfileConfig(cluster *extensions.Cluster) (*gcpv1alpha1.CloudProfileConfig, error) {
	if cluster.CloudProfile.Spec.ProviderConfig.Raw == nil {
		return nil, fmt.Errorf("no cloud provider config set in cluster's CloudProfile")
	}

	var (
		cloudProfileConfig = &gcpv1alpha1.CloudProfileConfig{}
		decoder            = kubernetes.GardenCodec.UniversalDeserializer()
	)

	if _, _, err := decoder.Decode(cluster.CloudProfile.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
		return nil, err
	}

	return cloudProfileConfig, nil
}

// getProviderSpecificImage returns the provider specific MachineImageVersion that matches with the given VmDetails
func getProviderSpecificImage(images []gcpv1alpha1.MachineImages, vm VmDetails) (gcpv1alpha1.MachineImageVersion, error) {
	imageIndex := slices.IndexFunc(images, func(image gcpv1alpha1.MachineImages) bool {
		return image.Name == vm.ImageBaseName
	})

	if imageIndex == -1 {
		return gcpv1alpha1.MachineImageVersion{},
			fmt.Errorf("machine image with name %s not found in cloudProfileConfig", vm.ImageBaseName)
	}

	versions := images[imageIndex].Versions
	versionIndex := slices.IndexFunc(versions, func(version gcpv1alpha1.MachineImageVersion) bool {
		return version.Version == vm.ImageVersion && version.Architecture != nil && *version.Architecture == vm.Architecture
	})

	if versionIndex == -1 {
		return gcpv1alpha1.MachineImageVersion{},
			fmt.Errorf("version %s for arch %s of image %s not found in cloudProfileConfig",
				vm.ImageVersion, vm.Architecture, vm.ImageBaseName)
	}

	return versions[versionIndex], nil
}
