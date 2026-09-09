# syntax=docker/dockerfile:1.7
# =============================================================================
# Sub2API Multi-Stage Dockerfile
# =============================================================================
# Stage 1: Build frontend
# Stage 2: Build Go backend with embedded frontend
# Stage 3: Final minimal image
# =============================================================================

ARG NODE_IMAGE=node:24-alpine
ARG GOLANG_IMAGE=golang:1.27.0-alpine
ARG ALPINE_IMAGE=alpine:3.21
ARG POSTGRES_IMAGE=postgres:18-alpine
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
ARG NPM_CONFIG_REGISTRY=

# -----------------------------------------------------------------------------
# Stage 1: Frontend Builder
# -----------------------------------------------------------------------------
# --platform=$BUILDPLATFORM: the frontend output is JS (arch-neutral), so build
# it on the native host arch instead of under QEMU emulation for the target.
FROM --platform=${BUILDPLATFORM} ${NODE_IMAGE} AS frontend-builder
ARG NPM_CONFIG_REGISTRY

WORKDIR /app/frontend

# Install pnpm (pinned to v9 to match CI and keep builds reproducible)
RUN corepack enable && corepack prepare pnpm@9 --activate

# Install dependencies first (better caching)
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN --mount=type=cache,id=sub2api-pnpm-store,target=/root/.local/share/pnpm/store \
    if [ -n "${NPM_CONFIG_REGISTRY}" ]; then pnpm config set registry "${NPM_CONFIG_REGISTRY}"; fi && \
    pnpm install --frozen-lockfile --prefer-offline

# Copy frontend source and build.
# LegalDocumentView.vue (admin-compliance gate) build-time imports
# ../../../../docs/legal/*.md?raw, so docs/legal/ must sit beside frontend/
# in the image (WORKDIR /app/frontend -> resolves to /app/docs/legal/*.md).
# Copy only that subtree to keep the build dependency minimal.
COPY frontend/ ./
COPY docs/legal/ /app/docs/legal/
RUN pnpm run build

# -----------------------------------------------------------------------------
# Stage 2: LDXP Toolkit Builder
# -----------------------------------------------------------------------------
# The LDXP administration runtime accepts only a local Linux/amd64 executable.
# Build it from repository source and emit its release assertion in the same
# stage; no downloaded or unchecked release artifact is used.
FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} AS ldxp-toolkit-builder

ARG VERSION=
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src/tools/ldxp-toolkit

