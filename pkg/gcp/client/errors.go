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
