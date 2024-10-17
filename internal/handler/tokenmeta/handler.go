// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenmeta

import (
	"net/http"

	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Handler accepts http requests and retrieves the requested token from the corresponding secret.
type Handler struct {
	c   client.Client
	log logr.Logger
}

// New constructs a new [Handler].
func New(c client.Client, log logr.Logger) *Handler {
	return &Handler{
		c:   c,
		log: log,
	}
}

// RegisterRoutes adds the supported routes to the passed mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/namespaces/{namespace}/secrets/{secret}/token", h.tokenRequest)
}

func (h *Handler) tokenRequest(w http.ResponseWriter, r *http.Request) {
	namespaceName := r.PathValue("namespace")
	secretName := r.PathValue("secret")

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespaceName,
			Name:      secretName,
		},
	}

	if len(namespaceName) == 0 || len(secretName) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("Namespace and secret name are both required")); err != nil {
			h.log.Error(err, "Failed writing a response")
		}
		return
	}

	h.log.Info("Requesting secret", "secret", client.ObjectKeyFromObject(secret))

	if err := h.c.Get(r.Context(), client.ObjectKeyFromObject(secret), secret); err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			if _, err := w.Write([]byte("Not found error: " + err.Error())); err != nil {
				h.log.Error(err, "Failed writing a response")
			}
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("Unexpected error: " + err.Error())); err != nil {
			h.log.Error(err, "Failed writing a response")
		}
		return
	}

	if secret.Labels[securityv1alpha1constants.LabelPurpose] != securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("Secret is not with purpose " + securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor)); err != nil {
			h.log.Error(err, "Failed writing a response")
		}
		return
	}

	t, ok := secret.Data[securityv1alpha1constants.DataKeyToken]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("Secret does not contain a token")); err != nil {
			h.log.Error(err, "Failed writing a response")
		}
		return
	}

	if _, err := w.Write(t); err != nil {
		h.log.Error(err, "Failed writing a response")
	}
}
