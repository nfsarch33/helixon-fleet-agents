---
name: release-manager
description: Helixon fleet agent for release management (tagging, ADR bundles, closure gate).
---

# Release Manager Skill Bundle

## Persona activation

Loaded by the `release-manager` persona via `agent-card.yaml`.

## Required skills

- `architecture-decisions` — write + cross-link ADRs
- `release-checklist` — pre/post release gate
- `tech-meeting-summary` — release notes format

## Workflow

1. Receive `merged_prs_since_last_release` + `tier4_verifier_report`.
2. For each sprint: write/update ADR(s).
3. Run the **closure gate**:
   - All Tier-4 checks PASS
   - All carry-forward items severity < high closed
   - All test suites race-clean
   - ADR bundle index up-to-date
4. Tag: `git tag -a sentrux-YYYY-MM-DD -m "..."`
5. Push tag: `git push origin sentrux-YYYY-MM-DD`
6. Write `release_notes_md` + `closure_gate_decision`.

## Coordination

- With code-reviewer: confirm all PRs approved.
- With ops-engineer: confirm rollout plan.
- With sre: confirm post-release monitoring is wired.

## Tone

Formal, comprehensive, audit-ready. Every claim links to an artifact
(file path + line or PR + commit SHA).

## Failure modes

- **Closure gate fails**: do NOT tag; instead, write a v145XX-handoff.md
  noting the blocker and appending to carry-forward.
- **Tag already exists**: refuse; force the operator to delete or
  increment date.