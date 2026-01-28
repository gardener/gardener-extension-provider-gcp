// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"fmt"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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

var _ = DescribeTableSubtree("NamespacedCloudProfile Validator", func(isCapabilitiesCloudProfile bool) {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		namespace   string
		ctx         = context.Background()

		namespacedCloudProfileValidator extensionswebhook.Validator
		namespacedCloudProfile          *core.NamespacedCloudProfile
		cloudProfile                    *v1beta1.CloudProfile
		capabilitiesDefinitions         []v1beta1.CapabilityDefinition
	)

	BeforeEach(func() {
		if isCapabilitiesCloudProfile {
			capabilitiesDefinitions = []v1beta1.CapabilityDefinition{
				{Name: v1beta1constants.ArchitectureName, Values: []string{v1beta1constants.ArchitectureAMD64}},
			}
		}
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
			Spec: v1beta1.CloudProfileSpec{
				MachineCapabilities: capabilitiesDefinitions,
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
			versions := `{"name":"image-1","versions":[{"version":"1.0","image":"imgRef1"}]}`
			namespacedVersions := `{"name":"image-1","versions":[{"version":"1.1","image":"imgRef2"}]},
  {"name":"image-2","versions":[{"version":"2.0","image":"imgRef3"}]}`
			if isCapabilitiesCloudProfile {
				versions = `{"name":"image-1","versions":[{"version":"1.0","capabilityFlavors": [{"image": "imgRef1"}]}]}`
				namespacedVersions = `{"name":"image-1","versions":[{"version":"1.1","capabilityFlavors": [{"image": "imgRef2"}]}]},
				{"name":"image-2","versions":[{"version":"2.0","capabilityFlavors": [{"image": "imgRef3"}]}]}`
			}
			cloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[%s]
}`, versions))}
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
%s
]
}`, namespacedVersions))}
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

			namespacedVersions := `{"name":"image-1","versions":[{"version":"1.0","image":"imgRef1"}]}`
			if isCapabilitiesCloudProfile {
				namespacedVersions = `{"name":"image-1","versions":[{"version":"1.0","capabilityFlavors": [{"image": "imgRef1"}]}]}`
			}

			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
%s
]
}`, namespacedVersions))}
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
			namespacedVersions := `{"name":"image-1","versions":[{"version":"1.1","image":"imgRef2"}]}`
			if isCapabilitiesCloudProfile {
				namespacedVersions = `{"name":"image-1","versions":[{"version":"1.1","capabilityFlavors": [{"image": "imgRef2"}]}]}`
			}
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[%s]}`, namespacedVersions))}

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

			if isCapabilitiesCloudProfile {
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.providerConfig.machineImages"),
					"Detail": Equal("machine image version image-1@1.2 is not defined in the NamespacedCloudProfile providerConfig"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.providerConfig.machineImages[0].versions[0]"),
					"BadValue": Equal("image-1@1.1"),
					"Detail":   Equal("machine image version is not defined in the NamespacedCloudProfile"),
				}))))
			} else {
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.providerConfig.machineImages"),
					"Detail": Equal("machine image version image-1@1.2 and architecture: \"amd64\" is not defined in the NamespacedCloudProfile providerConfig"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.providerConfig.machineImages[0].versions[0]"),
					"BadValue": Equal("image-1@1.1"),
					"Detail":   Equal("machine image version is not defined in the NamespacedCloudProfile"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.providerConfig.machineImages"),
					"Detail": Equal("machine image version image-1@1.1 has an excess entry for architecture \"amd64\", which is not defined in the machineImages spec"),
				}))))
			}

		})

		It("should fail for NamespacedCloudProfile specifying provider config without the according version capabilityFlavor in the spec.machineImages", func() {
			if !isCapabilitiesCloudProfile {
				return
			}
			namespacedVersions := `{"name":"image-1","versions":[{"version":"1.2","capabilityFlavors": [{"image": "imgRef2", "capabilities": {"architecture": ["arm64"]}}]}]}`
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[%s]
}`, namespacedVersions))}

			cloudProfile.Spec.MachineCapabilities[0].Values = []string{v1beta1constants.ArchitectureARM64, v1beta1constants.ArchitectureAMD64}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.2"},
							Architectures:     []string{"amd64"},
							CapabilityFlavors: []core.MachineImageFlavor{{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}}}},
					},
				},
			}
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.2 and capabilityFlavor map[architecture:[amd64]] is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.2 has an excess capabilityFlavor map[architecture:[arm64]], which is not defined in the NamespacedCloudProfile machineImages spec"),
			}))))
		})

		It("should fail for NamespacedCloudProfile specifying new spec.machineImages without the according version and architecture/capabilities entries in the provider config", func() {
			namespacedVersions := `  {"name":"image-1","versions":[
	{"version":"1.1","image":"image-id-1","architecture":"arm64"},
	{"version":"1.1","image":"image-id-2","architecture":"amd64"},
    {"version":"1.1-fallback","image":"image-id-3"}
  ]}`
			if isCapabilitiesCloudProfile {
				cloudProfile.Spec.MachineCapabilities[0].Values = []string{v1beta1constants.ArchitectureARM64, v1beta1constants.ArchitectureAMD64}
				namespacedVersions = `{"name":"image-1","versions":[
	{"version":"1.1","capabilityFlavors": [
		{"capabilities": {"architecture": ["arm64"] }, "image": "image-id-1"},
		{"capabilities": {"architecture": ["amd64"] }, "image": "image-id-2"}
	]},
	{"version":"1.1-fallback","capabilityFlavors": [
		{"capabilities": {"architecture": ["amd64"] }, "image": "image-id-3"}
	]}
]}`
			}
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(fmt.Sprintf(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[%s]}`, namespacedVersions))}

			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1"}, Architectures: []string{"amd64", "arm64"},
							CapabilityFlavors: []core.MachineImageFlavor{
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}},
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}},
							},
						},
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1-fallback"}, Architectures: []string{"arm64"},
							CapabilityFlavors: []core.MachineImageFlavor{
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}}},
						},
						{ExpirableVersion: core.ExpirableVersion{Version: "1.1-missing"}, Architectures: []string{"arm64"},
							CapabilityFlavors: []core.MachineImageFlavor{
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}}},
						},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)

			if isCapabilitiesCloudProfile {
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.providerConfig.machineImages"),
					"Detail": Equal("machine image version image-1@1.1-fallback and capabilityFlavor map[architecture:[arm64]] is not defined in the NamespacedCloudProfile providerConfig"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.providerConfig.machineImages"),
					"Detail": Equal("machine image version image-1@1.1-fallback has an excess capabilityFlavor map[architecture:[amd64]], which is not defined in the NamespacedCloudProfile machineImages spec"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.providerConfig.machineImages"),
					"Detail": Equal("machine image version image-1@1.1-missing is not defined in the NamespacedCloudProfile providerConfig"),
				}))))
				return
			}

			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-missing and architecture: \"arm64\" is not defined in the NamespacedCloudProfile providerConfig"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image version image-1@1.1-fallback and architecture: \"arm64\" is not defined in the NamespacedCloudProfile providerConfig"),
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
				"Field":  Equal("spec.providerConfig.machineImages"),
				"Detail": Equal("machine image image-3 is not defined in the NamespacedCloudProfile providerConfig"),
			}))))
		})
	})
},
	Entry("CloudProfile uses architecture only", false),
	Entry("CloudProfile uses capabilities", true),
)

