// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
)

var _ = Describe("KnownCodes", func() {
	DescribeTable("should match error messages to the correct error code",
		func(msg string, expectedCode v1beta1.ErrorCode) {
			Expect(KnownCodes[expectedCode](msg)).To(BeTrue())
		},
		Entry("unauthenticated: authentication failed",
			"Authentication failed for user",
			v1beta1.ErrorInfraUnauthenticated,
		),
		Entry("unauthorized: access denied",
			"Error 403: Access denied",
			v1beta1.ErrorInfraUnauthorized,
		),
		Entry("quota exceeded",
			"QUOTA_EXCEEDED: Quota has been met",
			v1beta1.ErrorInfraQuotaExceeded,
		),
		Entry("rate limits exceeded",
			"RequestLimitExceeded: too many requests",
			v1beta1.ErrorInfraRateLimitsExceeded,
		),
		Entry("dependencies: conflict",
			"Conflict: resource already exists",
			v1beta1.ErrorInfraDependencies,
		),
		Entry("retryable dependencies",
			"RetryableError: transient failure",
			v1beta1.ErrorRetryableInfraDependencies,
		),
		Entry("resources depleted: out of stock",
			"out of stock in zone us-central1-a",
			v1beta1.ErrorInfraResourcesDepleted,
		),
		Entry("configuration problem: disk type incompatible with machine type",
			"hyperdisk-balanced disk type cannot be used by t2d-standard-16 machine type.",
			v1beta1.ErrorConfigurationProblem,
		),
		Entry("configuration problem: invalid value",
			"Invalid value for field 'resource.machineType'",
			v1beta1.ErrorConfigurationProblem,
		),
		Entry("retryable configuration problem",
			"The requested configuration is currently not supported",
			v1beta1.ErrorRetryableConfigurationProblem,
		),
	)
})
