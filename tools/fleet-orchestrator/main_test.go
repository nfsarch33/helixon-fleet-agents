package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validCard = `schema_version: 1
persona:
  id: tester
  display_name: Test Persona
  description: for unit tests
  owner: test-owner
  home_repo: test-repo
  reviews_repos: [test-repo]
  default_tier: 2
  budget_usd_per_day: 1.50
skills:
  bundle: skills/tester/
  required: [test-skill]
hooks:
  beforeSubmitPrompt: hooks/beforeSubmitPrompt.sh
  afterFileEdit: hooks/afterFileEdit.sh
pair_lock: .sprint_lock
`

func writeCard(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "tester"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "tester", name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadCard_Valid(t *testing.T) {
	dir := t.TempDir()
	p := writeCard(t, dir, "agent-card.yaml", validCard)
	c, err := LoadCard(p)
	if err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
	if c.SchemaVersion != 1 {
		t.Fatalf("schema=%d", c.SchemaVersion)
	}
	if c.Persona.ID != "tester" {
		t.Fatalf("id=%q", c.Persona.ID)
	}
	if c.Persona.DefaultTier != 2 {
		t.Fatalf("tier=%d", c.Persona.DefaultTier)
	}
	if c.Persona.BudgetUSDDay != 1.50 {
		t.Fatalf("budget=%v", c.Persona.BudgetUSDDay)
	}
	if c.Skills.Bundle == "" || c.Hooks.BeforeSubmitPrompt == "" {
		t.Fatal("required fields missing")
	}
}

func TestLoadCard_RejectsBadSchema(t *testing.T) {
	dir := t.TempDir()
	p := writeCard(t, dir, "agent-card.yaml",
		strings.Replace(validCard, "schema_version: 1", "schema_version: 2", 1))
	if _, err := LoadCard(p); err == nil {
		t.Fatal("expected schema error")
	}
}

func TestLoadCard_RejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	bad := strings.Replace(validCard, "id: tester", "id: \"\"", 1)
	p := writeCard(t, dir, "agent-card.yaml", bad)
	if _, err := LoadCard(p); err == nil {
		t.Fatal("expected missing-id error")
	}
}

func TestLoadCard_RejectsMissingSkillsBundle(t *testing.T) {
	dir := t.TempDir()
	bad := strings.Replace(validCard, "bundle: skills/tester/", "bundle: \"\"", 1)
	p := writeCard(t, dir, "agent-card.yaml", bad)
	if _, err := LoadCard(p); err == nil {
		t.Fatal("expected missing-bundle error")
	}
}

func TestLoadAll_FindsAllCards(t *testing.T) {
	root := t.TempDir()
	// Create 3 personas
	for _, id := range []string{"alpha", "beta", "gamma"} {
		d := filepath.Join(root, id)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		card := strings.Replace(validCard, "id: tester", "id: "+id, 1)
		if err := os.WriteFile(filepath.Join(d, "agent-card.yaml"), []byte(card), 0o644); err != nil {
			t.Fatal(err)
		}
		// Decoy file that should be ignored
		if err := os.WriteFile(filepath.Join(d, "notes.md"), []byte("noise"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cards, err := LoadAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 3 {
		t.Fatalf("got %d cards, want 3", len(cards))
	}
	// Sorted by id
	want := []string{"alpha", "beta", "gamma"}
	for i, c := range cards {
		if c.Persona.ID != want[i] {
			t.Fatalf("position %d: got %q want %q", i, c.Persona.ID, want[i])
		}
	}
}

func TestCmdList_PrintsTable(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"alpha", "beta"} {
		d := filepath.Join(root, id)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		card := strings.Replace(validCard, "id: tester", "id: "+id, 1)
		if err := os.WriteFile(filepath.Join(d, "agent-card.yaml"), []byte(card), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := cmdList(&buf, root); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "beta") {
		t.Fatalf("missing ids in output: %s", out)
	}
	if !strings.Contains(out, "PERSONA_ID") {
		t.Fatalf("missing header: %s", out)
	}
}

func TestCmdValidate_AllOK(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"alpha", "beta"} {
		d := filepath.Join(root, id)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		card := strings.Replace(validCard, "id: tester", "id: "+id, 1)
		if err := os.WriteFile(filepath.Join(d, "agent-card.yaml"), []byte(card), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := cmdValidate(&buf, root); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "2 persona(s) valid") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestCmdCard_Found(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "alpha")
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	card := strings.Replace(validCard, "id: tester", "id: alpha", 1)
	if err := os.WriteFile(filepath.Join(d, "agent-card.yaml"), []byte(card), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := cmdCard(&buf, root, "alpha"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "id: alpha") {
		t.Fatalf("got %q", buf.String())
	}
}

func TestCmdCard_NotFound(t *testing.T) {
	root := t.TempDir()
	var buf bytes.Buffer
	if err := cmdCard(&buf, root, "missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCmdSchema_PrintsVersion(t *testing.T) {
	var buf bytes.Buffer
	cmdSchema(&buf)
	if !strings.Contains(buf.String(), "schema_version: 1") {
		t.Fatalf("got %q", buf.String())
	}
}