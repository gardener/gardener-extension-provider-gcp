// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure_test

import (
	"encoding/json"

	"github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("ShouldUseFlow", func() {
	Context("without any flow annotation", func() {
		It("should not use FlowContext", func() {
			cluster := makeCluster("11.0.0.0/16", "12.0.0.0/16")
			Expect(infrastructure.HasFlowAnnotation(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeFalse())
		})
	})
	Context("with flow annotation in infrastructure", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		cluster := makeCluster("11.0.0.0/16", "12.0.0.0/16")
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, gcp.AnnotationKeyUseFlow, "true")
		It("should use the FlowContext", func() {
			Expect(infrastructure.HasFlowAnnotation(infra, cluster)).To(BeTrue())
		})
	})
	Context("with flow annotation in shoot", func() {
		cluster := makeCluster("11.0.0.0/16", "12.0.0.0/16")
		metav1.SetMetaDataAnnotation(&cluster.Shoot.ObjectMeta, gcp.AnnotationKeyUseFlow, "true")
		It("should use the FlowContext", func() {
			Expect(infrastructure.HasFlowAnnotation(&extensionsv1alpha1.Infrastructure{}, cluster)).To(BeTrue())
		})
	})
})

var _ = Describe("ReconcilationStrategy", func() {
	cluster := makeCluster("11.0.0.0/16", "12.0.0.0/16")
	It("should use Flow if an annotation is found", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		metav1.SetMetaDataAnnotation(&infra.ObjectMeta, gcp.AnnotationKeyUseFlow, "true")

		useFlow, err := infrastructure.OnReconcile(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})
	It("should use Flow if resources were reconciled with Flow before, regardless of annotation", func() {
		state := newInfrastructureState()
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Object: state}

		useFlow, err := infrastructure.OnReconcile(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})

	It("should use Terraform if no flow state is found and there is no flow annotation", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Raw: getRawTerraformState(`{"provider": "terraform"}`)}

		useFlow, err := infrastructure.OnReconcile(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeFalse())
	})

	It("should delete with Terraform if resources were reconciled with Terraform", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		infra.Status.State = &runtime.RawExtension{Raw: getRawTerraformState(`{"provider": "terraform"}`)}

		useFlow, err := infrastructure.OnDelete(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeFalse())
	})
	It("should delete with Flow if resources were reconciled with Flow", func() {
		infra := &extensionsv1alpha1.Infrastructure{}
		state := newInfrastructureState()
		infra.Status.State = &runtime.RawExtension{Object: state}

		useFlow, err := infrastructure.OnDelete(infra, cluster)
		Expect(err).NotTo(HaveOccurred())
		Expect(useFlow).To(BeTrue())
	})

})

func getRawTerraformState(jsonContent string) []byte {
	state := &runtime.RawExtension{
		Raw: []byte(jsonContent),
	}
	stateRaw, _ := json.Marshal(state)
	return stateRaw
}

func newInfrastructureState() *v1alpha1.InfrastructureState {
	return &v1alpha1.InfrastructureState{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "InfrastructureState",
		},
	}
}

// makeCluster returns a cluster object used for testing.
//
//nolint:unparam
func makeCluster(pods, services string) *controller.Cluster {
	var (
		shoot = gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					Pods:     &pods,
					Services: &services,
				},
			},
		}
		cloudProfileConfig = v1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
		}
		cloudProfileConfigJSON, _ = json.Marshal(cloudProfileConfig)
		cloudProfile              = gardencorev1beta1.CloudProfile{
			Spec: gardencorev1beta1.CloudProfileSpec{
				ProviderConfig: &runtime.RawExtension{
					Raw: cloudProfileConfigJSON,
				},
			},
		}
	)

	return &controller.Cluster{
		Shoot:        &shoot,
		CloudProfile: &cloudProfile,
	}
}
