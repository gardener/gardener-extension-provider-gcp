// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator_test

import (
	"context"
	"encoding/json"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
	mockmanager "github.com/gardener/gardener/third_party/mock/controller-runtime/manager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/admission/validator"
	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	apisgcpv1alpha1 "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

var _ = Describe("Seed Validator", func() {
	var (
		ctrl          *gomock.Controller
		mgr           *mockmanager.MockManager
		c             *mockclient.MockClient
		seedValidator extensionswebhook.Validator
		scheme        *runtime.Scheme
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		scheme = runtime.NewScheme()
		Expect(core.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcp.AddToScheme(scheme)).To(Succeed())
		Expect(apisgcpv1alpha1.AddToScheme(scheme)).To(Succeed())
		Expect(gardencorev1beta1.AddToScheme(scheme)).To(Succeed())
		c = mockclient.NewMockClient(ctrl)

		mgr = mockmanager.NewMockManager(ctrl)
		mgr.EXPECT().GetScheme().Return(scheme).AnyTimes()
		mgr.EXPECT().GetClient().Return(c).AnyTimes()
		seedValidator = validator.NewSeedValidator(mgr)
	})

	// Helper function to generate Seed objects
	generateSeed := func(retentionType, retentionPeriod string, locked bool, isImmutableConfigured bool) *core.Seed {
		var config *runtime.RawExtension
		if isImmutableConfigured {

			immutability := make(map[string]interface{})
			if retentionType != "" {
				immutability["retentionType"] = retentionType
			}
			if retentionPeriod != "" {
				immutability["retentionPeriod"] = retentionPeriod
				immutability["locked"] = locked
			}

			backupBucketConfig := map[string]interface{}{
				"apiVersion":   "gcp.provider.extensions.gardener.cloud/v1alpha1",
				"kind":         "BackupBucketConfig",
				"immutability": immutability,
			}
			raw, err := json.Marshal(backupBucketConfig)
			Expect(err).NotTo(HaveOccurred())
			config = &runtime.RawExtension{
				Raw: raw,
			}
		} else {
			config = nil
		}

		var backup *core.Backup
		if config != nil {
			backup = &core.Backup{
				ProviderConfig: config,
			}
		}

		return &core.Seed{
			Spec: core.SeedSpec{
				Backup: backup,
			},
		}
	}

	Describe("ValidateUpdate", func() {
		DescribeTable("Valid update scenarios",
			func(oldSeed, newSeed *core.Seed) {
				err := seedValidator.Validate(context.Background(), newSeed, oldSeed)
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("Immutable settings unchanged",
				generateSeed("bucket", "96h", false, true),
				generateSeed("bucket", "96h", false, true),
			),
			Entry("Retention period increased while locked",
				generateSeed("bucket", "96h", true, true),
				generateSeed("bucket", "120h", true, true),
			),
			Entry("Retention period decreased when not locked",
				generateSeed("bucket", "96h", false, true),
				generateSeed("bucket", "48h", false, true),
			),
			Entry("Adding immutability to an existing bucket without it",
				generateSeed("", "", false, false),
				generateSeed("bucket", "96h", false, true),
			),
			Entry("Adding immutability with locked=true",
				generateSeed("", "", false, false),
				generateSeed("bucket", "96h", true, true),
			),
			Entry("Retention period exactly at minimum (24h)",
				generateSeed("bucket", "24h", false, true),
				generateSeed("bucket", "24h", false, true),
			),
			Entry("Transitioning from locked=false to locked=true",
				generateSeed("bucket", "96h", false, true),
				generateSeed("bucket", "96h", true, true),
			),
			Entry("Disabling immutability when not locked",
				generateSeed("bucket", "96h", false, true),
				generateSeed("", "", false, false),
			),
			Entry("Backup not configured",
				generateSeed("", "", false, false),
				generateSeed("", "", false, false),
			),
		)

		DescribeTable("Invalid update scenarios",
			func(oldSeed, newSeed *core.Seed, expectedError string) {
				err := seedValidator.Validate(context.Background(), newSeed, oldSeed)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedError))
			},
			Entry("Disabling immutable settings is not allowed if locked",
				generateSeed("bucket", "96h", true, true),
				generateSeed("", "", false, true),
				"immutability cannot be disabled once it is locked",
			),
			Entry("Unlocking a locked retention policy is not allowed",
				generateSeed("bucket", "96h", true, true),
				generateSeed("bucket", "96h", false, true),
				"immutable retention policy lock cannot be unlocked once it is locked",
			),
			Entry("Reducing retention period when locked is not allowed",
				generateSeed("bucket", "96h", true, true),
				generateSeed("bucket", "48h", true, true),
				"reducing the retention period from",
			),
			Entry("Changing retentionType is not allowed",
				generateSeed("bucket", "96h", true, true),
				generateSeed("object", "96h", true, true),
				"must be 'bucket'",
			),
			Entry("Retention period below minimum when not locked is not allowed",
				generateSeed("bucket", "96h", true, true),
				generateSeed("bucket", "23h", false, true),
				"must be a positive duration greater than 24h",
			),
			Entry("Retention period below minimum when locked is not allowed",
				generateSeed("bucket", "96h", true, true),
				generateSeed("bucket", "23h", true, true),
				"must be a positive duration greater than 24h",
			),
			Entry("Invalid retention period format when locked is not allowed",
				generateSeed("bucket", "96h", true, true),
				generateSeed("bucket", "invalid", true, true),
				"invalid duration",
			),
			Entry("Invalid retention period format when not locked is not allowed",
				generateSeed("bucket", "96h", false, true),
				generateSeed("bucket", "invalid", false, true),
				"invalid duration",
			),
		)
	})

	Describe("ValidateCreate", func() {
		DescribeTable("Valid creation scenarios",
			func(newSeed *core.Seed) {
				err := seedValidator.Validate(context.Background(), newSeed, nil)
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("Creation with valid immutable settings",
				generateSeed("bucket", "96h", false, true),
			),
			Entry("Creation without immutable settings",
				generateSeed("", "", false, false),
			),
			Entry("Creation with locked immutable settings",
				generateSeed("bucket", "96h", true, true),
			),
			Entry("Retention period exactly at minimum (24h)",
				generateSeed("bucket", "24h", false, true),
			),
			Entry("Backup not configured",
				generateSeed("", "", false, false),
			),
		)

		DescribeTable("Invalid creation scenarios",
			func(newSeed *core.Seed, expectedError string) {
				err := seedValidator.Validate(context.Background(), newSeed, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(expectedError))
			},
			Entry("Invalid retention type",
				generateSeed("invalid", "96h", false, true),
				"must be 'bucket'",
			),
			Entry("Invalid retention period format",
				&core.Seed{
					Spec: core.SeedSpec{
						Backup: &core.Backup{
							ProviderConfig: &runtime.RawExtension{
								Raw: []byte(`{
									"apiVersion":"gcp.provider.extensions.gardener.cloud/v1alpha1",
									"kind":"BackupBucketConfig",
									"immutability":{
										"retentionType":"bucket",
										"retentionPeriod":"invalid"
									}
								}`),
							},
						},
					},
				},
				"invalid duration",
			),
			Entry("Negative retention period",
				generateSeed("bucket", "-96h", false, true),
				"must be a positive duration greater than 24h",
			),
			Entry("Retention period below minimum when not locked",
				generateSeed("bucket", "23h", false, true),
				"must be a positive duration greater than 24h",
			),
			Entry("Retention period below minimum when locked",
				generateSeed("bucket", "23h", true, true),
				"must be a positive duration greater than 24h",
			),
			Entry("Invalid retention period format when locked is not allowed",
				generateSeed("bucket", "invalid", true, true),
				"invalid duration",
			),
		)
	})
})
