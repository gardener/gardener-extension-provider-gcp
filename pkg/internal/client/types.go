// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	// Regions retrieves the GCP Regions Service
	Regions() RegionsService
	// Images retrieves the GCP Images Service
	Images() ImagesService
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
	// Patch initiates a FirewallsPatchCall.
	Patch(projectID string, firewall string, rb *compute.Firewall) FirewallsPatchCall
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
	// Delete initiates a DisksDeleteCall
	Delete(project string, zone string, disk string) DisksDeleteCall
	// Insert initiates a DisksServiceCall
	Insert(projectID string, zone string, disk *compute.Disk) DisksInsertCall
}

// RegionsService is the interface for the GCP Regions service.
type RegionsService interface {
	// Get initiates a RegionsServiceCall
	Get(projectID string, region string) RegionsGetCall
}

// ImagesService is the interface for the GCP Image service.
type ImagesService interface {
	// List initiates a ImagesListCall
	List(projectID string) *compute.ImagesListCall
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

// FirewallsPatchCall is a patch call to the firewalls service.
type FirewallsPatchCall interface {
	// Do executes the patch call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)

	// Context sets the context for patch call.
	Context(context.Context) FirewallsPatchCall
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

// InstancesInsertCall is a insert call to the instances service.
type InstancesInsertCall interface {
	// Do executes the deletion call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the deletion call.
	Context(context.Context) InstancesInsertCall
}

// DisksInsertCall is a insert call to the Disks service.
type DisksInsertCall interface {
	// Do executes the deletion call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the deletion call.
	Context(context.Context) DisksInsertCall
}

// DisksGetCall is a get call to the Disks service.
type DisksGetCall interface {
	// Do executes the get call.
	Do(opts ...googleapi.CallOption) (*compute.Disk, error)
	// Context sets the context for the get call.
	Context(context.Context) DisksGetCall
}

// DisksDeleteCall is a delete call to the Disks service.
type DisksDeleteCall interface {
	// Do executes the delete call.
	Do(opts ...googleapi.CallOption) (*compute.Operation, error)
	// Context sets the context for the delete call.
	Context(context.Context) DisksDeleteCall
}

// RegionsGetCall is a get call to the Region service.
type RegionsGetCall interface {
	// Do executes the get call.
	Do(opts ...googleapi.CallOption) (*compute.Region, error)
	// Context sets the context for the get call.
	Context(context.Context) RegionsGetCall
}
