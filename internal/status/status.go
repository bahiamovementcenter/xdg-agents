// Package status works out, for each agent's layout, where its files are
// now, which XDG category each belongs to, where it should be, and how it
// can get there: the report docs/status.md describes.
//
// It looks only at names, types and symlink targets (Lstat, Readlink,
// ReadDir), never at what a file holds.
package status

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/bahiamovementcenter/xdg-agents/internal/layout"
	"github.com/bahiamovementcenter/xdg-agents/internal/xdg"
)

// State is what an entry needs, worked out in the order docs/status.md sets.
type State string

const (
	InPlace            State = "in_place"
	SymlinkedElsewhere State = "symlinked_elsewhere"
	Stays              State = "stays"
	Legacy             State = "legacy"
	ToMove             State = "to_move"
	Fixed              State = "fixed"
	NotAgents          State = "not_agents" // listed in the layout's untracked
	Unclassified       State = "unclassified"
)

// Item is one file or folder in the report.
type Item struct {
	Root      string       `json:"root"`     // the layout root it's under
	Path      string       `json:"path"`     // as written in the layout, or the name found on disk
	Location  string       `json:"location"` // absolute
	Kind      string       `json:"kind,omitempty"`
	Category  xdg.Category `json:"category,omitempty"`
	State     State        `json:"state"`
	Target    string       `json:"target,omitempty"`
	Via       string       `json:"via,omitempty"` // a variable, "symlink", or empty
	Reason    string       `json:"reason,omitempty"`
	Setting   string       `json:"setting,omitempty"` // a setting that can also move it
	Sensitive bool         `json:"sensitive,omitempty"`
	Present   bool         `json:"present"`
}

// RootInfo is where one of a layout's roots was found.
type RootInfo struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Env      string `json:"env,omitempty"`
	FromEnv  bool   `json:"from_env"`
	Exists   bool   `json:"exists"`
	Category string `json:"category"`
	// Where the root itself belongs: its category's XDG directory and the
	// agent's subfolder there (empty when that directory is unknown)
	Target string `json:"target,omitempty"`
	// The variable is set, but its folder doesn't exist yet and the files
	// are still at the default location: the state between setting the
	// variable and running apply
	EnvPending bool `json:"env_pending,omitempty"`
}

// Agent is one agent's part of the report.
type Agent struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Installed bool          `json:"installed"`
	Roots     []RootInfo    `json:"roots"`
	Items     []Item        `json:"items"`
	Summary   map[State]int `json:"summary"`
}

// Report is the whole status report.
type Report struct {
	XDG    xdg.Dirs `json:"xdg"`
	Agents []Agent  `json:"agents"`
}

// Options control what's included.
type Options struct {
	All   bool   // include layout entries that aren't on disk
	GOOS  string // "darwin" picks macOS defaults
	UID   int
	Named bool // agents were named on the command line: show them even if not installed
}

// Compute builds the report for the given layouts.
func Compute(layouts []layout.Layout, getenv func(string) string, opt Options) Report {
	dirs := xdg.Resolve(getenv)
	r := Report{XDG: dirs}
	for _, l := range layouts {
		a := agent(l, dirs, getenv, opt)
		if a.Installed || opt.Named {
			r.Agents = append(r.Agents, a)
		}
	}
	return r
}

type resolvedRoot struct {
	layout.Root
	name       string
	path       string
	fromEnv    bool
	envPending bool
	target     string
}

func agent(l layout.Layout, dirs xdg.Dirs, getenv func(string) string, opt Options) Agent {
	home := getenv("HOME")
	expand := func(p string) string {
		p = strings.ReplaceAll(p, "<uid>", strconv.Itoa(opt.UID))
		if rest, ok := strings.CutPrefix(p, "~/"); ok {
			return filepath.Join(home, rest)
		}
		return p
	}

	roots := map[string]resolvedRoot{}
	a := Agent{ID: l.ID, Name: l.Name, Summary: map[State]int{}}
	names := make([]string, 0, len(l.Roots))
	for n := range l.Roots {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		root := l.Roots[n]
		rr := resolvedRoot{Root: root, name: n}
		def := root.Default
		if opt.GOOS == "darwin" && root.DefaultMacOS != "" {
			def = root.DefaultMacOS
		}
		if v := getenv(root.Env); root.Env != "" && filepath.IsAbs(v) {
			rr.path, rr.fromEnv = filepath.Clean(v), true
			// Set ahead of the move: the files are still at the default
			if !exists(rr.path) && exists(expand(def)) {
				rr.path, rr.fromEnv, rr.envPending = expand(def), false, true
			}
		} else {
			rr.path = expand(def)
		}
		if d := dirs[root.Category]; d.Path != "" {
			rr.target = filepath.Join(d.Path, l.XDGName)
		}
		roots[n] = rr
		exists := exists(rr.path)
		// An agent is installed when a root in the home folder (or one its
		// variable points at) exists; system-wide roots alone don't count
		if exists && (rr.fromEnv || strings.HasPrefix(def, "~/")) {
			a.Installed = true
		}
		a.Roots = append(a.Roots, RootInfo{Name: n, Path: rr.path, Env: root.Env, FromEnv: rr.fromEnv, Exists: exists, Category: string(root.Category), Target: rr.target, EnvPending: rr.envPending})
	}

	settingFor := map[string]layout.Setting{}
	for _, s := range l.Settings {
		settingFor[s.Moves] = s
	}

	for _, e := range l.Paths {
		rr, ok := roots[e.Root]
		if !ok {
			continue
		}
		it := entryItem(e, rr, dirs, l.XDGName, expand)
		// A setting applies to the entry it moves, or the folder holding it
		// (autoMemoryDirectory moves projects/*/memory, inside projects/)
		for moves, s := range settingFor {
			if (moves == e.Path || strings.HasPrefix(moves, e.Path+"/")) && s.ForXDG != "no" {
				it.Setting = s.Name
			}
		}
		if !it.Present && !opt.All {
			continue
		}
		a.Items = append(a.Items, it)
	}
	a.Items = append(a.Items, unlisted(l, roots)...)
	for _, it := range a.Items {
		if it.Present {
			a.Summary[it.State]++
		}
	}
	return a
}

