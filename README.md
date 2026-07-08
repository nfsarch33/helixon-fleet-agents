# Helixon Fleet Agents

Sprint: **v14516 Pair 7 MVP** — 2026-07-15

Persona-driven agents for the Helixon fleet. Each persona owns one
slice of the platform lifecycle; together they form the operating
loop that pairs with the human operator (jaslian) and the Sentrux
audit cadence.

## Personas

| ID | Display name | Responsibility |
| -- | ------------ | -------------- |
| `code-reviewer` | Helixon Code Reviewer | PR diffs, security, supply-chain, TDD coverage, IaC/CaC |
| `ops-engineer` | Helixon Ops Engineer | ssh/agentcage/observability/IaC rollout/secrets |
| `sre` | Helixon SRE | SLOs, alerts, paging, post-incident review, EvoSpine trigger |
| `release-manager` | Helixon Release Manager | Tagging, ADR bundles, closure gate |

Each persona ships:

- `agent-card.yaml` — schema-version 1, the source of truth.
- `skills/<id>/SKILL.md` — required-skill bundle + workflow + tone.
- `hooks/beforeSubmitPrompt.sh` — tier-routing redirect.
- `hooks/afterFileEdit.sh` — NDJSON trail writer.

## Layout

```
helixon-fleet-agents/
├── README.md
├── personas/
│   ├── code-reviewer/
│   │   ├── agent-card.yaml
│   │   ├── skills/code-reviewer/SKILL.md
│   │   └── hooks/
│   │       ├── beforeSubmitPrompt.sh
│   │       └── afterFileEdit.sh
│   ├── ops-engineer/
│   │   ├── agent-card.yaml
│   │   ├── skills/ops-engineer/SKILL.md
│   │   └── hooks/
│   │       ├── beforeSubmitPrompt.sh
│   │       └── afterFileEdit.sh
│   ├── sre/
│   │   ├── agent-card.yaml
│   │   ├── skills/sre/SKILL.md
│   │   └── hooks/
│   │       ├── beforeSubmitPrompt.sh
│   │       └── afterFileEdit.sh
│   └── release-manager/
│       ├── agent-card.yaml
│       ├── skills/release-manager/SKILL.md
│       └── hooks/
│           ├── beforeSubmitPrompt.sh
│           └── afterFileEdit.sh
├── tools/
│   └── fleet-orchestrator/  (v14517)
└── docs/
    ├── persona-schema.md    (v14517)
    └── evospine-cycle.md    (v14520)
```

## Quick start

```bash
# List all personas
ls personas/

# Validate an agent-card
yq eval '.schema_version' personas/code-reviewer/agent-card.yaml
# → 1

# Install hooks into ~/.cursor/hooks.json
cp personas/code-reviewer/hooks/* ~/.cursor/hooks/

# Invoke a persona manually
HELIXON_AGENT_PERSONA=code-reviewer \
HELIXON_AGENT_TIER=2 \
HELIXON_SPRINT_ID=v14517 \
  bash personas/code-reviewer/hooks/beforeSubmitPrompt.sh \
  <<<'{"prompt":"review this PR"}'
```

## Acceptance for v14516

- [x] 4 personas with `agent-card.yaml` (schema 1)
- [x] 4 `SKILL.md` skill bundles
- [x] 4 `hooks/beforeSubmitPrompt.sh`
- [x] 4 `hooks/afterFileEdit.sh`
- [x] Tier-4 cross-layer verifier (helixon-platform) still PASS

## Carry-forward to v14517

- `tools/fleet-orchestrator/` Go CLI (load + validate all agent-cards)
- Persona-scheme documentation (`docs/persona-schema.md`)
- Pair-lock per-persona (`.sprint_lock` shared across the 4 personas)
- Cross-repo coordination rules (review-manager vs code-reviewer)