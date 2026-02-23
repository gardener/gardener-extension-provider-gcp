// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"fmt"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/controlplane"
	genericcontrolplaneactuator "github.com/gardener/gardener/extensions/pkg/controller/controlplane/genericactuator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/controller/infrastructure/infraflow"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

const (
	// GracefulDeletionWaitInterval is the default interval for retry operations.
	GracefulDeletionWaitInterval = 1 * time.Minute
	// GracefulDeletionTimeout is the timeout that defines how long the actuator should wait for resources to be deleted
	GracefulDeletionTimeout = 10 * time.Minute
)

// NewActuator creates a new Actuator that acts upon and updates the status of ControlPlane resources.
func NewActuator(
	mgr manager.Manager,
	a controlplane.Actuator,
	gracefulDeletionTimeout time.Duration,
	gracefulDeletionWaitInterval time.Duration,
) controlplane.Actuator {
	return &actuator{
		Actuator:                     a,
		client:                       mgr.GetClient(),
		gracefulDeletionTimeout:      gracefulDeletionTimeout,
		gracefulDeletionWaitInterval: gracefulDeletionWaitInterval,
	}
}

// actuator is an Actuator that acts upon and updates the status of ControlPlane resources.
type actuator struct {
	controlplane.Actuator
	client                       client.Client
	gracefulDeletionTimeout      time.Duration
	gracefulDeletionWaitInterval time.Duration
}

func (a *actuator) Reconcile(
	ctx context.Context,
	log logr.Logger,
	cp *extensionsv1alpha1.ControlPlane,
	cluster *extensionscontroller.Cluster,
) (bool, error) {
	// Call Reconcile on the composed Actuator
	ok, err := a.Actuator.Reconcile(ctx, log, cp, cluster)
	if err != nil {
		return ok, err
	}

	return ok, a.removeIgnoreAnnotations(ctx, log, cluster)
}

// removeIgnoreAnnotations removes the ignore annotation from the control plane managed resource
// if the remove-ignore annotation is found on it.
func (a *actuator) removeIgnoreAnnotations(ctx context.Context, log logr.Logger, cluster *extensionscontroller.Cluster) error {
	if cluster == nil {
		return nil
	}

	seedControlPlaneMr := resourcesv1alpha1.ManagedResource{}
	err := a.client.Get(ctx, client.ObjectKey{
		Namespace: cluster.ObjectMeta.Name,
		Name:      genericcontrolplaneactuator.ControlPlaneSeedChartResourceName}, &seedControlPlaneMr)
	if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get control plane managed resource: %w", err)
	}
	if apierrors.IsNotFound(err) {
		log.Info("control plane managed resource not found")
		return nil
	}

	if metav1.HasAnnotation(seedControlPlaneMr.ObjectMeta, gcp.AnnotationRemoveIgnore) {
		err = infraflow.DeleteControlPlaneMrIgnoreAnnotation(ctx, log, a.client, &seedControlPlaneMr)
		if err != nil {
			return err
		}
	}

	return nil
}
