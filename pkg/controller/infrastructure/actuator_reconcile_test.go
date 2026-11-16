// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"encoding/json"
	"testing"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

func TestInfrastructureStateFromRaw(t *testing.T) {
	for _, kind := range []string{"InfrastructureState", "FlowState"} {
		for _, useObject := range []bool{false, true} {
			name := kind + "/raw"
			if useObject {
				name = kind + "/object"
			}
			t.Run(name, func(t *testing.T) {
				g := NewWithT(t)
				state := &v1alpha1.InfrastructureState{
					TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String(), Kind: kind},
					Data: map[string]string{
						"resources_exist":           "true",
						"service_account_exist":     "true",
						"ids/service-account-email": "nodes@project.iam.gserviceaccount.com",
					},
					Routes: []v1alpha1.Route{{InstanceName: "worker", DestinationCIDR: "10.0.0.0/24", Zone: "europe-west1-b"}},
				}
				raw := &runtime.RawExtension{Object: state}
				if !useObject {
					data, err := json.Marshal(state)
					g.Expect(err).NotTo(HaveOccurred())
					raw = &runtime.RawExtension{Raw: data}
				}
				infra := &extensionsv1alpha1.Infrastructure{}
				infra.Status.State = raw
				actual, err := (&actuator{}).infrastructureStateFromRaw(infra)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(actual).To(Equal(&gcp.InfrastructureState{
					TypeMeta: state.TypeMeta,
					Data:     state.Data,
					Routes:   []gcp.Route{{InstanceName: "worker", DestinationCIDR: "10.0.0.0/24", Zone: "europe-west1-b"}},
				}))
			})
		}
	}

	t.Run("empty state", func(t *testing.T) {
		g := NewWithT(t)
		for _, raw := range []*runtime.RawExtension{nil, {}} {
			infra := &extensionsv1alpha1.Infrastructure{}
			infra.Status.State = raw
			state, err := (&actuator{}).infrastructureStateFromRaw(infra)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(state).To(Equal(&gcp.InfrastructureState{}))
		}
	})

	for name, data := range map[string]string{
		"invalid JSON":          `invalid`,
		"legacy Terraform":      `{"data":"{}","encoding":"none"}`,
		"missing type metadata": `{"data":{"resources_exist":"true"}}`,
		"unknown API version":   `{"apiVersion":"other.example/v1alpha1","kind":"InfrastructureState"}`,
		"unknown kind":          `{"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1","kind":"InfrastructureConfig"}`,
	} {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			infra := &extensionsv1alpha1.Infrastructure{}
			infra.Status.State = &runtime.RawExtension{Raw: []byte(data)}
			state, err := (&actuator{}).infrastructureStateFromRaw(infra)
			g.Expect(err).To(HaveOccurred())
			g.Expect(state).To(BeNil())
		})
	}
}
