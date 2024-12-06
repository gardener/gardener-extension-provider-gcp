// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/util"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/mutator"
	api "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/install"
)

var _ = Describe("NamespacedCloudProfile Mutator", func() {
	var (
		fakeClient  client.Client
		fakeManager manager.Manager
		namespace   string
		ctx         = context.Background()
		decoder     runtime.Decoder

		namespacedCloudProfileMutator extensionswebhook.Mutator
		namespacedCloudProfile        *v1beta1.NamespacedCloudProfile
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
		decoder = serializer.NewCodecFactory(fakeManager.GetScheme(), serializer.EnableStrict).UniversalDecoder()

		namespacedCloudProfileMutator = mutator.NewNamespacedCloudProfileMutator(fakeManager)
		namespacedCloudProfile = &v1beta1.NamespacedCloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile-1",
				Namespace: namespace,
			},
		}
	})

	Describe("#Mutate", func() {
		It("should succeed for NamespacedCloudProfile without provider config", func() {
			Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())
		})

		It("should skip if NamespacedCloudProfile is in deletion phase", func() {
			namespacedCloudProfile.DeletionTimestamp = ptr.To(metav1.Now())
			expectedProfile := namespacedCloudProfile.DeepCopy()

			Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

			Expect(namespacedCloudProfile).To(DeepEqual(expectedProfile))
		})

		Describe("merge the provider configurations from a NamespacedCloudProfile and the parent CloudProfile", func() {
			It("should correctly merge extended machineImages", func() {
				namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.0","image":"imgRef1"}]}
]}`)}
				namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte(`{
"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
"kind":"CloudProfileConfig",
"machineImages":[
  {"name":"image-1","versions":[{"version":"1.1","image":"imgRef2","architecture":"armhf"}]},
  {"name":"image-2","versions":[{"version":"2.0","image":"imgRef3"}]}
]}`)}

				Expect(namespacedCloudProfileMutator.Mutate(ctx, namespacedCloudProfile, nil)).To(Succeed())

				mergedConfig, err := decodeCloudProfileConfig(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(mergedConfig.MachineImages).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Name": Equal("image-1"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "1.0", Image: "imgRef1", Architecture: ptr.To("amd64")},
							api.MachineImageVersion{Version: "1.1", Image: "imgRef2", Architecture: ptr.To("armhf")},
						),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Name":     Equal("image-2"),
						"Versions": ContainElements(api.MachineImageVersion{Version: "2.0", Image: "imgRef3", Architecture: ptr.To("amd64")}),
					}),
				))
			})
		})
	})
})

func decodeCloudProfileConfig(decoder runtime.Decoder, config *runtime.RawExtension) (*api.CloudProfileConfig, error) {
	cloudProfileConfig := &api.CloudProfileConfig{}
	if err := util.Decode(decoder, config.Raw, cloudProfileConfig); err != nil {
		return nil, err
	}
	return cloudProfileConfig, nil
}
