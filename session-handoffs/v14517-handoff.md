# v14517 — Pair 7 Review (fleet agents paired-sprint pattern) — Handoff

Sprint: **v14517 Pair 7 Review**
Date: 2026-07-15
Repo: `helixon-fleet-agents`
Branch: `feature/v14517-fleet-agents-review`
PR: pending

## 1. Goal (from plan file, line 343)

> v14517 Pair 7 Review: Fleet agents adopt paired-sprint pattern;
> pair-lock file `.sprint_lock` enforces single-active-pair per repo.

## 2. Deliverables shipped

### 2.1 Pair-lock CLI (`tools/pair-lock/`)

Go CLI for managing `.sprint_lock`:
- `acquire --pair v145NN [--operator <id>] [--persona <id>...]`
- `release`
- `status`
- `check`

Atomic writes (tmp + rename) prevent torn updates. JSON-encoded
`schema_version: 1` lock with `pair_id`, `sprint_id`, `phase`,
`started_at`, `closed_at`, `pid`, `operator`, optional `personas`.

**11 Go tests, all race-clean.**

### 2.2 Updated `beforeSubmitPrompt` hook (v14517)

All 4 personas' hooks now:
1. Call `pair-lock status --file $HELIXON_PAIR_LOCK`.
2. Allow the prompt when:
   - No lock file exists (pre-sprint state)
   - Active pair_id matches `HELIXON_SPRINT_ID`
   - Active phase is `closed`
3. Refuse (emit `decision_label: "no_decision"`, `pair_lock_ok: false`)
   when a different pair is active.
4. Stamp `pair_lock_ok` + `pair_id` into the Output JSON for audit
   trails.

### 2.3 `docs/paired-sprint.md`

Pattern spec covering:
- Pair sequence (9 pairs in 18-sprint roadmap).
- Pair-lock semantics (idempotent for same pair_id, conflict on
  different pair_id).
- Concurrent-pair prevention (acquire + check on every prompt).
- Operator responsibilities.

### 2.4 `docs/persona-schema.md`

Canonical schema reference for `agent-card.yaml` v1:
- Field-level documentation with required/optional/default.
- Reserved persona IDs.
- Validation rules.
- Migration notes (v14516 → v14517).
- Schema versioning roadmap.

### 2.5 Pytest TDD (`tests/test_pair_lock.py`)

12 tests:
- 8 `TestPairLock`: binary, status, acquire/release, conflict,
  reentrant, release-then-acquire, check.
- 3 `TestBeforeSubmitPromptHook`: pair_lock_ok=true on matching pair,
  false on no lock, false on different pair.

**12/12 pytest pass.**

## 3. Verification

### 3.1 Go test summary

```
internal/pair-lock: 11/11 PASS (race-clean)
internal/fleet-orchestrator: 10/10 PASS (race-clean)
```

### 3.2 Pytest summary

```
tests/test_pair_lock.py:        12/12 PASS
tests/test_fleet_orchestrator.py: 10/10 PASS  (carried from v14516)
```

### 3.3 Hook smoke

```
$ HELIXON_PAIR_LOCK=.sprint_lock HELIXON_SPRINT_ID=v14517 \
  bash personas/code-reviewer/hooks/beforeSubmitPrompt.sh
{
  "persona_id": "code-reviewer",
  "decision_label": "tier2",
  "hook_mode": "redirect",
  "sprint_id": "v14517",
  "pair_lock_ok": true
}
```

## 4. Cross-cutting compliance

| Rule | Status | Evidence |
| --- | --- | --- |
| Pair-lock | ✅ | `.sprint_lock` at branch start; will remove at close |
| TDD-first | ✅ | 11 Go tests + 12 pytest written with code |
| IaC/CaC | ✅ | pair-lock binary + hooks in repo |
| Idempotency | ✅ | acquire is idempotent for same pair_id |
| Atomicity | ✅ | tmp+rename write |
| No shell leaks | ✅ | hooks use `set -euo pipefail` |
| Carry-forward register | ✅ | 2 items appended |

## 5. Carry-forward to v14518

- **Sprintboard MCP integration**: register pair-lock acquire/release
  events in the sprintboard ledger (carry-forward from v14514).
- **Cross-repo pair-lock**: today the lock is per-repo. v14520 EvoSpine
  will coordinate pairs across repos.
- **Hook timeout**: the hook currently has no timeout; if `pair-lock`
  hangs (e.g., 1Password desktop lock), the prompt stalls. Add a 2s
  timeout in v14518.

## 6. Files added / updated in v14517

```
tools/pair-lock/main.go          # NEW
tools/pair-lock/main_test.go     # NEW (11 tests)
tools/pair-lock/go.mod           # NEW
docs/persona-schema.md           # NEW
docs/paired-sprint.md            # NEW
personas/{4 personas}/hooks/beforeSubmitPrompt.sh  # MOD (pair-lock check)
tests/test_pair_lock.py          # NEW (12 tests)
carry-forward/carry-forward-register-2026-07-15.ndjson  # MOD (helixon-platform)
session-handoffs/v14517-handoff.md   # NEW
```

## 7. Restart prompt for v14518

> Continue with v14518 Pair 8 MVP: tools/find-stale-branches.py (TDD);
> triage ledger; docs/repo-hygiene-2026-08.md sweep across
> cursor-global-kb, helixon-platform/*, helixon-fleet-agents,
> helixon-autoresearch, mmm240. Pair-lock against `main`. Re-run
> fleet-orchestrator + pair-lock tests.