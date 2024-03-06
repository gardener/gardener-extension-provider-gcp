// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplaneexposure

import (
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the GCP exposure webhook to the manager.
type AddOptions struct {
	// ETCDStorage is the etcd storage configuration.
	ETCDStorage config.ETCDStorage
}

var logger = log.Log.WithName("gcp-controlplaneexposure-webhook")

// NewWithOptions a new control plane exposure webhook with the given options.
func NewWithOptions(mgr manager.Manager, opts AddOptions) (*extensionswebhook.Webhook, error) {
	logger.Info("Adding webhook to manager")
	return controlplane.New(mgr, controlplane.Args{
		Kind:     controlplane.KindSeed,
		Provider: gcp.Type,
		Types: []extensionswebhook.Type{
			{Obj: &appsv1.Deployment{}},
			{Obj: &druidv1alpha1.Etcd{}},
		},
		Mutator: genericmutator.NewMutator(mgr, NewEnsurer(&opts.ETCDStorage, logger), nil, nil, nil, logger),
	})
}

// New creates a new control plane exposure webhook with default options.
func New(mgr manager.Manager) (*extensionswebhook.Webhook, error) {
	return NewWithOptions(mgr, DefaultAddOptions)
}
