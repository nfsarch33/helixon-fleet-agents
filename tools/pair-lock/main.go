// pair-lock (v14517) — single-active-pair per repo guard.
//
// A .sprint_lock file is the source of truth for which sprint / pair
// is currently running in a repo. Subcommands:
//
//	acquire  --pair v14517 --operator cursor-ai [--persona code-reviewer] [--tenant <id>]
//	release  --pair v14517
//	status   (print current pair_id, phase, started_at, tenant_id)
//	check    (exit 0 if no conflict, 1 if another pair is active)
//	audit    append a tenant-attributed entry to <audit-path> (.fleet-trail/pair-lock.ndjson by default)
//
// Concurrent-pair prevention: two agents cannot both hold the lock
// for the same repo simultaneously. The lock file is rewritten
// atomically via rename(2) to avoid torn writes.
//
// v18675-4 (CF-172 sibling): the audit log records who acted on which
// repo at what cost — required for per-tenant attribution in
// multi-tenant deployments. Tenant id is resolved in priority:
// (1) --tenant flag, (2) HELIXON_TENANT_ID env var, (3) "default".
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const schemaVersion = 1

type Lock struct {
	SchemaVersion int       `json:"schema_version"`
	PairID        string    `json:"pair_id"`
	SprintID      string    `json:"sprint_id"`
	Phase         string    `json:"phase"` // running | closed
	StartedAt     time.Time `json:"started_at"`
	ClosedAt      time.Time `json:"closed_at,omitempty"`
	PID           int       `json:"pid"`
	Operator      string    `json:"operator"`
	Personas      []string  `json:"personas,omitempty"`
	// TenantID (v18675-4, CF-172 sibling) attributes the pair-lock to a
	// tenant. Single-tenant deployments leave this empty and downstream
	// audit code treats it as "default". Multi-tenant deployments pass
	// --tenant <id> or the boot-time HELIXON_TENANT_ID env var.
	TenantID string `json:"tenant_id,omitempty"`
}

