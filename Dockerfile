############# builder
FROM golang:1.13.4 AS builder

WORKDIR /go/src/github.com/gardener/gardener-extension-provider-gcp
COPY . .
RUN make install-requirements && make VERIFY=true all

############# gardener-extension-provider-gcp
FROM alpine:3.11.3 AS gardener-extension-provider-gcp

COPY charts /charts
COPY --from=builder /go/bin/gardener-extension-provider-gcp /gardener-extension-provider-gcp
ENTRYPOINT ["/gardener-extension-provider-gcp"]
