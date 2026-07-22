#!/usr/bin/env bash
set -euo pipefail

target=""
manifest=""
remote_root="/srv/sub2api/releases"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      target="$2"
      shift 2
      ;;
    --manifest)
      manifest="$2"
      shift 2
      ;;
    --remote-root)
      remote_root="$2"
      shift 2
      ;;
    *)
      printf 'unknown argument: %s\n' "$1" >&2
      exit 2
      ;;
  esac
done

if [[ -z "$target" || -z "$manifest" ]]; then
  printf 'usage: %s --target user@host --manifest /path/to/candidate.env [--remote-root /srv/sub2api/releases]\n' "$0" >&2
  exit 2
fi
if [[ "$target" == -* || ! "$remote_root" =~ ^/[A-Za-z0-9._/-]+$ ]]; then
  printf 'invalid SSH target or remote root\n' >&2
  exit 2
fi
if [[ ! -f "$manifest" ]]; then
  printf 'manifest not found: %s\n' "$manifest" >&2
  exit 1
fi

read_manifest() {
  local key="$1"
  awk -F= -v key="$key" '$1 == key {sub(/^[^=]*=/, ""); print; exit}' "$manifest"
}

commit="$(read_manifest commit)"
image="$(read_manifest image)"
archive="$(read_manifest archive)"
expected_sha="$(read_manifest archive_sha256)"
checksum="$archive.sha256"

if [[ ! "$commit" =~ ^[0-9a-f]{40}$ || ! "$image" =~ ^[A-Za-z0-9._/:@-]+$ || ! -f "$archive" || ! -f "$checksum" ]]; then
  printf 'candidate manifest is incomplete or invalid\n' >&2
  exit 1
fi

actual_sha="$(shasum -a 256 "$archive" | awk '{print $1}')"
if [[ "$actual_sha" != "$expected_sha" ]]; then
  printf 'local archive checksum mismatch\n' >&2
  exit 1
fi

ssh_options=(-o ConnectTimeout=10)
remote_dir="$remote_root/${commit:0:12}"

printf 'checking current production service before upload\n'
ssh "${ssh_options[@]}" "$target" \
  'docker inspect sub2api >/dev/null && curl -fsS --max-time 8 http://127.0.0.1:8080/health >/dev/null'

ssh "${ssh_options[@]}" "$target" "mkdir -p '$remote_dir'"
scp "${ssh_options[@]}" "$archive" "$checksum" "$manifest" "$target:$remote_dir/"

archive_name="$(basename "$archive")"
checksum_name="$(basename "$checksum")"
printf 'loading candidate image without recreating the running container\n'
ssh "${ssh_options[@]}" "$target" "
  set -eu
  cd '$remote_dir'
  sha256sum -c '$checksum_name'
  if command -v ionice >/dev/null 2>&1; then
    ionice -c 3 nice -n 19 docker load -i '$archive_name'
  else
    nice -n 19 docker load -i '$archive_name'
  fi
  docker image inspect '$image' --format 'loaded_image={{.Id}} architecture={{.Architecture}}'
  curl -fsS --max-time 8 http://127.0.0.1:8080/health >/dev/null
  docker inspect sub2api --format 'running_image={{.Image}} status={{.State.Status}}{{if .State.Health}} health={{.State.Health.Status}}{{end}}'
"

printf 'candidate_uploaded=%s\n' "$image"
printf 'cutover_performed=false\n'
