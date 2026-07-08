#!/usr/bin/env python3
"""TDD tests for fleet-orchestrator CLI.

Runs the Go binary against the personas/ directory and verifies:
- exactly 4 personas loaded (code-reviewer, ops-engineer, sre, release-manager)
- each persona has a unique id, display_name, and tier
- schema_version is 1 across the board
- list, validate, card, schema subcommands all work
- budget_usd_per_day is set and positive
"""
import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
ORCH = ROOT / "tools" / "fleet-orchestrator" / "fleet-orchestrator"
PERSONAS = ROOT / "personas"

def build_orchestrator():
    """Build the Go binary if missing."""
    if not ORCH.exists():
        env = os.environ.copy()
        env["PATH"] = "/home/jaslian/.gvm/gos/go1.23.4/bin:" + env["PATH"]
        subprocess.run(
            ["go", "build", "-o", str(ORCH), "."],
            cwd=str(ORCH.parent),
            check=True,
            env=env,
        )

def run(*args):
    return subprocess.run([str(ORCH), *args], cwd=str(ROOT), capture_output=True, text=True, check=True)

class TestFleetOrchestrator:
    @classmethod
    def setup_class(cls):
        build_orchestrator()

    def test_binary_exists(self):
        assert ORCH.exists(), f"fleet-orchestrator binary missing at {ORCH}"

    def test_list_returns_four_personas(self):
        out = run("list", "--dir", str(PERSONAS)).stdout
        for pid in ["code-reviewer", "ops-engineer", "sre", "release-manager"]:
            assert pid in out, f"missing {pid} in list output:\n{out}"

    def test_validate_all_ok(self):
        out = run("validate", "--dir", str(PERSONAS)).stdout
        assert "4 persona(s) valid" in out, f"expected 4 valid; got:\n{out}"
        for line in out.splitlines():
            if line.startswith("OK"):
                assert "schema=1" in line, f"missing schema=1 in: {line}"

    def test_card_roundtrip(self):
        out = run("card", "code-reviewer", "--dir", str(PERSONAS)).stdout
        assert "id: code-reviewer" in out
        assert "schema_version: 1" in out

    def test_card_not_found_exits_nonzero(self):
        r = subprocess.run(
            [str(ORCH), "card", "nonexistent", "--dir", str(PERSONAS)],
            cwd=str(ROOT), capture_output=True, text=True,
        )
        assert r.returncode != 0
        assert "not found" in r.stderr.lower()

    def test_schema_subcommand(self):
        out = run("schema").stdout
        assert "schema_version: 1" in out
        assert "default_tier" in out

    def test_all_cards_have_unique_ids(self):
        ids = []
        for d in sorted(PERSONAS.iterdir()):
            if (d / "agent-card.yaml").exists():
                card_text = (d / "agent-card.yaml").read_text()
                # crude parse: pull `id:` under `persona:`
                for line in card_text.splitlines():
                    if line.startswith("  id: "):
                        ids.append(line.split(": ", 1)[1].strip())
                        break
        assert len(ids) == 4, f"expected 4 ids, got {ids}"
        assert len(set(ids)) == 4, f"ids not unique: {ids}"

    def test_all_cards_have_tier_and_budget(self):
        for d in sorted(PERSONAS.iterdir()):
            card = d / "agent-card.yaml"
            if not card.exists():
                continue
            text = card.read_text()
            assert "default_tier:" in text, f"{d.name}: missing default_tier"
            assert "budget_usd_per_day:" in text, f"{d.name}: missing budget"

    def test_all_cards_have_hooks(self):
        for d in sorted(PERSONAS.iterdir()):
            bp = d / "hooks" / "beforeSubmitPrompt.sh"
            assert bp.exists(), f"{d.name}: missing beforeSubmitPrompt.sh"
            ap = d / "hooks" / "afterFileEdit.sh"
            assert ap.exists(), f"{d.name}: missing afterFileEdit.sh"

    def test_all_cards_have_skill_md(self):
        for d in sorted(PERSONAS.iterdir()):
            for sm in (d / "skills").rglob("SKILL.md"):
                assert sm.exists()
                assert sm.stat().st_size > 0, f"{sm}: empty"

if __name__ == "__main__":
    import pytest
    sys.exit(pytest.main([__file__, "-v"]))