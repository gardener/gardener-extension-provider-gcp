// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

//go:generate mockgen -package client -destination=mocks.go github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client Factory,DNSClient,ComputeClient,StorageClient

package client
