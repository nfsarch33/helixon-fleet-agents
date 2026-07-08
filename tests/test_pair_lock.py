#!/usr/bin/env python3
"""TDD tests for pair-lock CLI + hook integration.

- binary builds
- acquire -> release round-trip
- conflict on different pair_id
- reentrant on same pair_id
- beforeSubmitPrompt hook returns pair_lock_ok=true when lock matches
- beforeSubmitPrompt hook returns pair_lock_ok=false when no lock
"""
import json
import os
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
PAIR_LOCK_BIN = ROOT / "tools" / "pair-lock" / "pair-lock"
HOOK = ROOT / "personas" / "code-reviewer" / "hooks" / "beforeSubmitPrompt.sh"

def build():
    if not PAIR_LOCK_BIN.exists():
        env = os.environ.copy()
        env["PATH"] = "/home/jaslian/.gvm/gos/go1.23.4/bin:" + env["PATH"]
        subprocess.run(
            ["go", "build", "-o", str(PAIR_LOCK_BIN), "."],
            cwd=str(PAIR_LOCK_BIN.parent),
            check=True, env=env,
        )

def run_pair_lock(*args, cwd=None):
    return subprocess.run([str(PAIR_LOCK_BIN), *args], cwd=cwd or ROOT, capture_output=True, text=True)

class TestPairLock:
    @classmethod
    def setup_class(cls):
        build()

    def test_binary_exists(self):
        assert PAIR_LOCK_BIN.exists()

    def test_status_no_lock(self, tmp_path):
        r = run_pair_lock("status", "--file", str(tmp_path / ".sprint_lock"))
        assert r.returncode == 0
        assert "no active pair" in r.stdout

    def test_acquire_then_status(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        r = run_pair_lock("acquire", "--pair", "v14517", "--file", str(lock))
        assert r.returncode == 0, r.stderr
        assert "acquired pair v14517" in r.stdout

        r2 = run_pair_lock("status", "--file", str(lock))
        assert r2.returncode == 0
        assert '"pair_id"' in r2.stdout
        assert "v14517" in r2.stdout

    def test_conflict_on_different_pair(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        run_pair_lock("acquire", "--pair", "v14517", "--file", str(lock))
        r = run_pair_lock("acquire", "--pair", "v14518", "--file", str(lock))
        assert r.returncode != 0
        assert "another pair is active" in r.stderr

    def test_reentrant_same_pair(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        run_pair_lock("acquire", "--pair", "v14517", "--file", str(lock))
        r = run_pair_lock("acquire", "--pair", "v14517", "--file", str(lock))
        assert r.returncode == 0

    def test_release_then_aquire_new(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        run_pair_lock("acquire", "--pair", "v14517", "--file", str(lock))
        r = run_pair_lock("release", "--file", str(lock))
        assert r.returncode == 0
        # Now another pair can acquire
        r2 = run_pair_lock("acquire", "--pair", "v14518", "--file", str(lock))
        assert r2.returncode == 0

    def test_release_nothing_to_release(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        r = run_pair_lock("release", "--file", str(lock))
        assert r.returncode != 0

    def test_check_no_lock(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        r = run_pair_lock("check", "--file", str(lock))
        assert r.returncode == 0
        assert "no active pair" in r.stdout

    def test_check_active(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        run_pair_lock("acquire", "--pair", "v14517", "--operator", "test", "--file", str(lock))
        r = run_pair_lock("check", "--file", str(lock))
        assert r.returncode == 0
        assert "active" in r.stdout
        assert "v14517" in r.stdout


class TestBeforeSubmitPromptHook:
    def test_hook_returns_pair_lock_ok_true_when_lock_matches(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        run_pair_lock("acquire", "--pair", "v14517", "--file", str(lock))
        env = os.environ.copy()
        env["PATH"] = str(PAIR_LOCK_BIN.parent) + ":" + env["PATH"]
        env["HELIXON_AGENT_PERSONA"] = "code-reviewer"
        env["HELIXON_AGENT_TIER"] = "2"
        env["HELIXON_SPRINT_ID"] = "v14517"
        env["HELIXON_PAIR_LOCK"] = str(lock)
        r = subprocess.run(
            ["bash", str(HOOK)],
            input='{"prompt":"hello"}',
            capture_output=True, text=True, env=env,
        )
        assert r.returncode == 0, r.stderr
        data = json.loads(r.stdout)
        assert data["pair_lock_ok"] is True
        assert data["persona_id"] == "code-reviewer"

    def test_hook_returns_pair_lock_ok_false_when_no_lock(self, tmp_path):
        env = os.environ.copy()
        env["PATH"] = str(PAIR_LOCK_BIN.parent) + ":" + env["PATH"]
        env["HELIXON_AGENT_PERSONA"] = "code-reviewer"
        env["HELIXON_AGENT_TIER"] = "2"
        env["HELIXON_SPRINT_ID"] = "v14517"
        env["HELIXON_PAIR_LOCK"] = str(tmp_path / "no_such_lock")
        r = subprocess.run(
            ["bash", str(HOOK)],
            input='{"prompt":"hello"}',
            capture_output=True, text=True, env=env,
        )
        assert r.returncode == 0
        data = json.loads(r.stdout)
        assert data["pair_lock_ok"] is False
        assert data["decision_label"] == "no_decision"

    def test_hook_returns_pair_lock_ok_false_when_different_pair(self, tmp_path):
        lock = tmp_path / ".sprint_lock"
        run_pair_lock("acquire", "--pair", "v14599", "--file", str(lock))
        env = os.environ.copy()
        env["PATH"] = str(PAIR_LOCK_BIN.parent) + ":" + env["PATH"]
        env["HELIXON_AGENT_PERSONA"] = "code-reviewer"
        env["HELIXON_AGENT_TIER"] = "2"
        env["HELIXON_SPRINT_ID"] = "v14517"  # we expect v14517 but lock has v14599
        env["HELIXON_PAIR_LOCK"] = str(lock)
        r = subprocess.run(
            ["bash", str(HOOK)],
            input='{"prompt":"hello"}',
            capture_output=True, text=True, env=env,
        )
        assert r.returncode == 0
        data = json.loads(r.stdout)
        # When the lock is held by a different pair, the hook should
        # refuse. We check that pair_lock_ok is False OR decision is
        # no_decision.
        assert data["pair_lock_ok"] is False or data["decision_label"] == "no_decision"


if __name__ == "__main__":
    import pytest
    sys.exit(pytest.main([__file__, "-v"]))