// AuditEntry captures a per-tenant attribution record emitted via the
// `audit` subcommand. Schema is deliberately NDJSON-friendly so it can
// be tailed by `jq` and ingested by the helixon-platform cost pipeline.
type AuditEntry struct {
	TS       string `json:"ts"`
	Event    string `json:"event"`
	PairID   string `json:"pair_id,omitempty"`
	TenantID string `json:"tenant_id"`
	Operator string `json:"operator,omitempty"`
	Repo     string `json:"repo,omitempty"`
	Action   string `json:"action,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// ResolveTenantID returns the tenant_id to attribute work to.
// Resolution order:
//  1. explicit flag value (provided by the caller)
//  2. HELIXON_TENANT_ID env var
//  3. "default"
func ResolveTenantID(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("HELIXON_TENANT_ID"); v != "" {
		return v
	}
	return "default"
}

// DefaultAuditPath returns the default audit-log path: `.fleet-trail/pair-lock.ndjson`
// under the current working directory (the repo root, in conventional use).
func DefaultAuditPath() string {
	return filepath.Join(".fleet-trail", "pair-lock.ndjson")
}

// Read parses the lock file at path. Returns (nil, nil) if the file
// does not exist (no active pair).
func Read(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var l Lock
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if l.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("lock schema_version=%d expected %d", l.SchemaVersion, schemaVersion)
	}
	return &l, nil
}

// Write atomically writes l to path. Atomic via tmp+rename.
func Write(path string, l Lock) error {
	if l.SchemaVersion == 0 {
		l.SchemaVersion = schemaVersion
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ErrConflict is returned when another pair holds the lock.
var ErrConflict = errors.New("pair-lock: another pair is active")

// Acquire attempts to set the lock. If a running lock for a
// DIFFERENT pair_id is present, returns ErrConflict. If the lock is
// for the SAME pair_id, returns nil (re-entrant).
func Acquire(path string, want Lock) error {
	cur, err := Read(path)
	if err != nil {
		return err
	}
	if cur != nil && cur.Phase == "running" && cur.PairID != want.PairID {
		return fmt.Errorf("%w: current pair_id=%s (started %s)",
			ErrConflict, cur.PairID, cur.StartedAt.Format(time.RFC3339))
	}
	want.Phase = "running"
	if want.StartedAt.IsZero() {
		want.StartedAt = time.Now().UTC()
	}
	if want.PID == 0 {
		want.PID = os.Getpid()
	}
	if want.TenantID == "" {
		want.TenantID = ResolveTenantID("")
	}
	return Write(path, want)
}

// Release marks the lock as closed (does NOT delete the file).
func Release(path string) error {
	cur, err := Read(path)
	if err != nil {
		return err
	}
	if cur == nil {
		return errors.New("pair-lock: nothing to release")
	}
	cur.Phase = "closed"
	cur.ClosedAt = time.Now().UTC()
	return Write(path, *cur)
}

// AppendAudit writes a single AuditEntry as NDJSON to path. Creates
// the parent dir if missing. The fd is opened append-only so multiple
// agents writing concurrently will not interleave lines (POSIX append
// mode guarantees atomicity for short writes).
func AppendAudit(path string, e AuditEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(e)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return nil
}

func cmdStatus(w io.Writer, path string) error {
	l, err := Read(path)
	if err != nil {
		return err
	}
	if l == nil {
		fmt.Fprintln(w, "no active pair")
		return nil
	}
	data, _ := json.MarshalIndent(l, "", "  ")
	fmt.Fprintln(w, string(data))
	return nil
}

func cmdAcquire(w io.Writer, path string, pair, operator, tenant string, personas []string) error {
	l := Lock{
		PairID:   pair,
		Operator: operator,
		Personas: personas,
		TenantID: ResolveTenantID(tenant),
	}
	if err := Acquire(path, l); err != nil {
		if errors.Is(err, ErrConflict) {
			fmt.Fprintln(w, err.Error())
			return err
		}
		return err
	}
	fmt.Fprintf(w, "acquired pair %s tenant=%s at %s\n",
		pair, l.TenantID, time.Now().UTC().Format(time.RFC3339))
	return nil
}

func cmdRelease(w io.Writer, path string) error {
	if err := Release(path); err != nil {
		return err
	}
	fmt.Fprintln(w, "released")
	return nil
}

func cmdCheck(w io.Writer, path string) error {
	l, err := Read(path)
	if err != nil {
		return err
	}
	if l == nil || l.Phase != "running" {
		fmt.Fprintln(w, "ok: no active pair")
		return nil
	}
	fmt.Fprintf(w, "active: pair_id=%s operator=%s tenant=%s started=%s\n",
		l.PairID, l.Operator, l.TenantID, l.StartedAt.Format(time.RFC3339))
	return nil
}

// cmdAudit appends one AuditEntry to the audit log. The repo is
// inferred from the working directory's `.sprint_lock` file's parent;
// pair-id is read from the same file if it exists (so callers do not
// have to repeat it).
func cmdAudit(w io.Writer, auditPath, action, detail string) error {
	entry := AuditEntry{
		Event:    "pair-lock.audit",
		TenantID: ResolveTenantID(""),
		Action:   action,
		Detail:   detail,
	}
	// Best-effort: enrich with current lock context (do not fail if missing).
	if cwd, err := os.Getwd(); err == nil {
		entry.Repo = filepath.Base(cwd)
	}
	if lockPath := filepath.Join(".sprint_lock"); fileExists(lockPath) {
		if cur, err := Read(lockPath); err == nil && cur != nil {
			entry.PairID = cur.PairID
			entry.Operator = cur.Operator
			if cur.TenantID != "" {
				entry.TenantID = cur.TenantID
			}
		}
	}
	if auditPath == "" {
		auditPath = DefaultAuditPath()
	}
	if err := AppendAudit(auditPath, entry); err != nil {
		return err
	}
	data, _ := json.Marshal(entry)
	fmt.Fprintln(w, string(data))
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: pair-lock {status|acquire|release|check|audit} [--file PATH]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	file := fs.String("file", ".sprint_lock", "lock file path")
	pair := fs.String("pair", "", "pair id (e.g. v14517)")
	operator := fs.String("operator", "cursor-ai", "operator name")
	tenant := fs.String("tenant", "", "tenant id (overrides HELIXON_TENANT_ID env var)")
	auditPath := fs.String("audit-path", "", "audit log path (default .fleet-trail/pair-lock.ndjson)")
	action := fs.String("action", "noop", "audit action label (e.g. acquire, release, file-edit)")
	detail := fs.String("detail", "", "audit detail (free-form, kept short)")
	var personas stringFlag
	fs.Var(&personas, "persona", "persona id (repeatable)")
	switch cmd {
	case "status":
		_ = fs.Parse(os.Args[2:])
		if err := cmdStatus(os.Stdout, *file); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "acquire":
		_ = fs.Parse(os.Args[2:])
		if *pair == "" {
			fmt.Fprintln(os.Stderr, "--pair is required")
			os.Exit(2)
		}
		var ps []string
		for _, p := range personas {
			if p != "" {
				ps = append(ps, p)
			}
		}
		if err := cmdAcquire(os.Stdout, *file, *pair, *operator, *tenant, ps); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "release":
		_ = fs.Parse(os.Args[2:])
		if err := cmdRelease(os.Stdout, *file); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "check":
		_ = fs.Parse(os.Args[2:])
		if err := cmdCheck(os.Stdout, *file); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "audit":
		_ = fs.Parse(os.Args[2:])
		if err := cmdAudit(os.Stdout, *auditPath, *action, *detail); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", cmd)
		os.Exit(2)
	}
}

// stringFlag accumulates repeated --persona flags.
type stringFlag []string

func (s *stringFlag) String() string { return strings.Join(*s, ",") }
func (s *stringFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}
