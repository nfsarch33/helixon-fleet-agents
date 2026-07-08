package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRead_MissingFile(t *testing.T) {
	l, err := Read(filepath.Join(t.TempDir(), "lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	if l != nil {
		t.Fatalf("expected nil, got %+v", l)
	}
}

func TestReadWrite_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	want := Lock{
		SchemaVersion: schemaVersion,
		PairID:        "v14517",
		SprintID:      "v14517",
		Phase:         "running",
		StartedAt:     time.Now().UTC(),
		PID:           12345,
		Operator:      "test",
		Personas:      []string{"code-reviewer"},
	}
	if err := Write(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.PairID != want.PairID || got.Phase != want.Phase {
		t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

func TestRead_RejectsBadSchema(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	if err := os.WriteFile(p, []byte(`{"schema_version": 99, "pair_id": "x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(p); err == nil {
		t.Fatal("expected schema error")
	}
}

func TestAcquire_EmptyLockSucceeds(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	l := Lock{PairID: "v14517", Operator: "test"}
	if err := Acquire(p, l); err != nil {
		t.Fatal(err)
	}
	got, _ := Read(p)
	if got == nil || got.PairID != "v14517" {
		t.Fatalf("got %+v", got)
	}
}

func TestAcquire_ConflictOnDifferentPair(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	if err := Acquire(p, Lock{PairID: "v14517"}); err != nil {
		t.Fatal(err)
	}
	err := Acquire(p, Lock{PairID: "v14518"})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "another pair is active") {
		t.Fatalf("got %v", err)
	}
}

func TestAcquire_SamePairReentrant(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	if err := Acquire(p, Lock{PairID: "v14517"}); err != nil {
		t.Fatal(err)
	}
	// same pair acquires again — no error
	if err := Acquire(p, Lock{PairID: "v14517"}); err != nil {
		t.Fatalf("reentrant should succeed: %v", err)
	}
}

func TestRelease_MarksClosed(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	if err := Acquire(p, Lock{PairID: "v14517"}); err != nil {
		t.Fatal(err)
	}
	if err := Release(p); err != nil {
		t.Fatal(err)
	}
	got, _ := Read(p)
	if got.Phase != "closed" {
		t.Fatalf("phase=%q", got.Phase)
	}
	if got.ClosedAt.IsZero() {
		t.Fatal("closed_at should be set")
	}
}

func TestRelease_NothingToRelease(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	if err := Release(p); err == nil {
		t.Fatal("expected error")
	}
}

func TestWrite_AtomicNoTorn(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	if err := Write(p, Lock{SchemaVersion: schemaVersion, PairID: "v14517"}); err != nil {
		t.Fatal(err)
	}
	// tmp file should not exist
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file lingered")
	}
	// file should be valid JSON
	data, _ := os.ReadFile(p)
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("bad json: %v", err)
	}
}

func TestStatus_NoLock(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	if err := cmdStatus(io.Discard, p); err != nil {
		t.Fatal(err)
	}
}

func TestStatus_WithLock(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	_ = Write(p, Lock{SchemaVersion: schemaVersion, PairID: "v14517", Phase: "running", StartedAt: time.Now().UTC()})
	if err := cmdStatus(io.Discard, p); err != nil {
		t.Fatal(err)
	}
}