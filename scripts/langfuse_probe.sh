#!/usr/bin/env bash
set -euo pipefail

# Langfuse probe: checks health, lists recent traces, and summarizes observations.
# Usage:
#   source .env.tracing   # or export LANGFUSE_* vars
#   scripts/langfuse_probe.sh [limit]

LIMIT=${1:-5}

if [[ -z "${LANGFUSE_HOST:-}" || -z "${LANGFUSE_PUBLIC_KEY:-}" || -z "${LANGFUSE_SECRET_KEY:-}" ]]; then
  echo "Error: LANGFUSE_HOST, LANGFUSE_PUBLIC_KEY, and LANGFUSE_SECRET_KEY must be set (e.g., source .env.tracing)" >&2
  exit 1
fi

AUTH="${LANGFUSE_PUBLIC_KEY}:${LANGFUSE_SECRET_KEY}"

echo "==> Health: ${LANGFUSE_HOST}/api/public/health"
curl -sS -u "$AUTH" "${LANGFUSE_HOST%/}/api/public/health" || true
echo -e "\n"

echo "==> Recent traces (limit=${LIMIT})"
TRACES_JSON=$(curl -sS -u "$AUTH" "${LANGFUSE_HOST%/}/api/public/traces?limit=${LIMIT}" || true)
if [[ -z "$TRACES_JSON" || "$TRACES_JSON" == "" ]]; then
  echo "No traces returned or endpoint unavailable. If running Langfuse locally, ensure it is up (port 3001 by default)." >&2
  exit 0
fi

# Try to use jq if available for nicer output
if command -v jq >/dev/null 2>&1; then
  echo "$TRACES_JSON" | jq -r '.data[] | "- traceId: \(.id) name: \(.name // "") createdAt: \(.createdAt)"'
else
  echo "$TRACES_JSON" | sed -E 's/\s+/ /g' | head -c 800
  echo
fi

# Extract first trace id and summarize observations
if command -v jq >/dev/null 2>&1; then
  TRACE_ID=$(echo "$TRACES_JSON" | jq -r '.data[0].id // empty')
else
  TRACE_ID=""
fi

if [[ -n "$TRACE_ID" ]]; then
  echo "\n==> Observations for trace: $TRACE_ID"
  OBS_JSON=$(curl -sS -u "$AUTH" "${LANGFUSE_HOST%/}/api/public/observations?traceId=${TRACE_ID}" || true)
  if command -v jq >/dev/null 2>&1; then
    echo "$OBS_JSON" | jq -r '
      .data[] |
      "- name: \(.name) type: \(.type // ( .attributes."langfuse.observation.type" // "span"))\n  id: \(.id) start: \(.startTime)\n  attrs: game.operation_type=\(.attributes."game.operation_type" // "") session.id=\(.attributes."session.id" // .attributes."langfuse.session.id" // "") tool_name=\(.attributes.tool_name // "")"'
  else
    echo "$OBS_JSON" | sed -E 's/\s+/ /g' | head -c 1200; echo
  fi
fi

echo "\nDone."

