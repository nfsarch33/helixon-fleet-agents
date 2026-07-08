#!/usr/bin/env bash
# Helixon fleet agent — afterFileEdit hook (v14516)
#
# Fires after every Cursor file edit. Appends to a per-persona NDJSON
# trail in `.fleet-trail/` so post-sprint retros can attribute changes.
set -euo pipefail

PERSONA_ID="${HELIXON_AGENT_PERSONA:-code-reviewer}"
mkdir -p .fleet-trail

# Read stdin payload (Cursor passes JSON with file path + diff stats)
payload="$(cat)"
file="$(echo "$payload" | jq -r '.file // "unknown"' 2>/dev/null || echo 'unknown')"
added="$(echo "$payload" | jq -r '.added // 0' 2>/dev/null || echo 0)"
removed="$(echo "$payload" | jq -r '.removed // 0' 2>/dev/null || echo 0)"

ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
printf '{"ts":"%s","persona_id":"%s","file":"%s","added":%s,"removed":%s}\n' \
    "$ts" "$PERSONA_ID" "$file" "$added" "$removed" \
    >> ".fleet-trail/${PERSONA_ID}.ndjson"