var _ = Describe("NamespacedCloudProfile Validator - Mixed Format", func() {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		namespace   string
		ctx         = context.Background()

		namespacedCloudProfileValidator extensionswebhook.Validator
		namespacedCloudProfile          *core.NamespacedCloudProfile
		cloudProfile                    *v1beta1.CloudProfile
		capabilitiesDefinitions         []v1beta1.CapabilityDefinition
	)

	BeforeEach(func() {
		capabilitiesDefinitions = []v1beta1.CapabilityDefinition{
			{Name: v1beta1constants.ArchitectureName, Values: []string{v1beta1constants.ArchitectureAMD64, v1beta1constants.ArchitectureARM64}},
		}
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
			Spec: v1beta1.CloudProfileSpec{
				MachineCapabilities: capabilitiesDefinitions,
			},
		}
	})

	Describe("#Validate - Mixed Format", func() {
		It("should succeed if NamespacedCloudProfile uses mixed format with some versions in old format and others in new format", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[
    {"version":"1.0","capabilityFlavors": [{"image": "imgRef1", "capabilities": {"architecture": ["amd64"]}}]},
    {"version":"1.1","image":"imgRef2","architecture":"amd64"}
  ]}
]}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion:  core.ExpirableVersion{Version: "1.0"},
							CapabilityFlavors: []core.MachineImageFlavor{{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}}},
						},
						{
							ExpirableVersion:  core.ExpirableVersion{Version: "1.1"},
							CapabilityFlavors: []core.MachineImageFlavor{{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}}},
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should succeed if NamespacedCloudProfile uses old format with multiple architectures", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[
    {"version":"1.1","image":"imgRef2-amd64","architecture":"amd64"},
    {"version":"1.1","image":"imgRef2-arm64","architecture":"arm64"}
  ]}
]}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{Version: "1.1"},
							CapabilityFlavors: []core.MachineImageFlavor{
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}},
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}},
							},
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())
			Expect(namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should fail if NamespacedCloudProfile uses old format with missing architecture mapping", func() {
			namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[
    {"version":"1.1","image":"imgRef2-amd64","architecture":"amd64"}
  ]}
]}`)}
			namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
				{
					Name: "image-1",
					Versions: []core.MachineImageVersion{
						{
							ExpirableVersion: core.ExpirableVersion{Version: "1.1"},
							CapabilityFlavors: []core.MachineImageFlavor{
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureAMD64}}},
								{Capabilities: core.Capabilities{v1beta1constants.ArchitectureName: []string{v1beta1constants.ArchitectureARM64}}},
							},
						},
					},
				},
			}
			Expect(fakeClient.Create(ctx, cloudProfile)).To(Succeed())

			err := namespacedCloudProfileValidator.Validate(ctx, namespacedCloudProfile, nil)
			Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeRequired),
				"Detail": ContainSubstring("arm64"),
			}))))
		})
	})
})
