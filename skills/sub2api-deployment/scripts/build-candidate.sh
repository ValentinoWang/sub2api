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
test_packages="${GO_TEST_PACKAGES:-./internal/config ./internal/repository ./internal/service ./internal/handler ./cmd/server}"
build_parallelism="${LOCAL_BUILD_PARALLELISM:-2}"

if [[ "$platform" != "linux/amd64" ]]; then
  printf 'production candidate platform must be linux/amd64, got %s\n' "$platform" >&2
  exit 1
fi

mkdir -p "$artifact_dir"

printf 'running Go verification for %s\n' "$commit"
docker run --rm \
  -v "$repo_root/backend:/src" \
  -v sub2api-go-mod:/go/pkg/mod \
  -v sub2api-go-build:/root/.cache/go-build \
  -w /src \
  "$go_image" \
  sh -lc "/usr/local/go/bin/go mod download && /usr/local/go/bin/go test $test_packages"

version="$(tr -d '[:space:]' < backend/cmd/server/VERSION)"
build_date="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

printf 'building %s for %s\n' "$image" "$platform"
docker buildx build \
  --platform "$platform" \
  --load \
  --build-arg "VERSION=$version" \
  --build-arg "COMMIT=$commit" \
  --build-arg "DATE=$build_date" \
  --build-arg "GO_BUILD_PARALLELISM=$build_parallelism" \
  -t "$image" \
  .

architecture="$(docker image inspect "$image" --format '{{.Architecture}}')"
if [[ "$architecture" != "amd64" ]]; then
  printf 'candidate architecture mismatch: expected amd64, got %s\n' "$architecture" >&2
  exit 1
fi

docker run --rm --platform "$platform" --entrypoint /app/sub2api "$image" --version

tmp_archive="$archive.tmp"
trap 'rm -f "$tmp_archive"' EXIT
docker save "$image" | gzip -1 > "$tmp_archive"
mv "$tmp_archive" "$archive"
trap - EXIT

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
