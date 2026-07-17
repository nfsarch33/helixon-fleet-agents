# Persona schema (v14517 + v18675-4)

**Status:** Accepted — companion to `agent-card.yaml` schema-version 1.
**Owner:** Helixon fleet-agents team.
**Sprint:** v14517 Pair 7 Review; v18675-4 adds tenant-aware audit-trail fields.

This document defines every field in the `agent-card.yaml` schema. It
is the canonical reference for persona authors and orchestrator
implementers (Go, Python, future Rust).

## Top-level fields

| Field | Type | Required | Default | Notes |
| --- | --- | --- | --- | --- |
| `schema_version` | int | yes | — | must be `1` |
| `persona` | object | yes | — | see below |
| `skills` | object | yes | — | see below |
| `hooks` | object | yes | — | see below |
| `pair_lock` | string | no | `.sprint_lock` | filename relative to repo root |

## `persona` object

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `id` | string | yes | kebab-case, unique across the fleet |
| `display_name` | string | yes | human-readable label |
| `description` | string | yes | multi-line allowed (`>` YAML block) |
| `owner` | string | yes | team identifier |
| `home_repo` | string | yes | GitHub repo this persona primarily operates on |
| `reviews_repos` | []string | no | additional repos this persona reviews |
| `default_tier` | int | yes | 0..3 (see choose-llm tier router) |
| `budget_usd_per_day` | float | yes | daily cost ceiling (USD) |
| `tier_routing` | object | no | `default_tier`, `escalate_tier_on: []string` |
| `tenant_id` | string | no (v18675-4) | tenant attribution; default `shared-fleet` |
| `tenant_isolation` | string | no (v18675-4) | `per-persona-context` or `shared` |

### Reserved persona IDs

| ID | Owner |
| -- | --- |
| `code-reviewer` | helixon-platform-team |
| `ops-engineer` | helixon-platform-team |
| `sre` | helixon-platform-team |
| `release-manager` | helixon-platform-team |

New personas must register their ID in the Sprintboard MCP before use.

## `skills` object

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `bundle` | string | yes | path to `SKILL.md`, relative to persona dir |
| `required` | []string | yes | list of skill IDs (cross-references `cursor-global-kb/skills/`) |

Skill IDs are kebab-case and must exist in the global skill catalogue.
The orchestrator validates this at load time (v14517+).

## `hooks` object

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `beforeSubmitPrompt` | string | yes | path to shell script (relative to persona dir) |
| `afterFileEdit` | string | yes | path to shell script (relative to persona dir) |

The `beforeSubmitPrompt` hook:

1. Receives Cursor's beforeSubmitPrompt JSON on stdin.
2. Reads `.sprint_lock` (v14517) and refuses if a different pair is active.
3. Invokes `choose-llm hook decide --persona <id>` (v14511).
4. Emits a redirect Output JSON on stdout.

The `afterFileEdit` hook:

1. Receives Cursor's afterFileEdit JSON on stdin.
2. Appends a `{ts, persona_id, file, added, removed}` record to
   `.fleet-trail/<persona>.ndjson`.
3. Exits 0 silently if the payload is unparseable.

## `pair_lock` field

The `pair_lock` field points to the lock file used by `tools/pair-lock`.
Default: `.sprint_lock` at repo root.

The pair-lock enforces:

- One active sprint pair per repo at a time.
- Re-entrant acquires (same `pair_id`) are allowed.
- A conflicting acquire from a different `pair_id` returns
  `ErrConflict` and exits 1.

## Validation

`tools/fleet-orchestrator validate --dir <personas-root>` runs all
checks and emits a per-persona OK / FAIL line. The orchestrator exits
non-zero if any persona fails.

## v18675-4 — tenant-aware audit log (CF-172 sibling)

Per-tenant attribution across the helixon fleet is anchored by two
fields on the persona card and one per-pair-lock tenant_id:

| Field | Where | Default | Effect |
| --- | --- | --- | --- |
| `persona.tenant_id` | agent-card.yaml | `shared-fleet` | Persona-level tenant attribution. Read by the orchestrator `list` and `validate` commands and surfaced in their output. |
| `persona.tenant_isolation` | agent-card.yaml | `shared` | Hint to the orchestrator: `per-persona-context` keeps the persona's audit trail scoped to the persona, `shared` aggregates to fleet-wide. |
| `lock.tenant_id` | `.sprint_lock` | `default` | The pair-lock records which tenant the active sprint pair belongs to. Set via `pair-lock acquire --tenant <id>` or via the `HELIXON_TENANT_ID` env var. |

The `pair-lock audit` subcommand writes a NDJSON record per agent
decision to `.fleet-trail/pair-lock.ndjson` (overridable via
`--audit-path`). Each record has the shape:

```json
{"ts": "2026-07-17T03:00:00Z", "event": "pair-lock.audit", "pair_id": "v18675",
 "tenant_id": "acme-corp", "operator": "cursor-ai", "repo": "helixon-fleet-agents",
 "action": "beforeSubmitPrompt", "detail": "code-reviewer:local-tier2"}
```

The cost pipeline (helixon-platform `internal/costobs/`) consumes
the audit NDJSON and attributes per-prompt spend to the tenant.

## Migration

| From | To | Notes |
| --- | --- | --- |
| `schema_version: 1` (without `pair_lock`) | `schema_version: 1` (with `pair_lock: .sprint_lock`) | default applied by orchestrator if field omitted |
| `default_tier` nested under `persona.tier_routing` | `persona.default_tier` (top-level) | v14516 left both; v14517 prefers the top-level form |
| `schema_version: 1` (without `tenant_id`) | `schema_version: 1` (with `tenant_id: shared-fleet`) | v18675-4 default applied by orchestrator if field omitted |

## Migration

| From | To | Notes |
| --- | --- | --- |
| `schema_version: 1` (without `pair_lock`) | `schema_version: 1` (with `pair_lock: .sprint_lock`) | default applied by orchestrator if field omitted |
| `default_tier` nested under `persona.tier_routing` | `persona.default_tier` (top-level) | v14516 left both; v14517 prefers the top-level form |

## Versioning

| Schema version | Status | Notes |
| --- | --- | --- |
| 1 | accepted | current |
| 2 | draft | add `expires_at`, `cost_center`, `audit_log_path` (v14520 EvoSpine) |