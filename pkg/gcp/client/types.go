//  Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.
//

package client

import (
	compute "google.golang.org/api/compute/v1"
	iam "google.golang.org/api/iam/v1"
)

// Network is a type alias for the GCP client type.
type Network = compute.Network

// Subnetwork is a type alias for the GCP client type.
type Subnetwork = compute.Subnetwork

// Router is a type alias for the GCP client type.
type Router = compute.Router

// Route is a type alias for the GCP client type.
type Route = compute.Route

// RouterNat is a type alias for the GCP client type.
type RouterNat = compute.RouterNat

// Firewall is a type alias for the GCP client type.
type Firewall = compute.Firewall

// Address is a type alias for the GCP client type.
type Address = compute.Address

// ServiceAccount is a type alias for the GCP client type.
type ServiceAccount = iam.ServiceAccount
