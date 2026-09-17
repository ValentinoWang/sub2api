#!/usr/bin/env bash
set -euo pipefail
export PATH="/tmp/sub2api-release-tools-20260915/bin:$PATH"
export HARNESS_ENGINEERING_HOME="/Users/vsiyo/Desktop/Opensource_Tool/Harness_Engineering"
cd /Users/vsiyo/.local/share/sub2api/worktrees/release-0.2.4.4-20260915
exec bash tools/quality/run_local_ci.sh "$1"
