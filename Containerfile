# ==============================================================================
# mini-acm — LAB / TEST / DEV ONLY
# Orchestrates ACM hub SNO + compact managed clusters on lab infra providers.
# ==============================================================================

FROM registry.access.redhat.com/ubi9/go-toolset:latest AS build
WORKDIR /build
USER 0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o /build/mini-acm ./cmd/mini-acm

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

LABEL org.opencontainers.image.title="mini-acm" \
      org.opencontainers.image.description="Lab orchestrator for ACM hub + compact clusters - LAB/TEST/DEV ONLY" \
      org.opencontainers.image.source="https://github.com/dasmlab/mini-acm" \
      io.dasmlab.warning="LAB_TEST_DEV_ONLY"

ARG OC_CLI_URL=https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable/openshift-client-linux.tar.gz

RUN microdnf install -y tar gzip ca-certificates curl && \
    curl -fsSL "${OC_CLI_URL}" -o /tmp/oc.tar.gz && \
    tar -xzf /tmp/oc.tar.gz -C /usr/local/bin oc && \
    rm -f /tmp/oc.tar.gz && \
    microdnf clean all

COPY --from=build /build/mini-acm /usr/local/bin/mini-acm
COPY config /opt/mini-acm/config
COPY profiles /opt/mini-acm/profiles
COPY manifests /opt/mini-acm/manifests

RUN useradd -u 1001 -m -s /sbin/nologin miniacm && mkdir -p /data && chown 1001:1001 /data
USER 1001
WORKDIR /data
VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/mini-acm"]
CMD ["--help"]
