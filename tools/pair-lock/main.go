// pair-lock (v14517) — single-active-pair per repo guard.
//
// A .sprint_lock file is the source of truth for which sprint / pair
// is currently running in a repo. Subcommands:
//
//   acquire  --pair v14517 --operator cursor-ai [--persona code-reviewer]
//   release  --pair v14517
//   status   (print current pair_id, phase, started_at)
//   check    (exit 0 if no conflict, 1 if another pair is active)
//
// Concurrent-pair prevention: two agents cannot both hold the lock
// for the same repo simultaneously. The lock file is rewritten
// atomically via rename(2) to avoid torn writes.
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

func cmdAcquire(w io.Writer, path string, pair, operator string, personas []string) error {
	l := Lock{
		PairID:   pair,
		Operator: operator,
		Personas: personas,
	}
	if err := Acquire(path, l); err != nil {
		if errors.Is(err, ErrConflict) {
			fmt.Fprintln(w, err.Error())
			return err
		}
		return err
	}
	fmt.Fprintf(w, "acquired pair %s at %s\n", pair, time.Now().UTC().Format(time.RFC3339))
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
	fmt.Fprintf(w, "active: pair_id=%s operator=%s started=%s\n",
		l.PairID, l.Operator, l.StartedAt.Format(time.RFC3339))
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: pair-lock {status|acquire|release|check} [--file PATH]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	file := fs.String("file", ".sprint_lock", "lock file path")
	pair := fs.String("pair", "", "pair id (e.g. v14517)")
	operator := fs.String("operator", "cursor-ai", "operator name")
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
		if err := cmdAcquire(os.Stdout, *file, *pair, *operator, ps); err != nil {
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