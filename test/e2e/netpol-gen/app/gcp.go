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

package app

import (
	np "github.com/gardener/gardener/extensions/test/e2e/framework/networkpolicies"
)

// gcpNetworkPolicy holds GCP-specific network policy settings.
type gcpNetworkPolicy struct {
	np.Agnostic
	// metadata points to GCP-specific Metadata service.
	metadata *np.Host
}

// NewCloudAware returns gcp-specific configuration.
func NewCloudAware() np.CloudAware {
	return &gcpNetworkPolicy{
		metadata: &np.Host{
			Description: "Metadata service",
			HostName:    "169.254.169.254",
			Port:        80,
		},
	}
}

// Sources returns list of all GCP-specific sources and targets.
func (a *gcpNetworkPolicy) Rules() []np.Rule {
	ag := a.Agnostic
	return []np.Rule{
		a.newSource(ag.KubeAPIServer()).AllowPod(ag.EtcdMain(), ag.EtcdEvents()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.EtcdMain()).AllowHost(ag.External()).Build(),
		a.newSource(ag.EtcdEvents()).AllowHost(ag.External()).Build(),
		a.newSource(ag.CloudControllerManagerSecured()).AllowPod(ag.KubeAPIServer()).AllowHost(ag.External()).Build(),
		a.newSource(ag.Loki()).Build(),
		a.newSource(ag.Grafana()).AllowPod(ag.Prometheus(), ag.Loki()).Build(),
		a.newSource(ag.AddonManager()).AllowPod(ag.KubeAPIServer()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.KubeControllerManagerSecured()).AllowPod(ag.KubeAPIServer()).AllowHost(a.metadata, ag.External()).Build(),
		a.newSource(ag.KubeSchedulerSecured()).AllowPod(ag.KubeAPIServer()).Build(),
		a.newSource(ag.KubeStateMetricsShoot()).AllowPod(ag.KubeAPIServer()).Build(),
		a.newSource(ag.MachineControllerManager()).AllowPod(ag.KubeAPIServer()).AllowHost(ag.SeedKubeAPIServer(), ag.External()).Build(),
		a.newSource(ag.Prometheus()).AllowPod(
			ag.CloudControllerManagerSecured(),
			ag.EtcdEvents(),
			ag.EtcdMain(),
			ag.KubeAPIServer(),
			ag.KubeControllerManagerSecured(),
			ag.KubeSchedulerSecured(),
			ag.KubeStateMetricsShoot(),
			ag.MachineControllerManager(),
		).AllowTargetPod(ag.Loki().FromPort("metrics")).AllowHost(ag.SeedKubeAPIServer(), ag.External(), a.Agnostic.GardenPrometheus()).Build(),
	}
}

// EgressFromOtherNamespaces returns list of all GCP-specific sources and targets.
func (a *gcpNetworkPolicy) EgressFromOtherNamespaces(sourcePod *np.SourcePod) np.Rule {
	return np.NewSource(sourcePod).DenyPod(a.Sources()...).AllowPod(a.Agnostic.KubeAPIServer()).Build()
}

func (a *gcpNetworkPolicy) newSource(sourcePod *np.SourcePod) *np.RuleBuilder {
	return np.NewSource(sourcePod).DenyPod(a.Sources()...).DenyHost(a.metadata, a.Agnostic.External(), a.Agnostic.GardenPrometheus())
}

// Sources returns a list of SourcePods of GCP.
func (a *gcpNetworkPolicy) Sources() []*np.SourcePod {
	ag := a.Agnostic
	return []*np.SourcePod{
		ag.AddonManager(),
		ag.CloudControllerManagerSecured(),
		ag.Loki(),
		ag.EtcdEvents(),
		ag.EtcdMain(),
		ag.Grafana(),
		ag.KubeAPIServer(),
		ag.KubeControllerManagerSecured(),
		ag.KubeSchedulerSecured(),
		ag.KubeStateMetricsShoot(),
		ag.MachineControllerManager(),
		ag.Prometheus(),
	}
}

// Provider returns GCP cloud provider.
func (a *gcpNetworkPolicy) Provider() string {
	return "gcp"
}
