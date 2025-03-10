// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_WorkloadIdentity set the default validation rules for workload identities.
func SetDefaults_WorkloadIdentity(obj *WorkloadIdentity) {
	if len(obj.AllowedTokenURLs) == 0 {
		obj.AllowedTokenURLs = []string{"https://sts.googleapis.com/v1/token"}
	}
	if len(obj.AllowedServiceAccountImpersonationURLRegExps) == 0 {
		obj.AllowedServiceAccountImpersonationURLRegExps = []string{`^https://iamcredentials\.googleapis\.com/v1/projects/-/serviceAccounts/.+:generateAccessToken$`}
	}
}
