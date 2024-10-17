// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenmeta_test

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener-extension-provider-gcp/internal/handler/tokenmeta"
)

var _ = Describe("Token Metadata Handler", func() {
	var (
		c   client.Client
		log logr.Logger

		handler *tokenmeta.Handler
		mux     *http.ServeMux

		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = fakeclient.NewClientBuilder().Build()
		log = logr.Discard()

		handler = tokenmeta.New(c, log)
		mux = http.NewServeMux()
		handler.RegisterRoutes(mux)

	})

	DescribeTable(
		"requests",
		func(method string, uri string, expectedStatus int, expectedResponseBytes []byte, expectedHeaders map[string]string, secret *corev1.Secret) {
			if secret != nil {
				Expect(c.Create(ctx, secret)).To(Succeed())
			}

			req := httptest.NewRequest(method, uri, nil)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(expectedStatus))
			Expect(recorder.Body.Bytes()).To(Equal(expectedResponseBytes))
			Expect(len(recorder.Result().Header)).To(Equal(len(expectedHeaders)))
			for k, v := range expectedHeaders {
				Expect(recorder.Result().Header[k]).To(Equal([]string{v}))
			}
		},
		Entry(
			"should return the requested token",
			http.MethodGet,
			"https://abc.def/namespaces/bar/secrets/foo/token",
			200,
			[]byte("sometoken"),
			map[string]string{
				"Content-Type": "text/plain; charset=utf-8",
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					Labels: map[string]string{
						"security.gardener.cloud/purpose": "workload-identity-token-requestor",
					},
				},
				Data: map[string][]byte{
					"token": []byte("sometoken"),
				},
			},
		),
		Entry(
			"should return not found if the secret is gone",
			http.MethodGet,
			"https://abc.def/namespaces/bar/secrets/notfound/token",
			404,
			[]byte("Not found error: secrets \"notfound\" not found"),
			map[string]string{},
			nil,
		),
		Entry(
			"should return bad request if the secret does not have the correct purpose label",
			http.MethodGet,
			"https://abc.def/namespaces/bar/secrets/foo/token",
			400,
			[]byte("Secret is not with purpose workload-identity-token-requestor"),
			map[string]string{},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
				Data: map[string][]byte{
					"token": []byte("sometoken"),
				},
			},
		),
		Entry(
			"should return bad request if the secret does not contain a token",
			http.MethodGet,
			"https://abc.def/namespaces/bar/secrets/foo/token",
			400,
			[]byte("Secret does not contain a token"),
			map[string]string{},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
					Labels: map[string]string{
						"security.gardener.cloud/purpose": "workload-identity-token-requestor",
					},
				},
				Data: map[string][]byte{
					"not-a-token": []byte("sometoken"),
				},
			},
		),
	)
})
