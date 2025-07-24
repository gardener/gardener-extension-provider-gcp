// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"fmt"
	"net/http"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
	"google.golang.org/api/googleapi"
)

type beNotFoundErrorMatcher struct{}

// BeNotFoundError returns a matcher that checks if an error is a googleapi.Error with Code http.StatusNotFound.
func BeNotFoundError() types.GomegaMatcher {
	return &beNotFoundErrorMatcher{}
}

func (m *beNotFoundErrorMatcher) Match(actual interface{}) (success bool, err error) {
	if actual == nil {
		return false, nil
	}

	actualErr, actualOk := actual.(*googleapi.Error)
	if !actualOk {
		return false, fmt.Errorf("expected a googleapi.Error.  got:\n%s", format.Object(actual, 1))
	}

	return actualErr.Code == http.StatusNotFound, nil
}

func (m *beNotFoundErrorMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be not found error")
}

func (m *beNotFoundErrorMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to not be not found error")
}
