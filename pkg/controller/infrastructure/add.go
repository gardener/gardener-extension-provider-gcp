// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"net/http"

	"github.com/gardener/gardener/extensions/pkg/controller/infrastructure"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/helper"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
	gcpclient "github.com/gardener/gardener-extension-provider-gcp/pkg/gcp/client"
)

var (
	// DefaultAddOptions are the default AddOptions for AddToManager.
	DefaultAddOptions = AddOptions{}
)

// AddOptions are options to apply when adding the GCP infrastructure controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	IgnoreOperationAnnotation bool
	// DisableProjectedTokenMount specifies whether the projected token mount shall be disabled for the terraformer.
	// Used for testing only.
	DisableProjectedTokenMount bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass

	// TokenMetadataClient is the client used to issue requests to the local token metadata server.
	TokenMetadataClient *http.Client
	// TokenMetadataURL is a function that constructs the URL that should be called in order to retrieve the token contents of a secret.
	TokenMetadataURL func(secretName, secretNamespace string) string
}

// AddToManagerWithOptions adds a controller with the given AddOptions to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	return infrastructure.Add(ctx, mgr, infrastructure.AddArgs{
		Actuator:          NewActuator(mgr, opts.DisableProjectedTokenMount, opts.TokenMetadataURL, opts.TokenMetadataClient),
		ConfigValidator:   NewConfigValidator(mgr, log.Log, gcpclient.New(opts.TokenMetadataURL, opts.TokenMetadataClient)),
		ControllerOptions: opts.Controller,
		Predicates:        infrastructure.DefaultPredicates(ctx, mgr, opts.IgnoreOperationAnnotation),
		Type:              gcp.Type,
		KnownCodes:        helper.KnownCodes,
		ExtensionClass:    opts.ExtensionClass,
	})
}

// AddToManager adds a controller with the default AddOptions.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}
