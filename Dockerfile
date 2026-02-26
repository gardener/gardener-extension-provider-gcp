############# builder
FROM --platform=$BUILDPLATFORM golang:1.26.0 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-gcp

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG EFFECTIVE_VERSION
ARG TARGETOS
ARG TARGETARCH

RUN make build GOOS=$TARGETOS GOARCH=$TARGETARCH EFFECTIVE_VERSION=$EFFECTIVE_VERSION BUILD_OUTPUT_FILE="/output/bin/"

############# base image
FROM gcr.io/distroless/static-debian12:nonroot AS base

############# gardener-extension-provider-gcp
FROM base AS gardener-extension-provider-gcp
WORKDIR /

COPY --from=builder /output/bin/gardener-extension-provider-gcp /gardener-extension-provider-gcp
ENTRYPOINT ["/gardener-extension-provider-gcp"]

############# gardener-extension-admission-gcp
FROM base AS gardener-extension-admission-gcp
WORKDIR /

COPY --from=builder /output/bin/gardener-extension-admission-gcp /gardener-extension-admission-gcp
ENTRYPOINT ["/gardener-extension-admission-gcp"]
