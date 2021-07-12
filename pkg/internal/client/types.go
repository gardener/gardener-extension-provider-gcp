// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gcp

import (
	"context"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

// Interface is the interface for the GCP client.
type Interface interface {
	// Firewalls retrieves the GCP firewalls service.
	Firewalls() FirewallsService
	// Routes retrieves the GCP routes service.
	Routes() RoutesService
	// Instances retrieves the GCP instances Service
	Instances() InstancesService
	// Disks retrieves the GCP Disks Service
	Disks() DisksService
}

// FirewallsService is the interface for the GCP firewalls service.
type FirewallsService interface {
	// List initiates a FirewallsListCall.
	List(projectID string) FirewallsListCall
	// Delete initiates a FirewallsDeleteCall.
	Delete(projectID, firewall string) FirewallsDeleteCall
	// Get initiates a FirewallsGetCall.
	Get(projectID, firewall string) FirewallsGetCall
	// Insert initiates a FirewallsInsertCall.
	Insert(projectID string, rb *compute.Firewall) FirewallsInsertCall
}

// RoutesService is the interface for the GCP routes service.
type RoutesService interface {
	// List initiates a RoutesListCall.
	List(projectID string) RoutesListCall
	// Delete initiates a RoutesDeleteCall.
	Delete(projectID, route string) RoutesDeleteCall
}

// InstancesService is the interface for the GCP Instances service.
type InstancesService interface {
	// Get initiates a InstancesGetCall
	Get(projectID string, zone string, instance string) InstancesGetCall
	// Delete initiates a InstancesDeleteCall
	Delete(projectID string, zone string, instance string) InstancesDeleteCall
	// Insert initiates a InstancesInsertCall
	Insert(projectID string, zone string, instance *compute.Instance) InstancesInsertCall
}

// DisksService is the interface for the GCP Disks service.
type DisksService interface {
	// Get initiates a DisksServiceCall
	Get(projectID string, zone string, disk string) DisksGetCall
	// Insert initiates a DisksServiceCall
	Insert(projectID string, zone string, disk *compute.Disk) DisksInsertCall
}

// FirewallsListCall is a list call to the firewalls service.
type FirewallsListCall interface {
	// Pages runs the given function on the paginated result of listing the firewalls.
	Pages(context.Context, func(*compute.FirewallList) error) error
}

// FirewallsGetCall is a get call to the firewalls service.
type FirewallsGetCall interface {
	// Do executes the get call.
	Do(opts ...googleapi.CallOption) (*compute.Firewall, error)

	// Context sets the context for get call.
	Context(context.Context) FirewallsGetCall
}

// FirewallsInsertCall is a insert call to the firewalls service.
type FirewallsInsertCall interface {
	// Do executes the insert call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)

	// Context sets the context for Insert call.
	Context(context.Context) FirewallsInsertCall
}

// RoutesListCall is a list call to the routes service.
type RoutesListCall interface {
	// Pages runs the given function on the paginated result of listing the routes.
	Pages(context.Context, func(*compute.RouteList) error) error
}

// FirewallsDeleteCall is a delete call to the firewalls service.
type FirewallsDeleteCall interface {
	// Do executes the deletion call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the deletion call.
	Context(context.Context) FirewallsDeleteCall
}

// RoutesDeleteCall is a delete call to the routes service.
type RoutesDeleteCall interface {
	// Do executes the deletion call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the deletion call.
	Context(context.Context) RoutesDeleteCall
}

// InstancesGetCall is a get call to the instances service.
type InstancesGetCall interface {
	// Do executes the get call.
	Do(opts ...googleapi.CallOption) (*compute.Instance, error)
	// Context sets the context for the deletion call.
	Context(context.Context) InstancesGetCall
}

// InstancesDeleteCall is a deletion call to the instances service.
type InstancesDeleteCall interface {
	// Do executes the deletion call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the deletion call.
	Context(context.Context) InstancesDeleteCall
}

type InstancesInsertCall interface {
	// Do executes the deletion call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the deletion call.
	Context(context.Context) InstancesInsertCall
}

type DisksInsertCall interface {
	// Do executes the deletion call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the deletion call.
	Context(context.Context) DisksInsertCall
}

type DisksGetCall interface {
	// Do executes the get call.
	Do(opts ...googleapi.CallOption) (*compute.Disk, error)
	// Context sets the context for the get call.
	Context(context.Context) DisksGetCall
}