func entryItem(e layout.Entry, rr resolvedRoot, dirs xdg.Dirs, xdgName string, expand func(string) string) Item {
	loc := filepath.Join(rr.path, e.Path)
	if e.Path == "." {
		loc = rr.path
	}
	if e.DefaultPath != "" && !rr.fromEnv {
		loc = expand(e.DefaultPath)
	}
	it := Item{Root: rr.name, Path: e.Path, Location: loc, Category: e.Category, Sensitive: e.Sensitive, Kind: e.Kind}
	found := loc
	if strings.Contains(e.Path, "*") {
		if m, _ := filepath.Glob(loc); len(m) > 0 {
			found = m[0]
		}
	}
	fi, err := os.Lstat(found)
	it.Present = err == nil

	// Where it belongs: under its own category, except for what can only
	// move with its root, which goes where the root goes
	stays := e.Move == "with_root" && e.Category != rr.Category
	switch {
	case e.Move == "fixed":
	case stays:
		if rr.target != "" {
			it.Target = joinRel(rr.target, e.Path)
		}
	default:
		if d := dirs[e.Category]; d.Path != "" {
			it.Target = joinRel(filepath.Join(d.Path, xdgName), e.Path)
		}
	}

	switch {
	case it.Present && it.Target != "" && sameFile(found, it.Target):
		it.State = InPlace
	case it.Present && fi.Mode()&os.ModeSymlink != 0:
		it.State = SymlinkedElsewhere
		if t, err := os.Readlink(found); err == nil {
			it.Reason = "points to " + t
		}
	case stays:
		it.State = Stays
		it.Reason = "can only move with its root"
		if e.Kind == "file" {
			it.Reason = "a file can only move with its whole root"
		}
		if rr.Env != "" {
			it.Via = rr.Env
		}
	case e.Legacy:
		it.State = Legacy
		it.Reason = "no longer written; safe to delete"
	case e.Move == "fixed":
		it.State = Fixed
		it.Reason = "the agent only looks here"
	default:
		it.State = ToMove
		switch e.Move {
		case "with_root":
			it.Via = rr.Env
		case "env":
			it.Via = e.Env
		case "symlink":
			it.Via = "symlink"
		}
		if it.Target == "" {
			it.Reason = "no " + string(e.Category) + " directory (" + dirs[e.Category].Var + " is unset)"
		}
	}
	return it
}

// unlisted reports what's on disk directly in each root (and in its own
// default locations) that no entry or untracked path covers.
func unlisted(l layout.Layout, roots map[string]resolvedRoot) []Item {
	var items []Item
	for _, n := range sortedKeys(roots) {
		rr := roots[n]
		covered := map[string]bool{}
		whole := false
		for _, e := range l.Paths {
			if e.Root != n {
				continue
			}
			if e.Path == "." {
				whole = true
			}
			first, _, _ := strings.Cut(e.Path, "/")
			covered[first] = true
		}
		var untracked []string
		for _, u := range l.Untracked {
			first, _, _ := strings.Cut(u.Path, "/")
			untracked = append(untracked, first)
		}
		if whole {
			continue
		}
		entries, err := os.ReadDir(rr.path)
		if err != nil {
			continue
		}
		for _, d := range entries {
			name := d.Name()
			if covered[name] {
				continue
			}
			it := Item{Root: n, Path: name, Location: filepath.Join(rr.path, name), Present: true, State: Unclassified}
			switch {
			case d.Type()&os.ModeSymlink != 0:
				it.Kind = "symlink"
			case d.IsDir():
				it.Kind = "dir"
			default:
				it.Kind = "file"
			}
			for _, u := range untracked {
				if u == name {
					it.State = NotAgents
				}
			}
			items = append(items, it)
		}
	}
	return items
}

func joinRel(base, rel string) string {
	if rel == "." {
		return base
	}
	return filepath.Join(base, rel)
}

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// sameFile reports whether p is target, or a symlink that resolves to it.
func sameFile(p, target string) bool {
	if filepath.Clean(p) == filepath.Clean(target) {
		return true
	}
	rp, err1 := filepath.EvalSymlinks(p)
	rt, err2 := filepath.EvalSymlinks(target)
	return err1 == nil && err2 == nil && rp == rt
}

func sortedKeys(m map[string]resolvedRoot) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NeedsAttention reports whether anything shown is out of place: what
// --check exits 1 for.
func (r Report) NeedsAttention() bool {
	for _, a := range r.Agents {
		for _, it := range a.Items {
			if it.Present && (it.State == ToMove || it.State == SymlinkedElsewhere || it.State == Unclassified) {
				return true
			}
		}
	}
	return false
}
