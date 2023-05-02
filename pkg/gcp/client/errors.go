// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

// IgnoreNotFoundError returns nil if the error is a NotFound error. Otherwise, it returns the original error.
func IgnoreNotFoundError(err error) error {
	return IgnoreErrorCodes(err, http.StatusNotFound)
}

// IsNotFoundError returns true if the error has a HTTP 404 status code.
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
