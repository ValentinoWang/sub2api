#!/bin/sh
set -eu
: "${COST_TRAFFIC_INTERFACE:?explicit interface required}"
: "${COST_TRAFFIC_SOURCE:?explicit source required}"
case "$COST_TRAFFIC_INTERFACE" in ''|*[!a-zA-Z0-9_.:-]*) exit 2;; esac
vnstatd --initdb --noadd
if ! vnstat --json -i "$COST_TRAFFIC_INTERFACE" >/dev/null 2>&1; then
  vnstat --add -i "$COST_TRAFFIC_INTERFACE"
fi
vnstatd --nodaemon --noadd --sync &
VNSTAT_PROCESS=$!
/opt/cost-collect --source "$COST_TRAFFIC_SOURCE" --interface "$COST_TRAFFIC_INTERFACE" --adapter vnstat --scope container_interface --interval 10s &
COLLECTOR_PROCESS=$!
finish() {
  kill "$COLLECTOR_PROCESS" "$VNSTAT_PROCESS" 2>/dev/null || true
  wait "$COLLECTOR_PROCESS" "$VNSTAT_PROCESS" 2>/dev/null || true
}
trap finish EXIT
trap 'exit 0' TERM INT
wait -n "$VNSTAT_PROCESS" "$COLLECTOR_PROCESS"
