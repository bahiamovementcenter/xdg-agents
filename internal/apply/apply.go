// Package apply carries out a plan (internal/plan), reversibly: every step
// is recorded in a journal as it's done, and Undo reverses a journal.
//
// Nothing is changed unless every check passes first: the agent isn't
// running, the plan's variables are already set to their planned values
// (so the agent finds its files the moment it next starts), every source
// exists, no target does, and every move stays on one filesystem. Moves
// are renames: a move to another filesystem is refused, never done as a
// copy and delete. Nothing is deleted or overwritten.
package apply

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/bahiamovementcenter/xdg-agents/internal/plan"
)

// Record is one change made, as written to the journal.
type Record struct {
	Kind string `json:"kind"` // mkdir, move, symlink
	Path string `json:"path,omitempty"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// Journal is what one apply did.
type Journal struct {
	Created string   `json:"created"`
	Agents  []string `json:"agents"`
	Records []Record `json:"records"`
	Undone  bool     `json:"undone,omitempty"`
}

// Options are what Apply needs from the outside, so tests can supply them.
type Options struct {
	Getenv func(string) string
	// Running returns which of the given program names are running
	Running func(names []string) []string
	// StateDir is where journals go: $XDG_STATE_HOME/xdg-agents
	StateDir string
	Now      func() time.Time
}

// ErrNothingToDo is returned when the plan has no steps.
var ErrNothingToDo = errors.New("nothing to do: everything that can move is in place")

// Check returns every reason the plan can't be applied now. An empty result
// means Apply would make every step. It changes nothing.
func Check(p plan.Plan, names map[string][]string, opt Options) []string {
	if p.Refused != "" {
		return []string{p.Refused}
	}
	var problems []string
	add := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	for _, a := range p.Agents {
		if len(a.Steps) == 0 {
			continue
		}
		if running := opt.Running(names[a.ID]); len(running) > 0 {
			add("%s is running (%s): quit it first", a.Name, strings.Join(running, ", "))
		}
		for _, e := range a.Env {
			if got := opt.Getenv(e.Var); got != e.Value {
				add("%s must already be set to %s where you run apply, so %s finds its files as soon as it next starts: add `export %s=%q` to your shell profile, open a new shell, and run apply again",
					e.Var, e.Value, a.Name, e.Var, e.Value)
			}
		}
	}

	// Simulate the steps in order: later steps name paths that earlier moves
	// create
	var fs virtualFS
	for _, a := range p.Agents {
		for _, s := range a.Steps {
			switch s.Kind {
			case plan.Move:
				if !fs.exists(s.From) {
					add("can't move %s: it isn't there", s.From)
					continue
				}
				if fs.exists(s.To) {
					add("can't move %s to %s: the target already exists", s.From, s.To)
					continue
				}
				if !fs.sameDevice(s.From, s.To) {
					add("can't move %s to %s: they're on different filesystems, and apply only renames", s.From, s.To)
					continue
				}
				fs.moves = append(fs.moves, [2]string{s.From, s.To})
			case plan.Symlink:
				if fs.exists(s.From) {
					add("can't create the symlink %s: something is already there", s.From)
					continue
				}
				if !fs.exists(s.To) {
					add("can't link %s to %s: the target isn't there", s.From, s.To)
					continue
				}
				fs.links = append(fs.links, s.From)
			}
		}
	}
	return problems
}

// Apply makes the plan's steps, writing each to a new journal as it's done,
// and returns the journal's path. It refuses, changing nothing, if Check
// finds any problem. On a failure partway, the journal holds what was done
// so far, and Undo reverses it.
func Apply(p plan.Plan, names map[string][]string, opt Options) (string, error) {
	if problems := Check(p, names, opt); len(problems) > 0 {
		return "", fmt.Errorf("nothing was changed:\n  - %s", strings.Join(problems, "\n  - "))
	}
	j := Journal{Created: opt.Now().UTC().Format(time.RFC3339)}
	steps := 0
	for _, a := range p.Agents {
		if len(a.Steps) > 0 {
			j.Agents = append(j.Agents, a.ID)
			steps += len(a.Steps)
		}
	}
	if steps == 0 {
		return "", ErrNothingToDo
	}
	if err := os.MkdirAll(opt.StateDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(opt.StateDir, "journal-"+opt.Now().UTC().Format("20060102T150405.000000000Z")+".json")
	if err := save(path, j); err != nil {
		return "", err
	}
	record := func(r Record) error {
		j.Records = append(j.Records, r)
		return save(path, j)
	}

	for _, a := range p.Agents {
		for _, s := range a.Steps {
			switch s.Kind {
			case plan.Move:
				if err := mkdirParents(filepath.Dir(s.To), record); err != nil {
					return path, err
				}
				if err := os.Rename(s.From, s.To); err != nil {
					return path, fmt.Errorf("moving %s: %w (what was done so far is in %s; `xdg-agents undo` reverses it)", s.From, err, path)
				}
				if err := record(Record{Kind: "move", From: s.From, To: s.To}); err != nil {
					return path, err
				}
			case plan.Symlink:
				if err := os.Symlink(s.To, s.From); err != nil {
					return path, fmt.Errorf("linking %s: %w (what was done so far is in %s; `xdg-agents undo` reverses it)", s.From, err, path)
				}
				if err := record(Record{Kind: "symlink", Path: s.From, To: s.To}); err != nil {
					return path, err
				}
			}
		}
	}
	return path, nil
}

// Undo reverses a journal, last change first. Each change is undone only if
// what's on disk still matches it; otherwise Undo stops and says why,
// leaving the rest for a person to look at.
func Undo(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var j Journal
	if err := json.Unmarshal(raw, &j); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if j.Undone {
		return fmt.Errorf("%s was already undone", path)
	}
	for i := len(j.Records) - 1; i >= 0; i-- {
		r := j.Records[i]
		switch r.Kind {
		case "symlink":
			t, err := os.Readlink(r.Path)
			if err != nil || t != r.To {
				return fmt.Errorf("not removing %s: it's no longer the symlink to %s that apply made", r.Path, r.To)
			}
			if err := os.Remove(r.Path); err != nil {
				return err
			}
		case "move":
			if _, err := os.Lstat(r.From); err == nil {
				return fmt.Errorf("not moving %s back: something is at %s again", r.To, r.From)
			}
			if err := os.Rename(r.To, r.From); err != nil {
				return fmt.Errorf("moving %s back to %s: %w", r.To, r.From, err)
			}
		case "mkdir":
			// Only if it's empty again: anything put there since stays
			_ = os.Remove(r.Path)
		}
		j.Records = j.Records[:i]
		if err := save(path, j); err != nil {
			return err
		}
	}
	j.Undone = true
	return save(path, j)
}

// Latest returns the newest journal in dir that hasn't been undone.
func Latest(dir string) (string, error) {
	files, _ := filepath.Glob(filepath.Join(dir, "journal-*.json"))
	sort.Strings(files)
	for i := len(files) - 1; i >= 0; i-- {
		raw, err := os.ReadFile(files[i])
		if err != nil {
			continue
		}
		var j Journal
		if json.Unmarshal(raw, &j) == nil && !j.Undone {
			return files[i], nil
		}
	}
	return "", fmt.Errorf("no journal to undo in %s", dir)
}

func mkdirParents(dir string, record func(Record) error) error {
	var missing []string
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil {
			break
		}
		missing = append(missing, d)
		if d == filepath.Dir(d) {
			break
		}
	}
	for i := len(missing) - 1; i >= 0; i-- {
		if err := os.Mkdir(missing[i], 0o700); err != nil {
			return err
		}
		if err := record(Record{Kind: "mkdir", Path: missing[i]}); err != nil {
			return err
		}
	}
	return nil
}

// save writes the journal by writing a new file and renaming it over the
// old, so a crash never leaves half a journal.
func save(path string, j Journal) error {
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// virtualFS answers "will this path exist?" partway through a plan, given
// the moves and symlinks before it.
type virtualFS struct {
	moves [][2]string // from, to, in order
	links []string
}

// physical returns where p is on disk before the plan, or "" if an earlier
// move has taken it away.
func (v *virtualFS) physical(p string) string {
	for i := len(v.moves) - 1; i >= 0; i-- {
		from, to := v.moves[i][0], v.moves[i][1]
		switch {
		case under(p, to):
			p = from + strings.TrimPrefix(p, to)
		case under(p, from):
			return ""
		}
	}
	return p
}

func (v *virtualFS) exists(p string) bool {
	for _, l := range v.links {
		if p == l {
			return true
		}
	}
	ph := v.physical(p)
	if ph == "" {
		return false
	}
	_, err := os.Lstat(ph)
	return err == nil
}

// sameDevice reports whether a rename from from to to would stay on one
// filesystem: from's device against that of to's nearest existing parent.
func (v *virtualFS) sameDevice(from, to string) bool {
	src, ok := device(v.physical(from))
	if !ok {
		return false
	}
	for d := filepath.Dir(to); ; d = filepath.Dir(d) {
		if ph := v.physical(d); ph != "" {
			if dev, ok := device(ph); ok {
				return dev == src
			}
		}
		if d == filepath.Dir(d) {
			return false
		}
	}
}

func device(p string) (uint64, bool) {
	fi, err := os.Lstat(p)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true //nolint:unconvert // Dev's type differs between platforms
}

func under(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}
