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

package controlplane

import (
	"bytes"
	"context"
	"fmt"
	"regexp"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/internal"

	"github.com/coreos/go-systemd/v22/unit"
	"github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/version"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const csiMigrationVersion = "1.18"

// NewEnsurer creates a new controlplane ensurer.
func NewEnsurer(logger logr.Logger) genericmutator.Ensurer {
	return &ensurer{
		logger: logger.WithName("gcp-controlplane-ensurer"),
	}
}

type ensurer struct {
	genericmutator.NoopEnsurer
	client client.Client
	logger logr.Logger
}

// InjectClient injects the given client into the ensurer.
func (e *ensurer) InjectClient(client client.Client) error {
	e.client = client
	return nil
}

// EnsureKubeAPIServerDeployment ensures that the kube-apiserver deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeAPIServerDeployment(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-apiserver"); c != nil {
		ensureKubeAPIServerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
		ensureEnvVars(c, csiEnabled, csiMigrationComplete)
		ensureKubeAPIServerVolumeMounts(c, csiEnabled, csiMigrationComplete)
	}

	ensureKubeAPIServerVolumes(ps, csiEnabled, csiMigrationComplete)
	return e.ensureChecksumAnnotations(ctx, &new.Spec.Template, new.Namespace, csiEnabled, csiMigrationComplete)
}

// EnsureKubeControllerManagerDeployment ensures that the kube-controller-manager deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeControllerManagerDeployment(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-controller-manager"); c != nil {
		ensureKubeControllerManagerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
		ensureEnvVars(c, csiEnabled, csiMigrationComplete)
		ensureKubeControllerManagerVolumeMounts(c, cluster.Shoot.Spec.Kubernetes.Version, csiEnabled, csiMigrationComplete)
	}

	ensureKubeControllerManagerLabels(template, csiEnabled, csiMigrationComplete)
	ensureKubeControllerManagerVolumes(ps, cluster.Shoot.Spec.Kubernetes.Version, csiEnabled, csiMigrationComplete)
	return e.ensureChecksumAnnotations(ctx, &new.Spec.Template, new.Namespace, csiEnabled, csiMigrationComplete)
}

// EnsureKubeSchedulerDeployment ensures that the kube-scheduler deployment conforms to the provider requirements.
func (e *ensurer) EnsureKubeSchedulerDeployment(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ *appsv1.Deployment) error {
	template := &new.Spec.Template
	ps := &template.Spec

	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, csiMigrationComplete, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return err
	}

	if c := extensionswebhook.ContainerWithName(ps.Containers, "kube-scheduler"); c != nil {
		ensureKubeSchedulerCommandLineArgs(c, csiEnabled, csiMigrationComplete)
	}
	return nil
}

func ensureKubeAPIServerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationGCE=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"CSIMigrationGCEComplete=true", ",")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-provider=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
				"PersistentVolumeLabel", ",")
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
				"PersistentVolumeLabel", ",")
			return
		}
	}

	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "gce")
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-config=",
		"/etc/kubernetes/cloudprovider/cloudprovider.conf")
	c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--enable-admission-plugins=",
		"PersistentVolumeLabel", ",")
	c.Command = extensionswebhook.EnsureNoStringWithPrefixContains(c.Command, "--disable-admission-plugins=",
		"PersistentVolumeLabel", ",")
}

func ensureKubeControllerManagerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-provider=", "external")

	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationGCE=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"CSIMigrationGCEComplete=true", ",")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--cloud-config=")
			c.Command = extensionswebhook.EnsureNoStringWithPrefix(c.Command, "--external-cloud-volume-plugin=")
			return
		}
	}

	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--cloud-config=",
		"/etc/kubernetes/cloudprovider/cloudprovider.conf")
	c.Command = extensionswebhook.EnsureStringWithPrefix(c.Command, "--external-cloud-volume-plugin=", "gce")
}

func ensureKubeSchedulerCommandLineArgs(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled {
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigration=true", ",")
		c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
			"CSIMigrationGCE=true", ",")

		if csiMigrationComplete {
			c.Command = extensionswebhook.EnsureStringWithPrefixContains(c.Command, "--feature-gates=",
				"CSIMigrationGCEComplete=true", ",")
			return
		}
	}
}

func ensureKubeControllerManagerLabels(t *corev1.PodTemplateSpec, csiEnabled, csiMigrationComplete bool) {
	// make sure to always remove this label
	delete(t.Labels, v1beta1constants.LabelNetworkPolicyToBlockedCIDRs)

	if csiEnabled && csiMigrationComplete {
		if t.Labels != nil {
			delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks)
			delete(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks)
		}
		return
	}

	t.Labels = extensionswebhook.EnsureAnnotationOrLabel(t.Labels, v1beta1constants.LabelNetworkPolicyToPublicNetworks, v1beta1constants.LabelNetworkPolicyAllowed)
	t.Labels = extensionswebhook.EnsureAnnotationOrLabel(t.Labels, v1beta1constants.LabelNetworkPolicyToPrivateNetworks, v1beta1constants.LabelNetworkPolicyAllowed)
}

