// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var _ = Describe("Secret validation", func() {
	Describe("#ValidateCloudProviderSecret", func() {
		const (
			namespace  = "test-namespace"
			secretName = "test-secret"
		)

		var (
			secret  *corev1.Secret
			fldPath *field.Path
		)

		// Helper to create valid service account JSON with optional modifications
		createServiceAccountJSON := func(modifications map[string]any) string {
			baseJSON := map[string]any{
				"type":                        "service_account",
				"project_id":                  "my-project-123",
				"private_key_id":              "1234567890abcdef1234567890abcdef12345678",
				"private_key":                 "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
				"client_email":                "my-service-account@my-project-123.iam.gserviceaccount.com",
				"client_id":                   "123456789012345678901",
				"auth_uri":                    "https://accounts.google.com/o/oauth2/auth",
				"token_uri":                   "https://oauth2.googleapis.com/token",
				"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
				"client_x509_cert_url":        "https://www.googleapis.com/robot/v1/metadata/x509/my-service-account%40my-project-123.iam.gserviceaccount.com",
			}

			// Apply modifications
			for key, value := range modifications {
				if value == nil {
					delete(baseJSON, key)
				} else {
					baseJSON[key] = value
				}
			}

			jsonBytes, _ := json.Marshal(baseJSON)
			return string(jsonBytes)
		}

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{},
			}
			fldPath = field.NewPath("secret")
		})

		It("should pass with valid complete service account JSON", func() {
			secret.Data[gcp.ServiceAccountJSONField] = []byte(createServiceAccountJSON(nil))

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should pass with valid minimal service account JSON (only required fields)", func() {
			minimalJSON := createServiceAccountJSON(map[string]any{
				"client_id":                   nil, // remove optional fields
				"auth_uri":                    nil,
				"auth_provider_x509_cert_url": nil,
				"client_x509_cert_url":        nil,
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(minimalJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should fail when secret contains fields other than serviceaccount.json and storageAPIEndpoint", func() {
			secret.Data[gcp.ServiceAccountJSONField] = []byte(createServiceAccountJSON(nil))
			secret.Data["extra-field"] = []byte("value")

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data"),
			}))))
		})

		It("should pass when secret contains serviceaccount.json and storageAPIEndpoint", func() {
			secret.Data[gcp.ServiceAccountJSONField] = []byte(createServiceAccountJSON(nil))
			secret.Data["storageAPIEndpoint"] = []byte("https://storage.googleapis.com")

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(BeEmpty())
		})

		It("should fail when secret contains unexpected fields beyond serviceaccount.json and storageAPIEndpoint", func() {
			secret.Data[gcp.ServiceAccountJSONField] = []byte(createServiceAccountJSON(nil))
			secret.Data["storageAPIEndpoint"] = []byte("https://storage.googleapis.com")
			secret.Data["extra-field"] = []byte("value")

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data"),
			}))))
		})

		It("should fail when serviceaccount.json field is missing", func() {
			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("secret.data[serviceaccount.json]"),
			}))))
		})

		It("should fail when serviceaccount.json field is empty", func() {
			secret.Data[gcp.ServiceAccountJSONField] = []byte("")

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data[serviceaccount.json]"),
			}))))
		})

		It("should fail when serviceaccount.json contains invalid JSON", func() {
			secret.Data[gcp.ServiceAccountJSONField] = []byte("{invalid json}")

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("secret.data[serviceaccount.json]"),
				"BadValue": Equal("(hidden)"),
			}))))
		})

		It("should fail when required field 'type' is missing", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"type": nil, // remove field
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("secret.data[serviceaccount.json].type"),
			}))))
		})

		It("should fail when required field 'type' is empty", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"type": "",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data[serviceaccount.json].type"),
			}))))
		})

		It("should fail when type is not 'service_account'", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"type": "user_account",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeNotSupported),
				"Field":    Equal("secret.data[serviceaccount.json].type"),
				"BadValue": Equal("user_account"),
			}))))
		})

		It("should fail when project_id has invalid format", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"project_id": "My-Project-123", // uppercase not allowed
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data[serviceaccount.json].project_id"),
			}))))
		})

		It("should fail when client_email has invalid format", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"client_email": "not-a-service-account@example.com", // missing .iam.gserviceaccount.com
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data[serviceaccount.json].client_email"),
			}))))
		})

		It("should fail when optional field client_id is present but empty", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"client_id": "",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("secret.data[serviceaccount.json].client_id"),
				"BadValue": Equal("(hidden)"),
			}))))
		})

		It("should fail when client_id contains non-numeric characters", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"client_id": "abc123", // must be all numeric
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("secret.data[serviceaccount.json].client_id"),
				"BadValue": Equal("(hidden)"),
			}))))
		})

		It("should fail when private_key_id has invalid format", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"private_key_id": "ZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZZ", // uppercase not allowed
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("secret.data[serviceaccount.json].private_key_id"),
				"BadValue": Equal("(hidden)"),
			}))))
		})

		It("should fail when private_key doesn't have PEM header", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"private_key": "THIS-IS-A-FAKE-TEST-KEY\n-----END PRIVATE KEY-----\n",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("secret.data[serviceaccount.json].private_key"),
				"BadValue": Equal("(hidden)"),
			}))))
		})

		It("should fail when private_key doesn't have PEM footer", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"private_key": "-----BEGIN PRIVATE KEY-----\nTHIS-IS-A-FAKE-TEST-KEY",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("secret.data[serviceaccount.json].private_key"),
				"BadValue": Equal("(hidden)"),
			}))))
		})

		It("should fail when optional URL field token_uri is present but empty", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"token_uri": "",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("secret.data[serviceaccount.json].token_uri"),
			}))))
		})

		It("should fail when URL field uses http instead of https", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"auth_uri": "http://accounts.google.com/o/oauth2/auth",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data[serviceaccount.json].auth_uri"),
			}))))
		})

		It("should fail when URL field is malformed", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"token_uri": "not a valid url",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("secret.data[serviceaccount.json].token_uri"),
			}))))
		})

		It("should fail when unexpected field is present", func() {
			invalidJSON := createServiceAccountJSON(map[string]any{
				"unexpected_field": "value",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(invalidJSON)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("secret.data[serviceaccount.json].unexpected_field"),
			}))))
		})

		It("should pass when optional field universe_domain is present", func() {
			jsonWithUniverseDomain := createServiceAccountJSON(map[string]any{
				"universe_domain": "googleapis.com",
			})
			secret.Data[gcp.ServiceAccountJSONField] = []byte(jsonWithUniverseDomain)

			errs := ValidateCloudProviderSecret(secret, fldPath)
			Expect(errs).To(BeEmpty())
		})
	})
})
