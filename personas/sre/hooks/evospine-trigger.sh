#!/usr/bin/env bash
# EvoSpine cycle trigger hook (v14520)
#
# Runs from helixon-fleet-agents/personas/sre/hooks/ afterFileEdit.sh
# to spawn a new EvoSpine cycle after every N-th file edit.
#
# Throttle: only run one cycle per 30 minutes (using a PID lock).
set -euo pipefail

PERSONA_ID="${HELIXON_AGENT_PERSONA:-sre}"
LOCK_DIR="${HOME}/.cache/evospine"
LOCK_FILE="${LOCK_DIR}/last-cycle"

mkdir -p "$LOCK_DIR"

# Throttle: 30 minutes between cycles
if [ -f "$LOCK_FILE" ]; then
    last=$(cat "$LOCK_FILE" 2>/dev/null || echo 0)
    now=$(date +%s)
    if [ $((now - last)) -lt 1800 ]; then
        echo "evospine: throttled (last cycle $(($((now - last))/60))m ago)"
        exit 0
    fi
fi

# Find a Helixon repo to operate on (prefer helixon-platform)
target_repo=""
for r in /home/jaslian/Code/helixon-platform /home/jaslian/Code/helixon-fleet-agents /home/jaslian/Code/helixon-autoresearch; do
    if [ -d "$r/tools/evospine" ]; then
        target_repo="$r"
        break
    fi
done

if [ -z "$target_repo" ]; then
    echo "evospine: no helixon repo found"
    exit 0
fi

# Run the cycle (dry-run for now; v14521 will enable wet-run)
echo "evospine: spawning cycle in $target_repo"
( cd "$target_repo" && python3 tools/evospine/run-cycle.py \
    --repo nfsarch33/$(basename "$target_repo") \
    --cwd "$target_repo" \
    --dry-run ) || true

date +%s > "$LOCK_FILE"
echo "evospine: cycle dispatched"