func ensureEnvVars(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		c.Env = extensionswebhook.EnsureNoEnvVarWithName(c.Env, credentialsEnvVar.Name)
		return
	}

	c.Env = extensionswebhook.EnsureEnvVarWithName(c.Env, credentialsEnvVar)
}

var (
	credentialsEnvVar = corev1.EnvVar{
		Name:  "GOOGLE_APPLICATION_CREDENTIALS",
		Value: fmt.Sprintf("/srv/cloudprovider/%s", gcp.ServiceAccountJSONField),
	}
	etcSSLName = "etc-ssl"

	cloudProviderConfigVolumeMount = corev1.VolumeMount{
		Name:      internal.CloudProviderConfigName,
		MountPath: "/etc/kubernetes/cloudprovider",
	}
	cloudProviderSecretVolumeMount = corev1.VolumeMount{
		Name:      v1beta1constants.SecretNameCloudProvider,
		MountPath: "/srv/cloudprovider",
	}
	etcSSLVolumeMount = corev1.VolumeMount{
		Name:      etcSSLName,
		MountPath: "/etc/ssl",
		ReadOnly:  true,
	}

	usrShareCaCerts            = "usr-share-cacerts"
	directoryOrCreate          = corev1.HostPathDirectoryOrCreate
	usrShareCaCertsVolumeMount = corev1.VolumeMount{
		Name:      usrShareCaCerts,
		MountPath: "/usr/share/ca-certificates",
		ReadOnly:  true,
	}
	usrShareCaCertsVolume = corev1.Volume{
		Name: usrShareCaCerts,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/usr/share/ca-certificates",
				Type: &directoryOrCreate,
			},
		},
	}

	cloudProviderConfigVolume = corev1.Volume{
		Name: internal.CloudProviderConfigName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: internal.CloudProviderConfigName},
			},
		},
	}
	cloudProviderSecretVolume = corev1.Volume{
		Name: v1beta1constants.SecretNameCloudProvider,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: v1beta1constants.SecretNameCloudProvider,
			},
		},
	}
	etcSSLVolume = corev1.Volume{
		Name: etcSSLName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/etc/ssl",
				Type: &directoryOrCreate,
			},
		},
	}
)

func ensureKubeAPIServerVolumeMounts(c *corev1.Container, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, cloudProviderSecretVolumeMount.Name)
		return
	}

	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount)
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, cloudProviderSecretVolumeMount)
}

func ensureKubeControllerManagerVolumeMounts(c *corev1.Container, version string, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, cloudProviderSecretVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount.Name)
		c.VolumeMounts = extensionswebhook.EnsureNoVolumeMountWithName(c.VolumeMounts, usrShareCaCertsVolumeMount.Name)
		return
	}

	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, cloudProviderConfigVolumeMount)
	c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, cloudProviderSecretVolumeMount)

	if mustMountEtcSSLFolder(version) {
		c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, etcSSLVolumeMount)
		// some distros have symlinks from /etc/ssl/certs to /usr/share/ca-certificates
		c.VolumeMounts = extensionswebhook.EnsureVolumeMountWithName(c.VolumeMounts, usrShareCaCertsVolumeMount)
	}
}

func ensureKubeAPIServerVolumes(ps *corev1.PodSpec, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, cloudProviderConfigVolume.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, cloudProviderSecretVolume.Name)
		return
	}

	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, cloudProviderConfigVolume)
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, cloudProviderSecretVolume)
}

func ensureKubeControllerManagerVolumes(ps *corev1.PodSpec, version string, csiEnabled, csiMigrationComplete bool) {
	if csiEnabled && csiMigrationComplete {
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, cloudProviderConfigVolume.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, cloudProviderSecretVolume.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, etcSSLVolume.Name)
		ps.Volumes = extensionswebhook.EnsureNoVolumeWithName(ps.Volumes, usrShareCaCertsVolume.Name)
		return
	}

	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, cloudProviderConfigVolume)
	ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, cloudProviderSecretVolume)

	if mustMountEtcSSLFolder(version) {
		ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, etcSSLVolume)
		// some distros have symlinks from /etc/ssl/certs to /usr/share/ca-certificates
		ps.Volumes = extensionswebhook.EnsureVolumeWithName(ps.Volumes, usrShareCaCertsVolume)
	}
}

