############# builder
FROM eu.gcr.io/gardener-project/3rd/golang:1.15.3 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-gcp
COPY . .
RUN make install

############# base image
FROM eu.gcr.io/gardener-project/3rd/alpine:3.12.1 AS base

############# gardener-extension-provider-gcp
FROM base AS gardener-extension-provider-gcp

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-gcp /gardener-extension-provider-gcp
ENTRYPOINT ["/gardener-extension-provider-gcp"]

############# gardener-extension-admission-gcp
FROM base AS gardener-extension-admission-gcp

COPY --from=builder /go/bin/gardener-extension-admission-gcp /gardener-extension-admission-gcp
ENTRYPOINT ["/gardener-extension-admission-gcp"]
