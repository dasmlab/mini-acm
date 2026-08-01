# ==============================================================================
# mini-mock — LAB / TEST / DEV ONLY
# Multi-stage: Vue UI + Go CLI, then UBI minimal runtime with oc.
# ==============================================================================

FROM registry.access.redhat.com/ubi9/nodejs-20:latest AS web
WORKDIR /web
USER 0
COPY web/package.json web/package-lock.json ./
RUN npm ci || npm install
COPY web/ ./
RUN npm run build

FROM registry.access.redhat.com/ubi9/go-toolset:latest AS build
WORKDIR /build
USER 0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /web/dist/ ./cmd/mini-mock/static/

ARG VERSION=dev
ARG BUILD_VERSION
RUN VER="${BUILD_VERSION:-${VERSION}}"; \
    CGO_ENABLED=0 go build -ldflags "-X main.version=${VER}" -o /build/mini-mock ./cmd/mini-mock

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest

LABEL org.opencontainers.image.title="mini-mock" \
      org.opencontainers.image.description="Lab orchestrator for ACM hub + compact clusters - LAB/TEST/DEV ONLY" \
      org.opencontainers.image.source="https://github.com/dasmlab/mini-mock" \
      io.dasmlab.warning="LAB_TEST_DEV_ONLY"

ARG OC_CLI_URL=https://mirror.openshift.com/pub/openshift-v4/clients/ocp/stable/openshift-client-linux.tar.gz

RUN microdnf install -y tar gzip ca-certificates && \
    curl -fsSL "${OC_CLI_URL}" -o /tmp/oc.tar.gz && \
    tar -xzf /tmp/oc.tar.gz -C /usr/local/bin oc && \
    rm -f /tmp/oc.tar.gz && \
    microdnf clean all && \
    useradd -u 65532 -r -s /sbin/nologin minimock && \
    mkdir -p /data && chown 65532:65532 /data

COPY --from=build /build/mini-mock /usr/local/bin/mini-mock
COPY config /opt/mini-mock/config
COPY profiles /opt/mini-mock/profiles
COPY manifests /opt/mini-mock/manifests

USER 65532
WORKDIR /data
VOLUME ["/data"]
EXPOSE 8080

ENV DATA_DIR=/data
ENTRYPOINT ["/usr/local/bin/mini-mock"]
CMD ["serve", "--listen", ":8080", "--data-dir", "/data"]
