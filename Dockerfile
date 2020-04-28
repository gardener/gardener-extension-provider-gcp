############# builder
FROM golang:1.14.2 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-gcp
COPY . .
RUN make install

############# base image
FROM alpine:3.11.6 AS base

############# gardener-extension-provider-gcp
FROM base AS gardener-extension-provider-gcp

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-gcp /gardener-extension-provider-gcp
ENTRYPOINT ["/gardener-extension-provider-gcp"]

############# gardener-extension-validator-aws
FROM base AS gardener-extension-validator-gcp

COPY --from=builder /go/bin/gardener-extension-validator-gcp /gardener-extension-validator-gcp
ENTRYPOINT ["/gardener-extension-validator-gcp"]
