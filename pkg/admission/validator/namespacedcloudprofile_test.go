// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
)

var _ = Describe("NamespacedCloudProfile Validator", func() {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		namespace   string
		ctx         = context.Background()

		namespacedCloudProfileValidator extensionswebhook.Validator
		namespacedCloudProfile          *core.NamespacedCloudProfile
		cloudProfile                    *v1beta1.CloudProfile
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(install.AddToScheme(scheme))
		utilruntime.Must(v1beta1.AddToScheme(scheme))
		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()
		fakeManager = &test.FakeManager{
			Client: fakeClient,
			Scheme: scheme,
		}
		namespace = "garden-dev"

		namespacedCloudProfileValidator = validator.NewNamespacedCloudProfileValidator(fakeManager)
		namespacedCloudProfile = &core.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile-1",
				Namespace: namespace,
			},
			Spec: core.NamespacedCloudProfileSpec{
				Parent: core.CloudProfileReference{
					Name: "cloud-profile",
					Kind: "CloudProfile",
				},
			},
		}
		cloudProfile = &v1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cloud-profile",
			},
		}
	})

	Describe("#Validate", func() {
		It("should succeed for NamespacedCloudProfile without provider config", func() {
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should succeed if NamespacedCloudProfile is in deletion phase", func() {
			namespacedCloudProfile.DeletionTimestamp = ptr.To(metav1.Now())

			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should succeed if the NamespacedCloudProfile correctly defines new machine images and types", func() {
			cloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[{"name":"image-1","versions":[{"version":"1.0","image":"imgRef1","ami":"ami-123"}]}]
}`)}
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","image":"imgRef2"}]},
  {"name":"image-2","versions":[{"version":"2.0","image":"imgRef3"}]}
]
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name:     "image-1",
					Versions: []core.MachineImageVersion{{ExpirableVersion: core.ExpirableVersion{Version: "1.1"}, Architectures: []string{"amd64"}}},
				},
				{
					Name:     "image-2",
					Versions: []core.MachineImageVersion{{ExpirableVersion: core.ExpirableVersion{Version: "2.0"}, Architectures: []string{"amd64"}}},
				},
			}
			namespacedCloudProfile.Spec.MachineTypes = []core.MachineType{
				{Name: "type-2"},
			}
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should fail for NamespacedCloudProfile with invalid parent kind", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig"
}`)}
			namespacedCloudProfile.Spec.Parent = core.CloudProfileReference{
				Name: "cloud-profile",
				Kind: "NamespacedCloudProfile",
			}

			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(MatchError(ContainSubstring("parent reference must be of kind CloudProfile")))
		})

		It("should fail for NamespacedCloudProfile trying to override an already existing machine image version", func() {
			cloudProfile.Spec.MachineImages = []v1beta1.MachineImage{
				{Name: "image-1", Versions: []v1beta1.MachineImageVersion{{ExpirableVersion: v1beta1.ExpirableVersion{Version: "1.0"}}}},
			}
			cloudProfile.Spec.MachineTypes = []v1beta1.MachineType{{Name: "type-1"}}

			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","image":"imgRef1"}]}
]
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.0"}, Architectures: []string{"amd64"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.providerConfig.machineImages[0].versions[0]"),
				"Detail": Equal("machine image version image-1@1.0 is already defined in the parent CloudProfile"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying provider config without the according version in the spec.machineImages", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","image":"imgRef2"}]}
]
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.2"}, Architectures: []string{"amd64"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.2 is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("spec.providerConfig.machineImages[0].versions[0]"),
				"BadValue": Equal("image-1@1.1"),
				"Detail":   Equal("machine image version is not defined in the NamespacedCloudProfile"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages[0].versions[0]"),
				"Detail": Equal("machine image version image-1@1.2 and architecture: amd64 is not defined in the providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1 has an excess entry for architecture \"amd64\", which is not defined in the machineImages spec"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying new spec.machineImages without the according version and architecture entries in the provider config", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[
	{"version":"1.1","image":"image-id-1","architecture":"arm64"},
	{"version":"1.1","image":"image-id-2","architecture":"amd64"},
    {"version":"1.1-fallback","image":"image-id-3"}
  ]}
]
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1"}, Architectures: []string{"amd64", "arm64"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1-fallback"}, Architectures: []string{"arm64"}},
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1-missing"}, Architectures: []string{"arm64"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages[0].versions[1]"),
				"Detail": Equal("machine image version image-1@1.1-fallback and architecture: arm64 is not defined in the providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages[0].versions[2]"),
				"Detail": Equal("machine image version image-1@1.1-missing and architecture: arm64 is not defined in the providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-fallback is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-missing is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-fallback has an excess entry for architecture \"amd64\", which is not defined in the machineImages spec"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying new spec.machineImages without the according version in the provider config", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig"
}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-3",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "3.0"}},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages[0]"),
				"Detail": Equal("must provide an image mapping for image \"image-3\" in providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image image-3 is not defined in the NamespacedCloudProfile providerConfig"),
			}))))
		})
	})
})
