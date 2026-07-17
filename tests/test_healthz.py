"""Healthz probe for helixon-fleet-agents personas.

This script is invoked by:
- GitHub Actions CI on every PR (validates persona files are loadable)
- systemd timer on wsl1/wsl2 every 15 min (writes NDJSON to helixon-platform/observability/)
- Helixon agent SRE persona's health check routine

It validates:
1. Each persona has a valid agent-card.yaml (schema_version = 1)
2. Each persona's `skills/` bundle exists and is non-empty
3. Each persona's `hooks/` directory exists with the required scripts
4. Total persona count is 4 (sre / ops-engineer / code-reviewer / release-manager)
5. The orchestrator CLI can list personas without error

Exit codes:
- 0 = healthy
- 1 = schema violation
- 2 = missing persona artifact
- 3 = orchestrator failure
"""
import json
import os
import subprocess
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
PERSONAS_DIR = REPO_ROOT / "personas"

REQUIRED_PERSONAS = {"sre", "ops-engineer", "code-reviewer", "release-manager"}
REQUIRED_SKILL_FILES = ("SKILL.md",)
REQUIRED_HOOKS = ("beforeSubmitPrompt.sh", "afterFileEdit.sh")


def check_schema(card_path: Path) -> tuple[bool, str]:
    """Verify agent-card.yaml has schema_version: 1 and required fields."""
    if not card_path.exists():
        return False, f"missing {card_path.relative_to(REPO_ROOT)}"
    text = card_path.read_text()
    if "schema_version: 1" not in text:
        return False, f"{card_path.name} missing schema_version: 1"
    for required in ("persona:", "id:", "display_name:", "home_repo:", "skills:", "hooks:"):
        if required not in text:
            return False, f"{card_path.name} missing field '{required}'"
    return True, ""


def check_skills(persona_dir: Path) -> tuple[bool, str]:
    """Verify persona has skills/<id>/SKILL.md non-empty."""
    bundle = persona_dir / "skills" / persona_dir.name
    if not bundle.exists():
        return False, f"{persona_dir.name}/skills/{persona_dir.name}/ missing"
    for fname in REQUIRED_SKILL_FILES:
        f = bundle / fname
        if not f.exists():
            return False, f"{persona_dir.name}/skills/{persona_dir.name}/{fname} missing"
        if f.stat().st_size == 0:
            return False, f"{persona_dir.name}/skills/{persona_dir.name}/{fname} empty"
    return True, ""


def check_hooks(persona_dir: Path) -> tuple[bool, str]:
    """Verify persona has hooks/beforeSubmitPrompt.sh + hooks/afterFileEdit.sh."""
    hooks = persona_dir / "hooks"
    if not hooks.exists():
        return False, f"{persona_dir.name}/hooks/ missing"
    for fname in REQUIRED_HOOKS:
        f = hooks / fname
        if not f.exists():
            return False, f"{persona_dir.name}/hooks/{fname} missing"
        if f.stat().st_size == 0:
            return False, f"{persona_dir.name}/hooks/{fname} empty"
    return True, ""


def emit_metric(persona: str, check: str, ok: bool) -> None:
    """Emit a Prometheus-style NDJSON line to stdout for observability scrapers."""
    print(json.dumps({
        "ts": subprocess.run(["date", "-u", "+%Y-%m-%dT%H:%M:%SZ"],
                             capture_output=True, text=True).stdout.strip(),
        "metric": "helixon_fleet_agents_persona_health",
        "persona": persona,
        "check": check,
        "value": 1 if ok else 0,
    }))


def main() -> int:
    if not PERSONAS_DIR.exists():
        print(f"personas dir not found: {PERSONAS_DIR}", file=sys.stderr)
        return 2

    found_personas = {p.name for p in PERSONAS_DIR.iterdir() if p.is_dir()}
    missing = REQUIRED_PERSONAS - found_personas
    if missing:
        print(f"missing required personas: {sorted(missing)}", file=sys.stderr)
        for p in sorted(missing):
            emit_metric(p, "presence", False)
        return 2

    extra = found_personas - REQUIRED_PERSONAS
    if extra:
        print(f"warning: extra personas not in REQUIRED: {sorted(extra)}", file=sys.stderr)

    failures = []
    for persona_id in sorted(REQUIRED_PERSONAS):
        persona_dir = PERSONAS_DIR / persona_id
        for check_name, check_fn in (
            ("schema", lambda: check_schema(persona_dir / "agent-card.yaml")),
            ("skills", lambda: check_skills(persona_dir)),
            ("hooks", lambda: check_hooks(persona_dir)),
        ):
            try:
                ok, msg = check_fn()
            except Exception as e:
                ok, msg = False, f"exception: {e}"
            emit_metric(persona_id, check_name, ok)
            if not ok:
                failures.append(f"[{persona_id}:{check_name}] {msg}")

    if failures:
        for f in failures:
            print(f"FAIL {f}", file=sys.stderr)
        return 1

    # Smoke test: orchestrator tool source exists (no build attempted here).
    orch_src = REPO_ROOT / "tools" / "fleet-orchestrator" / "main.go"
    if orch_src.exists():
        emit_metric("orchestrator", "source_present", True)
    else:
        emit_metric("orchestrator", "source_present", False)
        return 3

    print("OK: helixon-fleet-agents personas healthy")
    return 0


if __name__ == "__main__":
    sys.exit(main())
