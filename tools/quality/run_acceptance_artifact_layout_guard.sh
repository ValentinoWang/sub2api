#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
manager_relative="Core/skills/design-acceptance-contract/scripts/manage_acceptance_artifacts.py"

if [[ -n "${HARNESS_ENGINEERING_HOME:-}" ]]; then
  harness_source="$HARNESS_ENGINEERING_HOME"
elif [[ -f "$project_root/.harness/upstream/$manager_relative" ]]; then
  harness_source="$project_root/.harness/upstream"
else
  harness_source="$project_root/../Harness_Engineering"
fi

manager="$harness_source/$manager_relative"
if [[ ! -f "$manager" ]]; then
  echo "Acceptance manager is missing: $manager" >&2
  echo "Set HARNESS_ENGINEERING_HOME to a Harness_Engineering checkout." >&2
  exit 1
fi

exec python3 "$manager" check-project --project-root "$project_root"
