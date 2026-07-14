#!/bin/sh
set -eu

: "${MIHOMO_SUBSCRIPTION_URL:?MIHOMO_SUBSCRIPTION_URL is required}"

config_path=${MIHOMO_CONFIG_PATH:-/srv/sub2api/mihomo/config.yaml}
image=${MIHOMO_IMAGE:-metacubex/mihomo@sha256:e6acd921addecfd59a8e2d38203f88356d635b54de6c0673db0e015139989312}
container=${MIHOMO_CONTAINER:-sub2api-mihomo}
tmp_path=$(mktemp "${config_path}.tmp.XXXXXX")
trap 'rm -f "$tmp_path"' EXIT

curl -fsSL --max-time 30 "$MIHOMO_SUBSCRIPTION_URL" -o "$tmp_path"
sed -i 's/^allow-lan:.*/allow-lan: true/' "$tmp_path"

docker run --rm \
  -v "$tmp_path:/root/.config/mihomo/config.yaml:ro" \
  "$image" -t >/dev/null

if cmp -s "$tmp_path" "$config_path"; then
  exit 0
fi

install -m 600 "$tmp_path" "$config_path"
docker restart "$container" >/dev/null
