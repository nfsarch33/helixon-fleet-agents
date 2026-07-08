# v14516 — Pair 7 MVP (helixon-fleet-agents) — Handoff

Sprint: **v14516 Pair 7 MVP**
Date: 2026-07-15
Repo: `helixon-fleet-agents` (newly created)
Branch: `main` (initial commit)

## 1. Goal (from plan file, line 342)

> v14516 Pair 7 MVP: `helixon-fleet-agents` repo skeleton; 4 personas
> (`code-reviewer`, `ops-engineer`, `sre`, `release-manager`); each
> ships `agent-card.yaml` + skill bundle + hook config.

## 2. Deliverables shipped

### 2.1 New GitHub repo: `helixon-fleet-agents`

Created via `gh repo create helixon-fleet-agents --public`. Pushed from
local clone at `~/Code/helixon-fleet-agents/`.

### 2.2 4 personas with `agent-card.yaml` (schema-version 1)

| Persona | Tier | Budget/day | Owner |
| --- | --- | --- | --- |
| `code-reviewer` | 2 | $5.00 | helixon-platform-team |
| `ops-engineer` | 1 | $4.00 | helixon-platform-team |
| `sre` | 2 | $6.00 | helixon-platform-team |
| `release-manager` | 3 | $3.00 | helixon-platform-team |

Each card has: `schema_version`, `persona` (id, display_name, owner,
home_repo, reviews_repos, default_tier, budget_usd_per_day,
tier_routing), `skills` (bundle path + required skill list), `hooks`
(beforeSubmitPrompt + afterFileEdit paths), `pair_lock`.

### 2.3 Skill bundles (4 × `SKILL.md`)

Each persona ships a `skills/<id>/SKILL.md` with:
- Persona activation rules
- Required skill list (cross-references existing global skills)
- Workflow (step-by-step)
- Coordination rules with peer personas
- Tone guidance
- Failure modes + escalation paths

### 2.4 Hooks (4 × shell scripts)

- `hooks/beforeSubmitPrompt.sh` — reads Cursor's stdin JSON, invokes
  `choose-llm hook decide` (v14511) with the persona's tier, emits a
  redirect Output on stdout. Falls back to a static `local-tierN`
  redirect if `choose-llm` is unavailable.
- `hooks/afterFileEdit.sh` — appends a `{ts, persona_id, file, added,
  removed}` record to `.fleet-trail/<persona>.ndjson` for retro
  attribution.

Both hooks are shared across all 4 personas (same shell code, env
var `HELIXON_AGENT_PERSONA` selects the persona).

### 2.5 Go orchestrator CLI: `tools/fleet-orchestrator/`

Cobra-style CLI (manual flag parsing) with subcommands:
- `list` — table of `PERSONA_ID | DISPLAY_NAME | TIER | BUDGET | OWNER`
- `validate --dir <path>` — checks every `agent-card.yaml` under dir
- `card <id>` — prints the parsed YAML for one persona
- `schema` — prints the agent-card schema (schema_version 1)

**10 Go tests, all race-clean.**

### 2.6 Pytest TDD: `tests/test_fleet_orchestrator.py`

10 tests:
- binary exists, builds if missing
- list returns 4 personas, validate passes 4
- card roundtrips, unknown id exits non-zero
- schema subcommand emits version
- IDs unique, all cards have tier + budget + hooks + SKILL.md

**10/10 pytest pass.**

## 3. Verification

### 3.1 Orchestrator CLI smoke

```
$ ./fleet-orchestrator list --dir ../../personas
PERSONA_ID           DISPLAY_NAME                   TIER  BUDGET   OWNER
--------------------------------------------------------------------------------
code-reviewer        Helixon Code Reviewer          2     $5.00    helixon-platform-team
ops-engineer         Helixon Ops Engineer           1     $4.00    helixon-platform-team
release-manager      Helixon Release Manager        3     $3.00    helixon-platform-team
sre                  Helixon SRE                    2     $6.00    helixon-platform-team
```

### 3.2 Go tests + pytest

```
internal/orchestrator: 10/10 PASS (race-clean)
tests/test_fleet_orchestrator.py: 10/10 PASS
```

## 4. Cross-cutting compliance

| Rule | Status | Evidence |
| --- | --- | --- |
| Pair-lock | ✅ | `.sprint_lock` at branch start; will remove at close |
| Vendor verification | ✅ | No new vendor; uses stdlib + `gopkg.in/yaml.v3` |
| TDD-first | ✅ | `main_test.go` written with `main.go`; pytest built first |
| IaC/CaC | ✅ | agent-cards are YAML in repo; hooks are bash scripts in repo |
| Idempotency | ✅ | orchestrator is read-only; no state mutations |
| No shell leaks | ✅ | hooks use `set -euo pipefail`; long inputs via heredoc |
| Token saving | n/a | this sprint doesn't make LLM calls |
| Carry-forward register | ✅ | appended (3 items) |

## 5. Carry-forward to v14517

- **Pair-lock across personas**: v14517 wires each persona's pair-lock
  to a single shared `.sprint_lock` file so two personas can't run in
  the same repo simultaneously.
- **Persona-schema doc** (`docs/persona-schema.md`): full JSON-schema
  equivalent of the YAML.
- **Hook integration test**: spawn `choose-llm hook decide` from a
  real beforeSubmitPrompt call and assert the redirect.
- **Agent-card v2 fields**: `expires_at`, `cost_center`,
  `audit_log_path`. None required for v14516 but flagged for v14520
  EvoSpine.

## 6. Files in v14516

```
helixon-fleet-agents/
├── README.md
├── .gitignore
├── .sprint_lock                                     # pair-lock
├── personas/
│   ├── code-reviewer/
│   │   ├── agent-card.yaml
│   │   ├── skills/code-reviewer/SKILL.md
│   │   └── hooks/{beforeSubmitPrompt,afterFileEdit}.sh
│   ├── ops-engineer/      (same shape)
│   ├── sre/               (same shape)
│   └── release-manager/   (same shape)
├── tools/fleet-orchestrator/
│   ├── go.mod
│   ├── main.go
│   ├── main_test.go        # 10 tests
│   └── go.sum
├── tests/
│   └── test_fleet_orchestrator.py  # 10 tests
└── session-handoffs/v14516-handoff.md   # this file
```

## 7. Restart prompt for v14517

> Continue with v14517 Pair 7 Review: fleet agents adopt paired-sprint
> pattern; pair-lock for concurrent-pair prevention. Wire all 4
> personas into a single shared `.sprint_lock` so two personas
> cannot run the same sprint. Re-run fleet-orchestrator tests.
> Cross-link agent-cards to helixon-platform's CI gauntlet.