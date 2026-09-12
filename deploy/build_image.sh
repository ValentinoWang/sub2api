#!/usr/bin/env bash
# Build a committed source snapshot; promote this image instead of rebuilding it.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SOURCE_REF="${1:-HEAD}"
COMMIT="$(git -C "${REPO_ROOT}" rev-parse --verify "${SOURCE_REF}^{commit}")"
VERSION="$(git -C "${REPO_ROOT}" show "${COMMIT}:backend/cmd/server/VERSION" | tr -d '\r\n')"
if [[ ! "${VERSION}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Invalid technical version: ${VERSION}" >&2
    exit 1
fi
BUILD_DATE="$(git -C "${REPO_ROOT}" show -s --format=%cI "${COMMIT}")"
IMAGE="sub2api-local:${VERSION}-${COMMIT:0:12}"
BUILD_CONTEXT="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-build.XXXXXX")"
trap 'rm -rf "${BUILD_CONTEXT}"' EXIT

git -C "${REPO_ROOT}" archive "${COMMIT}" | tar -x -C "${BUILD_CONTEXT}"
printf 'Building %s from %s (committed files only)\n' "${IMAGE}" "${COMMIT}"
docker build --platform linux/amd64 --tag "${IMAGE}" \
    --build-arg "VERSION=${VERSION}" \
    --build-arg "COMMIT=${COMMIT}" \
    --build-arg "DATE=${BUILD_DATE}" \
    --label "org.opencontainers.image.source=https://github.com/ValentinoWang/sub2api" \
    --label "org.opencontainers.image.revision=${COMMIT}" \
    --label "org.opencontainers.image.version=${VERSION}" \
    --label "org.opencontainers.image.created=${BUILD_DATE}" \
    --file "${BUILD_CONTEXT}/Dockerfile" \
    "${BUILD_CONTEXT}"
docker image inspect "${IMAGE}" --format '{{.Id}} {{.Os}}/{{.Architecture}}'