COPY tools/ldxp-toolkit/go.mod ./
COPY tools/ldxp-toolkit/*.go ./
COPY backend/cmd/server/VERSION /tmp/sub2api-version

RUN set -eu; \
	TARGET_OS="${TARGETOS:-linux}"; \
	TARGET_ARCH="${TARGETARCH:-amd64}"; \
	if [ "${TARGET_OS}" != "linux" ] || [ "${TARGET_ARCH}" != "amd64" ]; then \
		echo "LDXP toolkit release asset is available only for linux/amd64 (requested ${TARGET_OS}/${TARGET_ARCH})" >&2; \
		exit 1; \
	fi; \
	TOOLKIT_VERSION="${VERSION:-$(tr -d '\\r\\n' < /tmp/sub2api-version)}"; \
	case "${TOOLKIT_VERSION}" in \
		""|dev|unpackaged|[!0-9A-Za-z]*|*[!0-9A-Za-z._+-]*) echo "LDXP toolkit release version is missing or invalid" >&2; exit 1;; \
	esac; \
	if [ "${#TOOLKIT_VERSION}" -gt 128 ]; then echo "LDXP toolkit release version is too long" >&2; exit 1; fi; \
	mkdir -p /out; \
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.toolkitVersion=${TOOLKIT_VERSION}" -o /out/ldxp-toolkit .; \
	test -f /out/ldxp-toolkit && test -x /out/ldxp-toolkit; \
	TOOLKIT_SHA256="$(sha256sum /out/ldxp-toolkit | awk '{print $1}')"; \
	printf '{"schema_version":1,"program":"ldxp-toolkit","version":"%s","os":"linux","arch":"amd64","sha256":"%s"}\n' "${TOOLKIT_VERSION}" "${TOOLKIT_SHA256}" > /out/ldxp-toolkit-release.json

# -----------------------------------------------------------------------------
# Stage 3: Backend Builder
# -----------------------------------------------------------------------------
# --platform=$BUILDPLATFORM: run the Go toolchain on the native host arch and
# cross-compile to the target arch below. The binary is CGO_ENABLED=0, so this
# is a clean pure-Go cross-compile — no QEMU emulation of go mod download / go
# build (emulated networking here was dropping module fetches with EOF).
FROM --platform=${BUILDPLATFORM} ${GOLANG_IMAGE} AS backend-builder

# Build arguments for version info (set by CI)
ARG VERSION=
ARG COMMIT=docker
ARG DATE
ARG GOPROXY
ARG GOSUMDB
# Populated by buildx from the --platform target (e.g. linux/amd64).
ARG TARGETOS
ARG TARGETARCH

ENV GOPROXY=${GOPROXY}
ENV GOSUMDB=${GOSUMDB}

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app/backend

# Copy go mod files first (better caching)
COPY backend/go.mod backend/go.sum ./
# Cache mount keeps the module cache across builds so a transient CDN blip on
# retry resumes instead of re-fetching every zip from scratch.
RUN --mount=type=cache,id=sub2api-gomod,target=/go/pkg/mod \
    go mod download

# Copy backend source first
COPY backend/ ./

# Copy frontend dist from previous stage (must be after backend copy to avoid being overwritten)
COPY --from=frontend-builder /app/backend/internal/web/dist ./internal/web/dist

# Build the binary (BuildType=release for CI builds, embed frontend)
# Version precedence: build arg VERSION > exact git tag > cmd/server/VERSION
RUN --mount=type=cache,id=sub2api-gomod,target=/go/pkg/mod \
    --mount=type=cache,id=sub2api-gobuild,target=/root/.cache/go-build \
    VERSION_VALUE="${VERSION}" && \
    if [ -z "${VERSION_VALUE}" ]; then VERSION_VALUE="$(./scripts/resolve-version.sh)"; fi && \
    DATE_VALUE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build \
    -tags embed \
    -ldflags="-s -w -X main.Version=${VERSION_VALUE} -X main.Commit=${COMMIT} -X main.Date=${DATE_VALUE} -X main.BuildType=release" \
    -trimpath \
    -o /app/sub2api \
    ./cmd/server

# -----------------------------------------------------------------------------
# Stage 4: PostgreSQL Client (version-matched with docker-compose)
# -----------------------------------------------------------------------------
FROM ${POSTGRES_IMAGE} AS pg-client

# -----------------------------------------------------------------------------
# Stage 5: Final Runtime Image
# -----------------------------------------------------------------------------
FROM ${ALPINE_IMAGE}

# Labels
LABEL maintainer="Wei-Shaw <github.com/Wei-Shaw>"
LABEL description="Sub2API - AI API Gateway Platform"
LABEL org.opencontainers.image.source="https://github.com/Wei-Shaw/sub2api"

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    su-exec \
    libpq \
    zstd-libs \
    lz4-libs \
    krb5-libs \
    libldap \
    libedit \
    && rm -rf /var/cache/apk/*

# Copy pg_dump and psql from the same postgres image used in docker-compose
# This ensures version consistency between backup tools and the database server
COPY --from=pg-client /usr/local/bin/pg_dump /usr/local/bin/pg_dump
COPY --from=pg-client /usr/local/bin/psql /usr/local/bin/psql
COPY --from=pg-client /usr/local/lib/libpq.so.5* /usr/local/lib/

# Create non-root user
RUN addgroup -g 1000 sub2api && \
    adduser -u 1000 -G sub2api -s /bin/sh -D sub2api

# Set working directory
WORKDIR /app

# Copy binary/resources with ownership to avoid extra full-layer chown copy
COPY --from=backend-builder --chown=sub2api:sub2api /app/sub2api /app/sub2api
COPY --from=backend-builder --chown=sub2api:sub2api /app/backend/resources /app/resources
# The release manifest is a required assertion consumed by the Go runtime.
# Keep the asset outside the writable data directory; administrators may only
# copy it to the fixed private target after checksum verification.
COPY --from=ldxp-toolkit-builder --chown=root:root /out/ldxp-toolkit /app/ldxp-toolkit-assets/ldxp-toolkit
COPY --from=ldxp-toolkit-builder --chown=root:root /out/ldxp-toolkit-release.json /app/ldxp-toolkit-assets/ldxp-toolkit-release.json
RUN chmod 0555 /app/ldxp-toolkit-assets/ldxp-toolkit && \
	chmod 0444 /app/ldxp-toolkit-assets/ldxp-toolkit-release.json

ENV LIANDONG_TOOLKIT_DATA_DIR=/app/data \
	LIANDONG_TOOLKIT_ASSET_PATH=/app/ldxp-toolkit-assets/ldxp-toolkit \
	LIANDONG_TOOLKIT_ASSET_MANIFEST_PATH=/app/ldxp-toolkit-assets/ldxp-toolkit-release.json

# Create data directory
RUN mkdir -p /app/data && chown sub2api:sub2api /app/data

# Copy entrypoint script (fixes volume permissions then drops to sub2api)
COPY deploy/docker-entrypoint.sh /app/docker-entrypoint.sh
RUN chmod +x /app/docker-entrypoint.sh

# Expose port (can be overridden by SERVER_PORT env var)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -q -T 5 -O /dev/null http://localhost:${SERVER_PORT:-8080}/health || exit 1

# Run the application (entrypoint fixes /app/data ownership then execs as sub2api)
ENTRYPOINT ["/app/docker-entrypoint.sh"]
CMD ["/app/sub2api"]
