package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveTenantID_FlagWinsOverEnv(t *testing.T) {
	t.Setenv("HELIXON_TENANT_ID", "env-tenant")
	if got := ResolveTenantID("flag-tenant"); got != "flag-tenant" {
		t.Fatalf("ResolveTenantID(flag) = %q; want %q", got, "flag-tenant")
	}
}

func TestResolveTenantID_EnvWinsOverDefault(t *testing.T) {
	t.Setenv("HELIXON_TENANT_ID", "env-tenant")
	if got := ResolveTenantID(""); got != "env-tenant" {
		t.Fatalf("ResolveTenantID(\"\") = %q; want %q", got, "env-tenant")
	}
}

func TestResolveTenantID_DefaultWhenNothing(t *testing.T) {
	t.Setenv("HELIXON_TENANT_ID", "")
	if got := ResolveTenantID(""); got != "default" {
		t.Fatalf("ResolveTenantID(\"\") = %q; want %q", got, "default")
	}
}

func TestAcquire_StampsTenantOnLock(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	t.Setenv("HELIXON_TENANT_ID", "acme-corp")

	if err := Acquire(p, Lock{PairID: "v18675", Operator: "test"}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.TenantID != "acme-corp" {
		t.Fatalf("TenantID = %q; want %q", got.TenantID, "acme-corp")
	}
}

func TestAcquire_DefaultsToDefaultWhenNoTenant(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	t.Setenv("HELIXON_TENANT_ID", "")

	if err := Acquire(p, Lock{PairID: "v18675", Operator: "test"}); err != nil {
		t.Fatal(err)
	}
	got, err := Read(p)
	if err != nil {
		t.Fatal(err)
	}
	if got.TenantID != "default" {
		t.Fatalf("TenantID = %q; want %q", got.TenantID, "default")
	}
}

func TestAcquire_ExplicitTenantWinsOverEnv(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".sprint_lock")
	t.Setenv("HELIXON_TENANT_ID", "env-tenant")

	if err := Acquire(p, Lock{PairID: "v18675", Operator: "test", TenantID: "explicit-tenant"}); err != nil {
		t.Fatal(err)
	}
	got, _ := Read(p)
	if got.TenantID != "explicit-tenant" {
		t.Fatalf("TenantID = %q; want %q", got.TenantID, "explicit-tenant")
	}
}

func TestAppendAudit_WritesNDJSONLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "audit.ndjson")
	e := AuditEntry{
		TS:       time.Now().UTC().Format(time.RFC3339Nano),
		Event:    "pair-lock.audit",
		TenantID: "acme-corp",
		Action:   "test-action",
		Detail:   "hello",
	}
	if err := AppendAudit(p, e); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	// Single line ending with newline.
	if c := strings.Count(string(data), "\n"); c != 1 {
		t.Fatalf("expected 1 newline, got %d", c)
	}
	var got AuditEntry
	if err := json.Unmarshal(data[:len(data)-1], &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TenantID != "acme-corp" || got.Action != "test-action" {
		t.Fatalf("got %+v", got)
	}
}

func TestAppendAudit_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "nested", "deeper", "audit.ndjson")
	if err := AppendAudit(p, AuditEntry{TenantID: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("file not created: %v", err)
	}
}

func TestAppendAudit_AppendsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "audit.ndjson")
	for i := 0; i < 3; i++ {
		if err := AppendAudit(p, AuditEntry{TenantID: "tenant", Action: "step"}); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(p)
	if c := strings.Count(string(data), "\n"); c != 3 {
		t.Fatalf("expected 3 lines, got %d", c)
	}
	// Each line must be valid JSON.
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	n := 0
	for sc.Scan() {
		var e AuditEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("line %d bad json: %v", n, err)
		}
		n++
	}
	if n != 3 {
		t.Fatalf("scanned %d lines", n)
	}
}

func TestAppendAudit_DefaultsTimestampWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "audit.ndjson")
	if err := AppendAudit(p, AuditEntry{TenantID: "x"}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(p)
	var got AuditEntry
	if err := json.Unmarshal(data[:len(data)-1], &got); err != nil {
		t.Fatal(err)
	}
	if got.TS == "" {
		t.Fatal("TS should be auto-populated")
	}
}

func TestCmdAudit_WritesRecordWithTenant(t *testing.T) {
	dir := t.TempDir()
	audit := filepath.Join(dir, "audit.ndjson")
	t.Setenv("HELIXON_TENANT_ID", "audit-tenant")

	// cmdAudit reads .sprint_lock relative to cwd; chdir to dir so it
	// does not pick up a stale lock.
	oldCwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldCwd)

	if err := cmdAudit(os.Stdout, audit, "test-action", "detail-string"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(audit)
	if c := strings.Count(string(data), "\n"); c != 1 {
		t.Fatalf("expected 1 line, got %d", c)
	}
	var got AuditEntry
	if err := json.Unmarshal(data[:len(data)-1], &got); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if got.TenantID != "audit-tenant" {
		t.Fatalf("TenantID = %q; want %q", got.TenantID, "audit-tenant")
	}
	if got.Action != "test-action" {
		t.Fatalf("Action = %q", got.Action)
	}
	if got.Event != "pair-lock.audit" {
		t.Fatalf("Event = %q", got.Event)
	}
}

func TestCmdAudit_EnrichedByActiveLock(t *testing.T) {
	dir := t.TempDir()
	audit := filepath.Join(dir, "audit.ndjson")
	lock := filepath.Join(dir, ".sprint_lock")
	t.Setenv("HELIXON_TENANT_ID", "")

	oldCwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldCwd)

	if err := Acquire(lock, Lock{PairID: "v18675", Operator: "tester", TenantID: "lock-tenant"}); err != nil {
		t.Fatal(err)
	}
	if err := cmdAudit(os.Stdout, audit, "edit", "modified foo.go"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(audit)
	var got AuditEntry
	_ = json.Unmarshal(data[:len(data)-1], &got)
	if got.PairID != "v18675" {
		t.Fatalf("PairID = %q; want %q", got.PairID, "v18675")
	}
	if got.Operator != "tester" {
		t.Fatalf("Operator = %q; want %q", got.Operator, "tester")
	}
	// audit inherits the lock's tenant_id over the env var.
	if got.TenantID != "lock-tenant" {
		t.Fatalf("TenantID = %q; want %q", got.TenantID, "lock-tenant")
	}
}
