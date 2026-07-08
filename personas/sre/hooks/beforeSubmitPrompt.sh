#!/usr/bin/env bash
# Helixon fleet agent — beforeSubmitPrompt hook (v14516)
#
# Reads Cursor's beforeSubmitPrompt JSON from stdin, stamps the agent
# persona into the prompt, and emits a redirect Output.
#
# Pattern lifted from helixon-platform/cmd/choose-llm (v14511) and the
# v14515 enforcement wrapper.
set -euo pipefail

PERSONA_ID="${HELIXON_AGENT_PERSONA:-code-reviewer}"
TIER="${HELIXON_AGENT_TIER:-2}"

# Read stdin payload
payload="$(cat)"

# Pick the cell via the existing choose-llm CLI (preferred) or fall
# back to a static redirect.
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

# Emit Output JSON (Cursor reads stdout)
cat <<EOF
{
  "persona_id": "$PERSONA_ID",
  "decision_label": "tier${TIER}",
  "cell_id": "$cell_id",
  "hook_mode": "redirect",
  "reason": "fleet-agent:${PERSONA_ID}",
  "sprint_id": "${HELIXON_SPRINT_ID:-unknown}"
}
EOF