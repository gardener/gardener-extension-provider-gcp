// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"fmt"
	"net/http"

	"google.golang.org/api/googleapi"
)

// IsErrorCode checks if the error is of type googleapi.Error and the HTTP status matches one of the provided list of codes.
func IsErrorCode(err error, codes ...int) bool {
	if err == nil {
		return false
	}

	ae, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}

	for _, code := range codes {
		if ae.Code == code {
			return true
		}
	}

	return false
}

// IgnoreErrorCodes returns nil if the error matches one of the provided HTTP status codes.
func IgnoreErrorCodes(err error, codes ...int) error {
	if IsErrorCode(err, codes...) {
		return nil
	}

	return err
}

// IsRetentionPolicyNotMetError checks if the provided error is a Google API error with the reason "retentionPolicyNotMet".
// It returns true if the error is of type *googleapi.Error and contains an error with the specified reason,
// indicating that the retention policy has not been met. Otherwise, it returns false.
func IsRetentionPolicyNotMetError(err error) bool {
	if gErr, ok := err.(*googleapi.Error); ok {
		for _, e := range gErr.Errors {
			if e.Reason == "retentionPolicyNotMet" {
				return true
			}
		}
	}
	return false
}

// IgnoreNotFoundError returns nil if the error is a NotFound error. Otherwise, it returns the original error.
func IgnoreNotFoundError(err error) error {
	return IgnoreErrorCodes(err, http.StatusNotFound)
}

// IsNotFoundError returns true if the error has an HTTP 404 status code.
func IsNotFoundError(err error) bool {
	return IsErrorCode(err, http.StatusNotFound)
}

// InvalidUpdateError indicates an impossible update. When InvalidUpdateError is returned it means that an update was
// attempted on an immutable or unsupported field.
type InvalidUpdateError struct {
	fields []string
}

// NewInvalidUpdateError returns a new InvalidUpdateError.
func NewInvalidUpdateError(fields ...string) *InvalidUpdateError {
	return &InvalidUpdateError{
		fields: fields,
	}
}

func (i InvalidUpdateError) Error() string {
	return fmt.Sprintf("updating the following fields is not possible: %v", i.fields)
}
