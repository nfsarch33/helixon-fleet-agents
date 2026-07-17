package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tenantCard = `schema_version: 1
persona:
  id: tenant-tester
  display_name: Tenant Tester
  description: for unit tests
  owner: test-owner
  home_repo: test-repo
  reviews_repos: [test-repo]
  default_tier: 2
  budget_usd_per_day: 1.50
  tenant_id: acme-corp
  tenant_isolation: per-persona-context
skills:
  bundle: skills/tester/
  required: [test-skill]
hooks:
  beforeSubmitPrompt: hooks/beforeSubmitPrompt.sh
  afterFileEdit: hooks/afterFileEdit.sh
pair_lock: .sprint_lock
`

func TestLoadCard_ReadsTenantFields(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, "tenant-tester")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "agent-card.yaml")
	if err := os.WriteFile(p, []byte(tenantCard), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCard(p)
	if err != nil {
		t.Fatalf("LoadCard: %v", err)
	}
	if c.Persona.TenantID != "acme-corp" {
		t.Fatalf("TenantID = %q; want %q", c.Persona.TenantID, "acme-corp")
	}
	if c.Persona.TenantIsolation != "per-persona-context" {
		t.Fatalf("TenantIsolation = %q; want %q", c.Persona.TenantIsolation, "per-persona-context")
	}
}

func TestLoadCard_MissingTenantDefaultsToEmpty(t *testing.T) {
	dir := t.TempDir()
	d := filepath.Join(dir, "tester")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, "agent-card.yaml")
	if err := os.WriteFile(p, []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCard(p)
	if err != nil {
		t.Fatalf("LoadCard: %v", err)
	}
	if c.Persona.TenantID != "" {
		t.Fatalf("TenantID = %q; want empty", c.Persona.TenantID)
	}
}

func TestCmdList_SurfacesTenantColumn(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "tenant-tester")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "agent-card.yaml"), []byte(tenantCard), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := cmdList(&buf, root); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "TENANT") {
		t.Fatalf("missing TENANT column: %s", out)
	}
	if !strings.Contains(out, "acme-corp") {
		t.Fatalf("missing tenant value: %s", out)
	}
}

func TestCmdList_DefaultsToDefaultWhenTenantMissing(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "tester")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "agent-card.yaml"), []byte(validCard), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := cmdList(&buf, root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "default") {
		t.Fatalf("missing default tenant placeholder: %s", buf.String())
	}
}

func TestCmdValidate_PrintsTenant(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "tenant-tester")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "agent-card.yaml"), []byte(tenantCard), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := cmdValidate(&buf, root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "tenant=acme-corp") {
		t.Fatalf("missing tenant in validate output: %s", buf.String())
	}
}
