// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
//

package infraflow

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp/v1alpha1"
)

const (
	// FlowStateKind is the kind used for the FlowState type.
	FlowStateKind = "FlowState"
)

var (
	// SchemeGroupVersion is the SchemeGroupVersion for use with the FlowState object.
	SchemeGroupVersion = v1alpha1.SchemeGroupVersion
)

// FlowState stores information about the infrastructure state for use with the FlowReconciler.
type FlowState struct {
	metav1.TypeMeta
	Data map[string]string `json:"data"`
}

// NewFlowState creates a new FlowState object.
func NewFlowState() *FlowState {
	return &FlowState{
		TypeMeta: metav1.TypeMeta{
			Kind:       FlowStateKind,
			APIVersion: SchemeGroupVersion.String(),
		},
		Data: map[string]string{},
	}
}

// ToJSON marshals state as JSON
func (f *FlowState) ToJSON() ([]byte, error) {
	return json.Marshal(f)
}

// HasValidVersion checks if flow version is supported.
func (f *FlowState) HasValidVersion() bool {
	return f != nil && f.Kind == FlowStateKind && f.APIVersion == SchemeGroupVersion.String()
}

// IsJSONFlowState returns true if the provided JSON is a valid FlowState
func IsJSONFlowState(raw []byte) (bool, error) {
	// first check if state is from flow or Terraformer
	marker := &metav1.TypeMeta{}
	if err := json.Unmarshal(raw, marker); err != nil {
		return false, err
	}

	if marker.Kind == FlowStateKind && marker.APIVersion == SchemeGroupVersion.String() {
		return true, nil
	}

	return false, nil
}

// NewFlowStateFromJSON unmarshals from JSON or YAML.
// Returns nil if input contains no kind field with value "FlowState".
func NewFlowStateFromJSON(raw []byte) (*FlowState, error) {
	state := &FlowState{}
	if err := json.Unmarshal(raw, state); err != nil {
		return nil, err
	}

	if state.Data == nil {
		state.Data = map[string]string{}
	}
	return state, nil
}
