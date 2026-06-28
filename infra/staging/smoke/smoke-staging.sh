#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${1:-${STAGING_API_BASE_URL:-}}"
if [ -z "${BASE_URL}" ]; then
  echo "Usage: STAGING_API_BASE_URL=https://api.staging.atlas.example.com $0"
  exit 1
fi

health_url="${BASE_URL%/}/api/v1/health"
exercises_url="${BASE_URL%/}/api/v1/exercises"

echo "Running staging smoke checks against ${BASE_URL%/}" >&2
curl --fail --show-error --silent "${health_url}" | tee /tmp/atlas-staging-health.json >/dev/null
jq -e '.status == "ok"' /tmp/atlas-staging-health.json >/dev/null

# Public endpoint check to validate router and DB read path.
curl --fail --show-error --silent "${exercises_url}" >/tmp/atlas-staging-exercises.json

echo "Staging smoke checks passed" >&2
