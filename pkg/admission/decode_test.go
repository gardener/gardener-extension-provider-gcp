// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package admission

import (
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apisgcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

func equalBackupBucketConfig(a, b *apisgcp.BackupBucketConfig) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	return reflect.DeepEqual(a.Immutability, b.Immutability)
}

var _ = Describe("Decode", func() {
	var (
		decoder runtime.Decoder
		scheme  *runtime.Scheme
	)
	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(core.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcp.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcpv1alpha1.AddToScheme(scheme)).To(Succeed())

		decoder = serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()

	})
	DescribeTable("DecodeBackupBucketConfig",
		func(config *runtime.RawExtension, want *apisgcp.BackupBucketConfig, wantErr bool) {
			got, err := DecodeBackupBucketConfig(decoder, config)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(equalBackupBucketConfig(got, want)).To(BeTrue())
		},
		Entry("valid config", &runtime.RawExtension{Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig", "immutability": {"retentionType": "bucket", "retentionPeriod": "24h", "locked": true}}`)}, &apisgcp.BackupBucketConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "gcp.provider.extensions.gardener.cloud/v1alpha1",
				Kind:       "BackupBucketConfig",
			},
			Immutability: &apisgcp.ImmutableConfig{
				RetentionType:   "bucket",
				RetentionPeriod: metav1.Duration{Duration: 24 * time.Hour},
				Locked:          true,
			},
		}, false),
		Entry("invalid config", &runtime.RawExtension{Raw: []byte(`invalid`)}, nil, true),
		Entry("missing fields", &runtime.RawExtension{Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1","kind": "BackupBucketConfig"}`)}, &apisgcp.BackupBucketConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "gcp.provider.extensions.gardener.cloud/v1alpha1",
				Kind:       "BackupBucketConfig",
			},
		}, false),
		Entry("different data in provider config", &runtime.RawExtension{Raw: []byte(`{"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1", "kind": "DifferentConfig", "someField": "someValue"}`)}, nil, true),
	)
})
