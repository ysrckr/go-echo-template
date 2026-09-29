# syntax=docker/dockerfile:1.7

# Multi-platform build. The builder always runs natively on the host
# ($BUILDPLATFORM) and cross-compiles to the requested target, so building for
# linux/arm64 on an amd64 host (or vice versa) never pays the QEMU penalty.
#
#   docker buildx build --platform linux/amd64,linux/arm64 -t app:latest --push .

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

# Provided automatically by buildx.
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src

# Dependencies are their own layer so code edits do not re-download the module graph.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# CGO off + netgo/osusergo gives a fully static binary that runs on scratch.
# -s -w strips the symbol table and DWARF; -trimpath removes local build paths.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
      -trimpath \
      -tags netgo,osusergo \
      -ldflags="-s -w -extldflags '-static' -X main.version=${VERSION}" \
      -o /out/app ./cmd/api


# Collects the few files scratch cannot provide on its own.
FROM alpine:3.24 AS certs
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -g '' -u 65532 nonroot


FROM scratch AS final

# TLS roots are required: the Infisical SDK talks to https://app.infisical.com,
# and Postgres over sslmode=require/verify-full needs them too.
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=certs /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=certs /etc/passwd /etc/passwd

COPY --from=builder /out/app /app

USER 65532:65532
EXPOSE 8080

ENV HTTP_HOST=0.0.0.0 \
    HTTP_PORT=8080 \
    LOG_FORMAT=json \
    TZ=UTC

# The binary probes itself; a scratch image has no shell or curl.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD ["/app", "-healthcheck"]

ENTRYPOINT ["/app"]
