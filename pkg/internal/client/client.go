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

type firewallsListCall struct {
	firewallsListCall *compute.FirewallsListCall
}

type routesListCall struct {
	routesListCall *compute.RoutesListCall
}

type firewallsDeleteCall struct {
	firewallsDeleteCall *compute.FirewallsDeleteCall
}

type routesDeleteCall struct {
	routesDeleteCall *compute.RoutesDeleteCall
}

// NewFromServiceAccount creates a new client from the given service account.
func NewFromServiceAccount(ctx context.Context, serviceAccount []byte) (Interface, error) {
	jwt, err := google.JWTConfigFromJSON(serviceAccount, compute.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	httpClient := oauth2.NewClient(ctx, jwt.TokenSource(ctx))
	service, err := compute.NewService(context.TODO(), option.WithHTTPClient(httpClient))
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

// List implements FirewallsService.
func (f *firewallsService) List(projectID string) FirewallsListCall {
	return &firewallsListCall{f.firewallsService.List(projectID)}
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
