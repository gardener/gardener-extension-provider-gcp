// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
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
