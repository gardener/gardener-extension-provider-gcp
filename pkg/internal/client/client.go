// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gcp

import (
	"context"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

type client struct {
	service *compute.Service
}

type firewallsService struct {
	firewallsService *compute.FirewallsService
}

type routesService struct {
	routesService *compute.RoutesService
}
type instancesService struct {
	instancesService *compute.InstancesService
}
type instancesGetCall struct {
	instancesGetCall *compute.InstancesGetCall
}
type instancesInsertCall struct {
	instancesInsertCall *compute.InstancesInsertCall
}
type instancesDeleteCall struct {
	instancesDeleteCall *compute.InstancesDeleteCall
}
type disksService struct {
	disksService *compute.DisksService
}
type disksInsertCall struct {
	disksInsertCall *compute.DisksInsertCall
}

type disksGetCall struct {
	disksGetCall *compute.DisksGetCall
}

type disksDeleteCall struct {
	disksDeleteCall *compute.DisksDeleteCall
}
type regionsService struct {
	regionsService *compute.RegionsService
}
type regionsGetCall struct {
	regionsGetCall *compute.RegionsGetCall
}
type imagesService struct {
	imagesService *compute.ImagesService
}
type firewallsListCall struct {
	firewallsListCall *compute.FirewallsListCall
}

type firewallsGetCall struct {
	firewallsGetCall *compute.FirewallsGetCall
}

type firewallsInsertCall struct {
	firewallsInsertCall *compute.FirewallsInsertCall
}

type routesListCall struct {
	routesListCall *compute.RoutesListCall
}

type firewallsDeleteCall struct {
	firewallsDeleteCall *compute.FirewallsDeleteCall
}

type firewallsPatchCall struct {
	firewallsPatchCall *compute.FirewallsPatchCall
}

type routesDeleteCall struct {
	routesDeleteCall *compute.RoutesDeleteCall
}

type imagesListCall struct {
	imagesListCall *compute.ImagesListCall
}

// NewFromServiceAccount creates a new client from the given service account.
func NewFromServiceAccount(ctx context.Context, serviceAccount []byte) (Interface, error) {
	jwt, err := google.JWTConfigFromJSON(serviceAccount, compute.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	httpClient := oauth2.NewClient(ctx, jwt.TokenSource(ctx))
	service, err := compute.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	return New(service), nil
}

// New creates a new client backed by the given compute service.
func New(compute *compute.Service) Interface {
	return &client{compute}
}

// Firewalls implements Interface.
func (c *client) Firewalls() FirewallsService {
	return &firewallsService{c.service.Firewalls}
}

// Routes implements Interface.
func (c *client) Routes() RoutesService {
	return &routesService{c.service.Routes}
}

// Interfaces implements Interface.
func (c *client) Instances() InstancesService {
	return &instancesService{c.service.Instances}
}

// Disks implements Interface.
func (c *client) Disks() DisksService {
	return &disksService{c.service.Disks}
}

// Regions implements Interface.
func (c *client) Regions() RegionsService {
	return &regionsService{c.service.Regions}
}

// Images implements Interface.
func (c *client) Images() ImagesService {
	return &imagesService{c.service.Images}
}

// Get implements InstancesService.
func (i *instancesService) Get(projectID string, zone string, instance string) InstancesGetCall {
	return &instancesGetCall{i.instancesService.Get(projectID, zone, instance)}
}

// Insert implements InstancesService.
func (i *instancesService) Insert(projectID string, zone string, instance *compute.Instance) InstancesInsertCall {
	return &instancesInsertCall{i.instancesService.Insert(projectID, zone, instance)}
}

// Delete implements InstancesDeleteCall.
func (i *instancesService) Delete(projectID string, zone string, instance string) InstancesDeleteCall {
	return &instancesDeleteCall{i.instancesService.Delete(projectID, zone, instance)}
}

// List implements FirewallsService.
func (f *firewallsService) List(projectID string) FirewallsListCall {
	return &firewallsListCall{f.firewallsService.List(projectID)}
}

// Get implements FirewallsService.
func (f *firewallsService) Get(projectID string, firewall string) FirewallsGetCall {
	return &firewallsGetCall{f.firewallsService.Get(projectID, firewall)}
}

// Insert implements FirewallsService.
func (f *firewallsService) Insert(projectID string, rb *compute.Firewall) FirewallsInsertCall {
	return &firewallsInsertCall{f.firewallsService.Insert(projectID, rb)}
}

// Patch implements FirewallsService.
func (f *firewallsService) Patch(projectID string, firewall string, rb *compute.Firewall) FirewallsPatchCall {
	return &firewallsPatchCall{f.firewallsService.Patch(projectID, firewall, rb)}
}

// List implements RoutesService.
func (r *routesService) List(projectID string) RoutesListCall {
	return &routesListCall{r.routesService.List(projectID)}
}

// Pages implements FirewallsListCall.
func (c *firewallsListCall) Pages(ctx context.Context, f func(*compute.FirewallList) error) error {
	return c.firewallsListCall.Pages(ctx, f)
}

// Pages implements RoutesListCall.
func (c *routesListCall) Pages(ctx context.Context, f func(*compute.RouteList) error) error {
	return c.routesListCall.Pages(ctx, f)
}

// Delete implements FirewallsService.
func (f *firewallsService) Delete(projectID, firewall string) FirewallsDeleteCall {
	return &firewallsDeleteCall{f.firewallsService.Delete(projectID, firewall)}
}

// Delete implements RoutesService.
func (r *routesService) Delete(projectID, route string) RoutesDeleteCall {
	return &routesDeleteCall{r.routesService.Delete(projectID, route)}
}

// Insert implements DisksService.
func (d *disksService) Insert(projectID string, zone string, disk *compute.Disk) DisksInsertCall {
	return &disksInsertCall{d.disksService.Insert(projectID, zone, disk)}
}

// Get implements DisksService.
func (d *disksService) Get(projectID string, zone string, disk string) DisksGetCall {
	return &disksGetCall{d.disksService.Get(projectID, zone, disk)}
}

// Delete implements DisksService.
func (d *disksService) Delete(projectID string, zone string, disk string) DisksDeleteCall {
	return &disksDeleteCall{d.disksService.Delete(projectID, zone, disk)}
}

// List implements ImagesService.
func (i *imagesService) List(projectID string) ImagesListCall {
	return &imagesListCall{i.imagesService.List(projectID)}
}

// Context implements FirewallsDeleteCall.
func (c *firewallsDeleteCall) Context(ctx context.Context) FirewallsDeleteCall {
	return &firewallsDeleteCall{c.firewallsDeleteCall.Context(ctx)}
}

// Context implements RoutesDeleteCall.
func (c *routesDeleteCall) Context(ctx context.Context) RoutesDeleteCall {
	return &routesDeleteCall{c.routesDeleteCall.Context(ctx)}
}

// Do implements FirewallsDeleteCall.
func (c *firewallsDeleteCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.firewallsDeleteCall.Do(opts...)
}

// Do implements RoutesDeleteCall.
func (c *routesDeleteCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.routesDeleteCall.Do(opts...)
}

// Do implements FirewallsGetCall.
func (c *firewallsGetCall) Do(opts ...googleapi.CallOption) (*compute.Firewall, error) {
	return c.firewallsGetCall.Do(opts...)
}

// Context implements FirewallsGetCall.
func (c *firewallsGetCall) Context(ctx context.Context) FirewallsGetCall {
	return &firewallsGetCall{c.firewallsGetCall.Context(ctx)}
}

// Do implements FirewallsInsertCall.
func (c *firewallsInsertCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.firewallsInsertCall.Do(opts...)
}

// Context implements FirewallsGetCall.
func (c *firewallsInsertCall) Context(ctx context.Context) FirewallsInsertCall {
	return &firewallsInsertCall{c.firewallsInsertCall.Context(ctx)}
}

// Do implements FirewallsPatchCall.
func (c *firewallsPatchCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.firewallsPatchCall.Do(opts...)
}

// Context implements FirewallsPatchCall.
func (c *firewallsPatchCall) Context(ctx context.Context) FirewallsPatchCall {
	return &firewallsPatchCall{c.firewallsPatchCall.Context(ctx)}
}

// Do implements InstancesGetCall.
func (c *instancesGetCall) Do(opts ...googleapi.CallOption) (*compute.Instance, error) {
	return c.instancesGetCall.Do(opts...)
}

// Context implements InstancesGetCall.
func (c *instancesGetCall) Context(ctx context.Context) InstancesGetCall {
	return &instancesGetCall{c.instancesGetCall.Context(ctx)}
}

// Do implements InstancesDeleteCall.
func (c *instancesDeleteCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.instancesDeleteCall.Do(opts...)
}

// Context implements InstancesGetCall.
func (c *instancesDeleteCall) Context(ctx context.Context) InstancesDeleteCall {
	return &instancesDeleteCall{c.instancesDeleteCall.Context(ctx)}
}

// Do implements InstancesInsertCall.
func (c *instancesInsertCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.instancesInsertCall.Do(opts...)
}

// Context implements InstancesInsertCall.
func (c *instancesInsertCall) Context(ctx context.Context) InstancesInsertCall {
	return &instancesInsertCall{c.instancesInsertCall.Context(ctx)}
}

// Do implements DisksInsertCall.
func (c *disksInsertCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.disksInsertCall.Do(opts...)
}

// Context implements DisksInsertCall.
func (c *disksInsertCall) Context(ctx context.Context) DisksInsertCall {
	return &disksInsertCall{c.disksInsertCall.Context(ctx)}
}

// Do implements DisksGetCall.
func (c *disksGetCall) Do(opts ...googleapi.CallOption) (*compute.Disk, error) {
	return c.disksGetCall.Do(opts...)
}

// Context implements DisksGetCall.
func (c *disksGetCall) Context(ctx context.Context) DisksGetCall {
	return &disksGetCall{c.disksGetCall.Context(ctx)}
}

// Do implements DisksDeleteCall.
func (c *disksDeleteCall) Do(opts ...googleapi.CallOption) (*compute.Operation, error) {
	return c.disksDeleteCall.Do(opts...)
}

// Context implements DisksDeleteCall.
func (c *disksDeleteCall) Context(ctx context.Context) DisksDeleteCall {
	return &disksDeleteCall{c.disksDeleteCall.Context(ctx)}
}

// Get implements RegionsService.
func (r *regionsService) Get(projectID string, region string) RegionsGetCall {
	return &regionsGetCall{r.regionsService.Get(projectID, region)}
}

// Do implements RegionsGetCall.
func (r *regionsGetCall) Do(opts ...googleapi.CallOption) (*compute.Region, error) {
	return r.regionsGetCall.Do(opts...)
}

// Context implements RegionsGetCall.
func (r *regionsGetCall) Context(ctx context.Context) RegionsGetCall {
	return &regionsGetCall{r.regionsGetCall.Context(ctx)}
}

// Do implements ImagesListCall.
func (i *imagesListCall) Do(opts ...googleapi.CallOption) (*compute.ImageList, error) {
	return i.imagesListCall.Do(opts...)
}

// Fields implements ImagesListCall.
func (i *imagesListCall) Fields(s ...googleapi.Field) *compute.ImagesListCall {
	return i.imagesListCall.Fields(s...)
}

// OrderBy implements ImagesListCall
func (i *imagesListCall) OrderBy(orderBy string) *compute.ImagesListCall {
	return i.imagesListCall.OrderBy(orderBy)
}
