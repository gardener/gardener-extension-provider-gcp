// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
	"github.com/gardener/gardener-extension-provider-gcp/pkg/gcp"
)

// Reconciler is an interface for the infrastructure reconciliation.
type Reconciler interface {
	// Reconcile manages infrastructure resources according to desired spec.
	Reconcile(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
	// Delete removes any created infrastructure resource on the provider.
	Delete(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
	// Restore restores the infrastructure after a control plane migration. Effectively it performs a recovery of data from the infrastructure.status.state and
	// proceeds to reconcile.
	Restore(ctx context.Context, infra *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) error
}

// ReconcilerFactory can construct the different infrastructure reconciler implementations.
type ReconcilerFactory interface {
	Build(useFlow bool) (Reconciler, error)
}

// ReconcilerFactoryImpl is an implementation of a ReconcilerFactory
type ReconcilerFactoryImpl struct {
	log   logr.Logger
	a     *actuator
	infra *extensionsv1alpha1.Infrastructure
}

// Build builds the Reconciler according to the arguments.
func (f ReconcilerFactoryImpl) Build(useFlow bool) (Reconciler, error) {
	if useFlow {
		reconciler, err := NewFlowReconciler(f.a.client, f.a.restConfig, f.log, f.a.disableProjectedTokenMount)
		if err != nil {
			return nil, fmt.Errorf("failed to init flow reconciler: %w", err)
		}
		return reconciler, nil
	}

	reconciler := NewTerraformReconciler(f.a.client, f.a.restConfig, f.log, f.a.disableProjectedTokenMount)
	return reconciler, nil
}

// StrategySelector decides the reconciler used.
type StrategySelector interface {
	Select(infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) (bool, error)
}

// SelectorFunc decides the reconciler used.
type SelectorFunc func(*extensionsv1alpha1.Infrastructure, *extensions.Cluster) (bool, error)

// Select selects the reconciler implementation.
func (s SelectorFunc) Select(infrastructure *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) (bool, error) {
	return s(infrastructure, cluster)
}

// OnReconcile returns true if the operation should use the Flow for the given cluster.
func OnReconcile(infra *extensionsv1alpha1.Infrastructure, cluster *extensions.Cluster) (bool, error) {
	hasState, err := hasFlowState(infra.Status.State)
	if err != nil {
		return false, err
	}
	return hasState || HasFlowAnnotation(infra, cluster), nil
}

// OnDelete returns true if the operation should use the Flow deletion for the given cluster.
func OnDelete(infra *extensionsv1alpha1.Infrastructure, _ *extensions.Cluster) (bool, error) {
	return hasFlowState(infra.Status.State)
}

// OnRestore decides the reconciler used on migration.
var OnRestore = OnDelete

// HasFlowAnnotation returns true if the new flow reconciler should be used for the reconciliation.
func HasFlowAnnotation(infrastructure *extensionsv1alpha1.Infrastructure, cluster *controller.Cluster) bool {
	if ok := hasBoolAnnotation(infrastructure, gcp.GlobalAnnotationKeyUseFlow, gcp.AnnotationKeyUseFlow); ok != nil {
		return *ok
	}
	if shoot := cluster.Shoot; shoot != nil {
		if ok := hasBoolAnnotation(shoot, gcp.GlobalAnnotationKeyUseFlow, gcp.AnnotationKeyUseFlow); ok != nil {
			return *ok
		}
	}

	return false
}

func hasBoolAnnotation(o v1.Object, keys ...string) *bool {
	if annotations := o.GetAnnotations(); annotations != nil {
		for _, k := range keys {
			if v, ok := annotations[k]; ok {
				return pointer.Bool(strings.EqualFold(v, "true"))
			}
		}
	}

	return nil
}

func hasFlowState(state *runtime.RawExtension) (bool, error) {
	if state == nil {
		return false, nil
	}

	flowState := runtime.TypeMeta{}
	stateJson, err := state.MarshalJSON()
	if err != nil {
		return false, err
	}

	if err := json.Unmarshal(stateJson, &flowState); err != nil {
		return false, err
	}

	if flowState.GroupVersionKind().GroupVersion() == v1alpha1.SchemeGroupVersion {
		return true, nil
	}

	return false, nil
}
