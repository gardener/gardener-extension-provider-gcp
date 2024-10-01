// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"

	apisgcp "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/validation"
)

var _ = Describe("#ValidateWorkloadIdentityConfig", func() {
	var (
		workloadIdentityConfig *apisgcp.WorkloadIdentityConfig
	)

	BeforeEach(func() {
		workloadIdentityConfig = &apisgcp.WorkloadIdentityConfig{
			ProjectID: "my-project",
			CredentialsConfig: &runtime.RawExtension{
				Raw: []byte(`
{
	"universe_domain": "googleapis.com",
	"type": "external_account",
	"audience": "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
	"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
	"token_url": "https://sts.googleapis.com/v1/token",
	"credential_source": {
		"file": "/abc/cloudprovider/xyz",
		"abc": {
		  "foo": "text"
		}
	}
}
`),
			},
		}
	})

	It("should validate the config successfully", func() {
		Expect(validation.ValidateWorkloadIdentityConfig(workloadIdentityConfig, field.NewPath(""))).To(BeEmpty())
	})

	It("should contain all expected validation errors", func() {
		workloadIdentityConfig.ProjectID = "_invalid"
		workloadIdentityConfig.CredentialsConfig.Raw = []byte(`
{
	"extra": "field",
	"type": "not_external_account",
	"audience": "//iam.googleapis.com/projects/11111111/locations/global/workloadIdentityPools/foopool/providers/fooprovider",
	"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
	"token_url": "https://sts.googleapis.com/v1/token",
	"credential_source": {
		"file": "/abc/cloudprovider/xyz",
		"abc": {
			"foo": "text"
		}
	}
}
`)
		errorList := validation.ValidateWorkloadIdentityConfig(workloadIdentityConfig, field.NewPath("providerConfig"))
		Expect(errorList).To(ConsistOfFields(
			Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("providerConfig.credentialsConfig"),
				"Detail": Equal("missing required field: \"universe_domain\""),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("providerConfig.credentialsConfig.type"),
				"Detail": Equal("should equal \"external_account\""),
			},
			Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("providerConfig.credentialsConfig"),
				"Detail": Equal("contains extra fields, required fields are: audience, subject_token_type, token_url, type, universe_domain"),
			},
			Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("providerConfig.projectID"),
				"BadValue": Equal("_invalid"),
				"Detail":   Equal("does not match the expected format"),
			},
		))
	})

	It("should validate the config successfully during update", func() {
		newConfig := workloadIdentityConfig.DeepCopy()
		Expect(validation.ValidateWorkloadIdentityConfigUpdate(workloadIdentityConfig, newConfig, field.NewPath(""))).To(BeEmpty())
	})

	It("should not allow chaning the projectID during update", func() {
		newConfig := workloadIdentityConfig.DeepCopy()
		newConfig.ProjectID = "valid123"
		errorList := validation.ValidateWorkloadIdentityConfigUpdate(workloadIdentityConfig, newConfig, field.NewPath("providerConfig"))

		Expect(errorList).To(ConsistOfFields(
			Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("providerConfig.projectID"),
				"BadValue": Equal("valid123"),
				"Detail":   Equal("field is immutable"),
			},
		))
	})
})
