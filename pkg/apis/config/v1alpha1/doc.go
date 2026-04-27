// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +k8s:conversion-gen=github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

//go:generate crd-ref-docs --source-path=. --config ../../../../hack/api-reference/config.yaml --renderer=markdown --templates-dir $GARDENER_HACK_DIR/api-reference/template --output-path ../../../../hack/api-reference/config.md --log-level=ERROR

// Package v1alpha1 contains the GCP provider configuration API resources.
// +groupName=	gcp.provider.extensions.config.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener-extension-provider-gcp/pkg/apis/config/v1alpha1"