// Beginning with 1.17 Gardener no longer uses the hyperkube image for the Kubernetes control plane components.
// The hyperkube image contained all the well-known root CAs, but the dedicated images don't. This is why we
// mount the /etc/ssl folder from the host here.
func mustMountEtcSSLFolder(version string) bool {
	k8sVersionAtLeast117, err := versionutils.CompareVersions(version, ">=", "1.17")
	if err != nil {
		return false
	}
	return k8sVersionAtLeast117
}

func (e *ensurer) ensureChecksumAnnotations(ctx context.Context, template *corev1.PodTemplateSpec, namespace string, csiEnabled, csiMigrationComplete bool) error {
	if csiEnabled && csiMigrationComplete {
		if template.Annotations != nil {
			delete(template.Annotations, "checksum/secret-"+v1beta1constants.SecretNameCloudProvider)
			delete(template.Annotations, "checksum/configmap-"+internal.CloudProviderConfigName)
		}
		return nil
	}

	if err := controlplane.EnsureSecretChecksumAnnotation(ctx, template, e.client, namespace, v1beta1constants.SecretNameCloudProvider); err != nil {
		return err
	}
	return controlplane.EnsureConfigMapChecksumAnnotation(ctx, template, e.client, namespace, internal.CloudProviderConfigName)
}

// EnsureKubeletServiceUnitOptions ensures that the kubelet.service unit options conform to the provider requirements.
func (e *ensurer) EnsureKubeletServiceUnitOptions(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ []*unit.UnitOption) ([]*unit.UnitOption, error) {
	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return nil, err
	}

	csiEnabled, _, err := csimigration.CheckCSIConditions(cluster, csiMigrationVersion)
	if err != nil {
		return nil, err
	}

	if opt := extensionswebhook.UnitOptionWithSectionAndName(new, "Service", "ExecStart"); opt != nil {
		command := extensionswebhook.DeserializeCommandLine(opt.Value)
		command = ensureKubeletCommandLineArgs(command, csiEnabled)
		opt.Value = extensionswebhook.SerializeCommandLine(command, 1, " \\\n    ")
	}

	new = extensionswebhook.EnsureUnitOption(new, &unit.UnitOption{
		Section: "Service",
		Name:    "ExecStartPre",
		Value:   `/bin/sh -c 'hostnamectl set-hostname $(wget -q -O- --header "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/hostname | cut -d '.' -f 1)'`,
	})

	return new, nil
}

func ensureKubeletCommandLineArgs(command []string, csiEnabled bool) []string {
	if csiEnabled {
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "external")
		command = extensionswebhook.EnsureStringWithPrefix(command, "--enable-controller-attach-detach=", "true")
	} else {
		command = extensionswebhook.EnsureStringWithPrefix(command, "--cloud-provider=", "gce")
	}
	return command
}

// EnsureKubeletConfiguration ensures that the kubelet configuration conforms to the provider requirements.
func (e *ensurer) EnsureKubeletConfiguration(ctx context.Context, ectx genericmutator.EnsurerContext, new, _ *kubeletconfigv1beta1.KubeletConfiguration) error {
	cluster, err := ectx.GetCluster(ctx)
	if err != nil {
		return err
	}

	csiEnabled, err := version.CompareVersions(cluster.Shoot.Spec.Kubernetes.Version, ">=", csiMigrationVersion)
	if err != nil {
		return err
	}

	if csiEnabled {
		if new.FeatureGates == nil {
			new.FeatureGates = make(map[string]bool)
		}

		new.FeatureGates["CSIMigration"] = true
		new.FeatureGates["CSIMigrationGCE"] = true
		// kubelets of new worker nodes can directly be started with the the `CSIMigrationGCEComplete` feature gate
		new.FeatureGates["CSIMigrationGCEComplete"] = true
	}

	return nil
}

var regexFindProperty = regexp.MustCompile("net.ipv4.ip_forward[[:space:]]*=[[:space:]]*([[:alnum:]]+)")

// EnsureKubernetesGeneralConfiguration ensures that the kubernetes general configuration conforms to the provider requirements.
func (e *ensurer) EnsureKubernetesGeneralConfiguration(ctx context.Context, _ genericmutator.EnsurerContext, new, _ *string) error {
	// If the needed property exists, ensure the correct value
	if regexFindProperty.MatchString(*new) {
		res := regexFindProperty.ReplaceAll([]byte(*new), []byte("net.ipv4.ip_forward = 1"))
		*new = string(res)
		return nil
	}

	// If the property do not exist, append it in the end of the string
	buf := bytes.Buffer{}
	buf.WriteString(*new)
	buf.WriteString("\n")
	buf.WriteString("# GCE specific settings\n")
	buf.WriteString("net.ipv4.ip_forward = 1")

	*new = buf.String()
	return nil
}
