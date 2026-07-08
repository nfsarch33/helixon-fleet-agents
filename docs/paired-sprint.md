# Paired-sprint pattern (v14517)

**Status:** Adopted — applied to v14509+ (Sentrux audits) and v14510+ (MVP/Review pairs).
**Sprint:** v14517 Pair 7 Review.

## Motivation

Each Sentrux audit closes 2 sprints (an MVP sprint and a Review sprint)
that share a single goal. Without explicit pairing, two parallel sprints
can collide on the same files, the same `.sprint_lock`, or the same
release tag.

## Pattern

| Step | Owner | Artifact |
| --- | --- | --- |
| 1. Open pair-lock | MVP agent | `.sprint_lock` (pair_id=v145NN, phase=running) |
| 2. MVP deliverable | MVP agent | code + tests + handoff |
| 3. Close pair-lock | MVP agent | `.sprint_lock` (phase=closed) |
| 4. Open pair-lock (same pair_id) | Review agent | `.sprint_lock` (phase=running) |
| 5. Review deliverable | Review agent | audit + ADR + verifier update |
| 6. Close pair-lock + tag | Review agent | release tag sentrux-YYYY-MM-DD |
| 7. Append carry-forward | Review agent | `carry-forward-register-YYYY-MM-DD.ndjson` |

## Pair-lock semantics

- The lock file is a single JSON object with `pair_id`, `sprint_id`,
  `phase`, `started_at`, `pid`, `operator`, optional `personas`.
- Acquire is **idempotent for the same pair_id**; conflicts on a
  different pair_id return `ErrConflict`.
- The lock is **per-repo**; two repos may run the same sprint in
  parallel (e.g., `helixon-platform` v14517 + `helixon-fleet-agents` v14517).
- A new MVP must **never** start before the prior pair's lock is
  `closed`.

## Concurrent-pair prevention

- `tools/pair-lock acquire --pair v145NN` is the only sanctioned entry
  point.
- The Cursor `beforeSubmitPrompt` hook calls `pair-lock check` and
  refuses to redirect to a new cell if the lock belongs to a different
  pair (or another operator).
- A misbehaving agent that bypasses the hook is detectable: the
  `.sprint_lock` file will be stale; `git log` will show changes
  attributed to an absent operator.

## Pair sequence for Helixon

The 18-sprint roadmap from `v14504-v14521_closeout_plan_c0def683.plan.md`
defines 9 pairs:

| Pair | Sprints | Theme |
| --- | --- | --- |
| 1 | v14504 / v14505 | prerequisites + control-plane |
| 2 | v14506 / v14507 / v14508 | control-plane + retry + 1Password SDK |
| 3 | v14509 | Sentrux audit |
| 4 | v14510 / v14511 | choose-llm + hook + cost-obs + agentcage |
| 5 | v14512 / v14513 | observability sidecar + paging |
| 6 | v14514 / v14515 | MCP restore + token-saving |
| 7 | v14516 / v14517 | fleet-agents (this sprint) |
| 8 | v14518 / v14519 | repo hygiene |
| 9 | v14520 / v14521 | EvoSpine + final Sentrux |

## Operator responsibilities

- Acquire the lock at sprint start with `--operator <your-id>`.
- Close the lock at sprint end (don't delete the file; mark
  `phase=closed` so retros can audit the timeline).
- Carry-forward items must include the sprint id and severity.
- **Never force-push** while a pair-lock is open; force-push during a
  pair is a sentinel for botched pair transitions.