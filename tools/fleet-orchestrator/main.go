// fleet-orchestrator (v14516 MVP) — load + validate persona agent-cards.
//
// Subcommands:
//
//	list               — print a table of persona id / tier / budget / owner
//	validate <dir>     — validate every agent-card.yaml under dir
//	card <id>          — print the parsed card for one persona
//	schema             — print the agent-card schema (schema_version 1)
//
// Source of truth: docs/persona-schema.md (v14517) and the agent-card.yaml
// files in this repo.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const schemaVersion = 1

type Persona struct {
	ID           string   `yaml:"id"`
	DisplayName  string   `yaml:"display_name"`
	Description  string   `yaml:"description"`
	Owner        string   `yaml:"owner"`
	HomeRepo     string   `yaml:"home_repo"`
	ReviewsRepos []string `yaml:"reviews_repos"`
	DefaultTier  int      `yaml:"default_tier"`
	BudgetUSDDay float64  `yaml:"budget_usd_per_day"`
	// TenantID + TenantIsolation (v18675-4, CF-172 sibling) attribute a
	// persona to a tenant. Default value "shared-fleet" matches the
	// existing fleet deployments.
	TenantID        string `yaml:"tenant_id"`
	TenantIsolation string `yaml:"tenant_isolation"`
}

type Skills struct {
	Bundle   string   `yaml:"bundle"`
	Required []string `yaml:"required"`
}

type Hooks struct {
	BeforeSubmitPrompt string `yaml:"beforeSubmitPrompt"`
	AfterFileEdit      string `yaml:"afterFileEdit"`
}

type AgentCard struct {
	SchemaVersion int     `yaml:"schema_version"`
	Persona       Persona `yaml:"persona"`
	Skills        Skills  `yaml:"skills"`
	Hooks         Hooks   `yaml:"hooks"`
	PairLock      string  `yaml:"pair_lock"`
}

// LoadCard parses one agent-card.yaml.
func LoadCard(path string) (*AgentCard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c AgentCard
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("%s: schema_version=%d expected %d", path, c.SchemaVersion, schemaVersion)
	}
	if c.Persona.ID == "" {
		return nil, fmt.Errorf("%s: persona.id is required", path)
	}
	if c.Skills.Bundle == "" {
		return nil, fmt.Errorf("%s: skills.bundle is required", path)
	}
	if c.Hooks.BeforeSubmitPrompt == "" {
		return nil, fmt.Errorf("%s: hooks.beforeSubmitPrompt is required", path)
	}
	return &c, nil
}

// LoadAll walks dir for agent-card.yaml files and returns them sorted by id.
func LoadAll(dir string) ([]*AgentCard, error) {
	var out []*AgentCard
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(p) != "agent-card.yaml" {
			return nil
		}
		c, err := LoadCard(p)
		if err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Persona.ID < out[j].Persona.ID })
	return out, nil
}

func cmdList(w io.Writer, dir string) error {
	cards, err := LoadAll(dir)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "%-20s %-30s %-5s %-8s %-15s %s\n", "PERSONA_ID", "DISPLAY_NAME", "TIER", "BUDGET", "OWNER", "TENANT")
	fmt.Fprintln(w, strings.Repeat("-", 100))
	for _, c := range cards {
		tenant := c.Persona.TenantID
		if tenant == "" {
			tenant = "default"
		}
		fmt.Fprintf(w, "%-20s %-30s %-5d $%-7.2f %-15s %s\n",
			c.Persona.ID, c.Persona.DisplayName,
			c.Persona.DefaultTier, c.Persona.BudgetUSDDay, c.Persona.Owner, tenant)
	}
	return nil
}

func cmdValidate(w io.Writer, dir string) error {
	cards, err := LoadAll(dir)
	if err != nil {
		return err
	}
	for _, c := range cards {
		tenant := c.Persona.TenantID
		if tenant == "" {
			tenant = "default"
		}
		fmt.Fprintf(w, "OK   %s  (schema=%d, tier=%d, budget=$%.2f, tenant=%s)\n",
			c.Persona.ID, c.SchemaVersion, c.Persona.DefaultTier, c.Persona.BudgetUSDDay, tenant)
	}
	fmt.Fprintf(w, "\n%d persona(s) valid.\n", len(cards))
	return nil
}

func cmdCard(w io.Writer, dir, id string) error {
	cards, err := LoadAll(dir)
	if err != nil {
		return err
	}
	for _, c := range cards {
		if c.Persona.ID == id {
			data, _ := yaml.Marshal(c)
			fmt.Fprintln(w, string(data))
			return nil
		}
	}
	return fmt.Errorf("persona %q not found in %s", id, dir)
}

func cmdSchema(w io.Writer) {
	fmt.Fprintln(w, "schema_version: 1")
	fmt.Fprintln(w, "persona:")
	fmt.Fprintln(w, "  id: string  (required)")
	fmt.Fprintln(w, "  display_name: string")
	fmt.Fprintln(w, "  description: string")
	fmt.Fprintln(w, "  owner: string")
	fmt.Fprintln(w, "  home_repo: string")
	fmt.Fprintln(w, "  reviews_repos: []string")
	fmt.Fprintln(w, "  default_tier: int  (0..3)")
	fmt.Fprintln(w, "  budget_usd_per_day: float")
	fmt.Fprintln(w, "  tenant_id: string  (v18675-4; default shared-fleet)")
	fmt.Fprintln(w, "  tenant_isolation: enum  (v18675-4; per-persona-context|shared)")
	fmt.Fprintln(w, "skills:")
	fmt.Fprintln(w, "  bundle: path")
	fmt.Fprintln(w, "  required: []string")
	fmt.Fprintln(w, "hooks:")
	fmt.Fprintln(w, "  beforeSubmitPrompt: path")
	fmt.Fprintln(w, "  afterFileEdit: path")
	fmt.Fprintln(w, "pair_lock: filename")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fleet-orchestrator {list|validate|card|schema} [args]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dir := fs.String("dir", "personas", "personas root directory")
	switch cmd {
	case "list":
		_ = fs.Parse(os.Args[2:])
		if err := cmdList(os.Stdout, *dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "validate":
		_ = fs.Parse(os.Args[2:])
		if err := cmdValidate(os.Stdout, *dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "card":
		// Card takes a positional arg AFTER the flags. Allow either:
		//   fleet-orchestrator card --dir X id
		//   fleet-orchestrator card id --dir X
		// by splitting non-flag args.
		var positional []string
		args := os.Args[2:]
		var filtered []string
		for i := 0; i < len(args); i++ {
			a := args[i]
			if a == "--dir" && i+1 < len(args) {
				*dir = args[i+1]
				i++
				continue
			}
			if strings.HasPrefix(a, "--dir=") {
				*dir = strings.TrimPrefix(a, "--dir=")
				continue
			}
			filtered = append(filtered, a)
		}
		positional = filtered
		if len(positional) < 1 {
			fmt.Fprintln(os.Stderr, "usage: fleet-orchestrator card [--dir X] <persona-id>")
			os.Exit(2)
		}
		if err := cmdCard(os.Stdout, *dir, positional[0]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		_ = fs // unused in this branch
	case "schema":
		cmdSchema(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", cmd)
		os.Exit(2)
	}
}
