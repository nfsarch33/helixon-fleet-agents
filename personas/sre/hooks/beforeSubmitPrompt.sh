#!/usr/bin/env bash
# Helixon fleet agent — beforeSubmitPrompt hook (v14517)
#
# Reads Cursor's beforeSubmitPrompt JSON from stdin, checks the
# pair-lock, picks the right LLM cell, and emits a redirect Output.
#
# Pattern lifted from helixon-platform/cmd/choose-llm (v14511) and the
# v14515 enforcement wrapper.
#
# v14517 changes:
#   - calls `pair-lock check` before any decision; refuses with
#     no_decision if a different pair is active.
#   - emits `pair_id` + `pair_lock_ok` in the Output for audit trails.
set -euo pipefail

PERSONA_ID="${HELIXON_AGENT_PERSONA:-code-reviewer}"
TIER="${HELIXON_AGENT_TIER:-2}"
PAIR_LOCK_FILE="${HELIXON_PAIR_LOCK:-.sprint_lock}"
PAIR_ID="${HELIXON_SPRINT_ID:-unknown}"

# 1. Pair-lock check (v14517)
pair_lock_ok="false"
pair_id="$PAIR_ID"
if command -v pair-lock >/dev/null 2>&1; then
    if pair-lock check --file "$PAIR_LOCK_FILE" >/dev/null 2>&1; then
        pair_lock_ok="true"
    fi
    # Pull the active pair_id for the Output
    active="$(pair-lock status --file "$PAIR_LOCK_FILE" 2>/dev/null | grep -oE '"pair_id": *"[^"]+"' | head -1 | sed -E 's/.*"([^"]+)"/\1/')"
    if [ -n "$active" ]; then
        pair_id="$active"
    fi
fi

if [ "$pair_lock_ok" != "true" ]; then
    cat <<EOF
{
  "persona_id": "$PERSONA_ID",
  "decision_label": "no_decision",
  "hook_mode": "abstain",
  "reason": "pair-lock: another pair is active (or no lock)",
  "sprint_id": "$pair_id",
  "pair_lock_ok": false
}
EOF
    exit 0
fi

# 2. Read stdin payload
payload="$(cat)"

# 3. Pick the cell via the existing choose-llm CLI (preferred) or
#    fall back to a static redirect.
cell_id=""
if command -v choose-llm >/dev/null 2>&1; then
    cell_id="$(echo "$payload" | choose-llm hook decide \
                 --persona "$PERSONA_ID" \
                 --tier "$TIER" 2>/dev/null \
                 | jq -r '.cell_id // empty')"
fi

if [[ -z "$cell_id" ]]; then
    cell_id="local-tier${TIER}"
fi

# 4. Emit Output JSON (Cursor reads stdout)
cat <<EOF
{
  "persona_id": "$PERSONA_ID",
  "decision_label": "tier${TIER}",
  "cell_id": "$cell_id",
  "hook_mode": "redirect",
  "reason": "fleet-agent:${PERSONA_ID}",
  "sprint_id": "$pair_id",
  "pair_lock_ok": true
}
EOF