#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

if ! git diff --quiet || ! git diff --cached --quiet; then
  printf 'tracked worktree changes must be committed before building\n' >&2
  exit 1
fi

commit="$(git rev-parse --verify HEAD)"
short_commit="${commit:0:12}"
platform="${PLATFORM:-linux/amd64}"
image_repository="${IMAGE_REPOSITORY:-sub2api-local}"
image="${image_repository}:${short_commit}"
artifact_dir="${ARTIFACT_DIR:-$repo_root/.artifacts/sub2api-deployment/$short_commit}"
archive="$artifact_dir/sub2api-${short_commit}-linux-amd64.tar.gz"
checksum="$archive.sha256"
manifest="$artifact_dir/candidate.env"
go_image="${GO_TEST_IMAGE:-golang:1.26.5-alpine}"
go_test_platform="${GO_TEST_PLATFORM:-}"
go_test_goproxy="${GO_TEST_GOPROXY:-https://goproxy.cn,direct}"
go_test_gosumdb="${GO_TEST_GOSUMDB:-sum.golang.google.cn}"
test_packages="${GO_TEST_PACKAGES:-./internal/config ./internal/repository ./internal/service ./internal/handler ./cmd/server}"
build_parallelism="${LOCAL_BUILD_PARALLELISM:-2}"
go_mod_cache="${GO_MOD_CACHE_VOLUME:-sub2api-go-mod}"
go_build_cache="${GO_BUILD_CACHE_VOLUME:-sub2api-go-build}"
production_base_registry="${PRODUCTION_BASE_IMAGE_REGISTRY:-docker.m.daocloud.io/library}"
node_image="${NODE_IMAGE:-$production_base_registry/node:24-alpine}"
golang_image="${GOLANG_IMAGE:-$production_base_registry/golang:1.26.5-alpine}"
postgres_image="${POSTGRES_IMAGE:-$production_base_registry/postgres:18-alpine}"
alpine_image="${ALPINE_IMAGE:-$production_base_registry/alpine:3.21}"

host_goos="$(docker info --format '{{.OSType}}')"
host_arch="$(docker info --format '{{.Architecture}}')"
case "$host_arch" in
  aarch64 | arm64) host_arch="arm64" ;;
  x86_64 | amd64) host_arch="amd64" ;;
esac
if [[ -z "$go_test_platform" ]]; then
  go_test_platform="${host_goos}/${host_arch}"
fi

docker_pull_retry() {
  local pull_platform="$1"
  local image_name="$2"
  local attempt
  for attempt in 1 2 3; do
    if docker pull --platform "$pull_platform" "$image_name"; then
      return 0
    fi
    printf 'pull failed for %s on %s, retry %s/3\n' "$image_name" "$pull_platform" "$attempt" >&2
    sleep 2
  done
  return 1
}

if [[ "$platform" != "linux/amd64" ]]; then
  printf 'production candidate platform must be linux/amd64, got %s\n' "$platform" >&2
  exit 1
fi

mkdir -p "$artifact_dir"
dockerfile_for_build="$repo_root/Dockerfile"
tmp_dockerfile=""

cleanup_tmp_dockerfile() {
  if [[ -n "$tmp_dockerfile" && -f "$tmp_dockerfile" ]]; then
    rm -f "$tmp_dockerfile"
  fi
}
trap cleanup_tmp_dockerfile EXIT

printf 'running Go verification for %s\n' "$commit"
docker_pull_retry "$go_test_platform" "$go_image"
docker run --rm \
  --platform "$go_test_platform" \
  -e "GOPROXY=$go_test_goproxy" \
  -e "GOSUMDB=$go_test_gosumdb" \
  -v "$repo_root/backend:/src" \
  -v "$go_mod_cache:/go/pkg/mod" \
  -v "$go_build_cache:/root/.cache/go-build" \
  -w /src \
  "$go_image" \
  sh -lc "/usr/local/go/bin/go mod download && /usr/local/go/bin/go test $test_packages"

version="$(tr -d '[:space:]' < backend/cmd/server/VERSION)"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

printf 'warming production base images for %s\n' "$platform"
docker_pull_retry "$platform" "$node_image"
docker_pull_retry "$platform" "$golang_image"
docker_pull_retry "$platform" "$postgres_image"
docker_pull_retry "$platform" "$alpine_image"

if head -n 1 "$repo_root/Dockerfile" | grep -q '^# syntax='; then
  tmp_dockerfile="$artifact_dir/Dockerfile.no-syntax"
  tail -n +2 "$repo_root/Dockerfile" > "$tmp_dockerfile"
  dockerfile_for_build="$tmp_dockerfile"
fi

printf 'building %s for %s\n' "$image" "$platform"
docker buildx build \
  -f "$dockerfile_for_build" \
  --platform "$platform" \
  --load \
  --build-arg "VERSION=$version" \
  --build-arg "COMMIT=$commit" \
  --build-arg "DATE=$build_date" \
  --build-arg "GO_BUILD_PARALLELISM=$build_parallelism" \
  --build-arg "NODE_IMAGE=$node_image" \
  --build-arg "GOLANG_IMAGE=$golang_image" \
  --build-arg "POSTGRES_IMAGE=$postgres_image" \
  --build-arg "ALPINE_IMAGE=$alpine_image" \
  -t "$image" \
  .

architecture="$(docker image inspect "$image" --format '{{.Architecture}}')"
if [[ "$architecture" != "amd64" ]]; then
  printf 'candidate architecture mismatch: expected amd64, got %s\n' "$architecture" >&2
  exit 1
fi

docker run --rm --platform "$platform" --entrypoint /app/sub2api "$image" --version

tmp_archive="$archive.tmp"
trap 'rm -f "$tmp_archive"; cleanup_tmp_dockerfile' EXIT
docker save "$image" | gzip -1 > "$tmp_archive"
mv "$tmp_archive" "$archive"
trap cleanup_tmp_dockerfile EXIT

(
  cd "$artifact_dir"
  shasum -a 256 "$(basename "$archive")" > "$(basename "$checksum")"
)
archive_sha256="$(awk '{print $1}' "$checksum")"
image_id="$(docker image inspect "$image" --format '{{.Id}}')"

{
  printf 'commit=%s\n' "$commit"
  printf 'image=%s\n' "$image"
  printf 'image_id=%s\n' "$image_id"
  printf 'platform=%s\n' "$platform"
  printf 'archive=%s\n' "$archive"
  printf 'archive_sha256=%s\n' "$archive_sha256"
  printf 'built_at=%s\n' "$build_date"
} > "$manifest"

printf 'candidate_manifest=%s\n' "$manifest"
printf 'candidate_image=%s\n' "$image"
printf 'candidate_archive_sha256=%s\n' "$archive_sha256"
