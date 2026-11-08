package apply

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bahiamovementcenter/xdg-agents/internal/layout"
	"github.com/bahiamovementcenter/xdg-agents/internal/plan"
	"github.com/bahiamovementcenter/xdg-agents/internal/status"
)

func mkfs(t *testing.T, root string, paths ...string) {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, p)
		if strings.HasSuffix(p, "/") {
			if err := os.MkdirAll(full, 0o700); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// tree lists everything under root: path, kind and symlink target, never
// contents.
func tree(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == root {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(p)
			out = append(out, rel+" -> "+target)
		case d.IsDir():
			out = append(out, rel+"/")
		default:
			out = append(out, rel)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

type fixture struct {
	home string
	env  map[string]string
	opt  Options
}

func setup(t *testing.T) fixture {
	t.Helper()
	home := t.TempDir()
	mkfs(t, home, ".claude/settings.json", ".claude/projects/p/s.jsonl", ".claude/cache/c",
		".claude/plugins/x", ".claude/.credentials.json", ".claude.json")
	env := map[string]string{
		"HOME":                         home,
		"XDG_RUNTIME_DIR":              filepath.Join(home, "run"),
		"CLAUDE_CONFIG_DIR":            filepath.Join(home, ".config/claude"),
		"CLAUDE_CODE_PLUGIN_CACHE_DIR": filepath.Join(home, ".local/share/claude/plugins"),
	}
	n := 0
	return fixture{home: home, env: env, opt: Options{
		Getenv:   func(k string) string { return env[k] },
		Running:  func([]string) []string { return nil },
		StateDir: filepath.Join(home, ".local/state/xdg-agents"),
		Now:      func() time.Time { n++; return time.Date(2026, 11, 7, 12, 0, n, 0, time.UTC) },
	}}
}

func (f fixture) plan(t *testing.T) plan.Plan {
	t.Helper()
	l, err := layout.Load("../../agents/claude-code.ncl")
	if err != nil {
		t.Fatal(err)
	}
	r := status.Compute([]layout.Layout{l}, f.opt.Getenv, status.Options{UID: 1000})
	return plan.Build(r, "linux")
}

var names = map[string][]string{"claude-code": {"claude-code", "claude"}}

func TestApplyThenUndo(t *testing.T) {
	f := setup(t)
	before := tree(t, f.home)
	p := f.plan(t)
	if problems := Check(p, names, f.opt); len(problems) > 0 {
		t.Fatalf("check: %v", problems)
	}
	journal, err := Apply(p, names, f.opt)
	if err != nil {
		t.Fatal(err)
	}
	h := func(rel string) string { return filepath.Join(f.home, rel) }
	for _, path := range []string{".config/claude/settings.json", ".config/claude/.claude.json",
		".local/state/claude/projects/p/s.jsonl", ".cache/claude/cache/c", ".local/share/claude/plugins/x"} {
		if _, err := os.Stat(h(path)); err != nil {
			t.Errorf("after apply, %s is missing: %v", path, err)
		}
	}
	for _, gone := range []string{".claude", ".claude.json"} {
		if _, err := os.Lstat(h(gone)); err == nil {
			t.Errorf("after apply, ~/%s is still there", gone)
		}
	}
	if target, err := os.Readlink(h(".config/claude/projects")); err != nil || target != h(".local/state/claude/projects") {
		t.Errorf("projects symlink: %q %v", target, err)
	}

	// status now finds things in place
	l, _ := layout.Load("../../agents/claude-code.ncl")
	r := status.Compute([]layout.Layout{l}, f.opt.Getenv, status.Options{UID: 1000})
	if n := r.Agents[0].Summary[status.ToMove]; n != 0 {
		t.Errorf("after apply, status still has %d to move: %+v", n, r.Agents[0].Items)
	}

	if err := Undo(journal); err != nil {
		t.Fatal(err)
	}
	after := tree(t, f.home)
	// Undo leaves only the journal folder behind
	var trimmed []string
	for _, p := range after {
		if !strings.HasPrefix(p, ".local/state/xdg-agents") && p != ".local/state/" && p != ".local/" {
			trimmed = append(trimmed, p)
		}
	}
	if !reflect.DeepEqual(trimmed, before) {
		t.Errorf("undo didn't restore the home folder:\nbefore %v\nafter  %v", before, trimmed)
	}
	if err := Undo(journal); err == nil {
		t.Error("undoing twice must fail")
	}
}

// Without the variables set, nothing is touched
func TestRefusesWithoutVariables(t *testing.T) {
	f := setup(t)
	delete(f.env, "CLAUDE_CONFIG_DIR")
	before := tree(t, f.home)
	_, err := Apply(f.plan(t), names, f.opt)
	if err == nil || !strings.Contains(err.Error(), "CLAUDE_CONFIG_DIR must already be set") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(tree(t, f.home), before) {
		t.Error("files changed although apply refused")
	}
}

func TestRefusesWhileRunning(t *testing.T) {
	f := setup(t)
	f.opt.Running = func(n []string) []string { return []string{"claude"} }
	before := tree(t, f.home)
	_, err := Apply(f.plan(t), names, f.opt)
	if err == nil || !strings.Contains(err.Error(), "Claude Code is running") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(tree(t, f.home), before) {
		t.Error("files changed although apply refused")
	}
}

// A source that vanished between plan and apply stops everything
func TestRefusesMissingSource(t *testing.T) {
	f := setup(t)
	p := f.plan(t)
	if err := os.RemoveAll(filepath.Join(f.home, ".claude.json")); err != nil {
		t.Fatal(err)
	}
	before := tree(t, f.home)
	if _, err := Apply(p, names, f.opt); err == nil || !strings.Contains(err.Error(), "isn't there") {
		t.Fatalf("err = %v", err)
	}
	if !reflect.DeepEqual(tree(t, f.home), before) {
		t.Error("files changed although apply refused")
	}
}

// Undo stops rather than remove a symlink someone has changed since
func TestUndoStopsOnChangedSymlink(t *testing.T) {
	f := setup(t)
	journal, err := Apply(f.plan(t), names, f.opt)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.home, ".config/claude/cache")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/somewhere/else", link); err != nil {
		t.Fatal(err)
	}
	if err := Undo(journal); err == nil || !strings.Contains(err.Error(), "no longer the symlink") {
		t.Errorf("err = %v", err)
	}
	if target, _ := os.Readlink(link); target != "/somewhere/else" {
		t.Errorf("undo touched the changed symlink: %s", target)
	}
}

func TestLatest(t *testing.T) {
	f := setup(t)
	journal, err := Apply(f.plan(t), names, f.opt)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Latest(f.opt.StateDir)
	if err != nil || got != journal {
		t.Errorf("Latest = %s, %v; want %s", got, err, journal)
	}
	if err := Undo(journal); err != nil {
		t.Fatal(err)
	}
	if _, err := Latest(f.opt.StateDir); err == nil {
		t.Error("Latest found an undone journal")
	}
}

func TestNothingToDo(t *testing.T) {
	f := setup(t)
	if _, err := Apply(plan.Plan{Agents: []plan.Agent{{ID: "claude-code"}}}, names, f.opt); err != ErrNothingToDo {
		t.Errorf("err = %v", err)
	}
}

// Codex goes through the same apply and undo as Claude Code
func TestCodexApplyThenUndo(t *testing.T) {
	home := t.TempDir()
	mkfs(t, home, ".codex/config.toml", ".codex/auth.json", ".codex/sessions/2026/s.jsonl",
		".codex/log/codex-tui.log", ".codex/packages/standalone/x")
	env := map[string]string{"HOME": home, "CODEX_HOME": filepath.Join(home, ".config/codex")}
	n := 0
	opt := Options{
		Getenv:   func(k string) string { return env[k] },
		Running:  func([]string) []string { return nil },
		StateDir: filepath.Join(home, ".local/state/xdg-agents"),
		Now:      func() time.Time { n++; return time.Date(2026, 11, 8, 12, 0, n, 0, time.UTC) },
	}
	l, err := layout.Load("../../agents/codex.ncl")
	if err != nil {
		t.Fatal(err)
	}
	before := tree(t, home)
	p := plan.Build(status.Compute([]layout.Layout{l}, opt.Getenv, status.Options{UID: 1000}), "linux")
	names := map[string][]string{"codex": {"codex"}}
	journal, err := Apply(p, names, opt)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{".config/codex/config.toml", ".config/codex/auth.json",
		".local/state/codex/sessions/2026/s.jsonl", ".local/state/codex/log/codex-tui.log", ".cache/codex/packages/standalone/x",
		".config/codex/sessions/2026/s.jsonl", ".config/codex/packages/standalone/x"} {
		if _, err := os.Stat(filepath.Join(home, path)); err != nil {
			t.Errorf("after apply, %s is missing: %v", path, err)
		}
	}
	if err := Undo(journal); err != nil {
		t.Fatal(err)
	}
	var trimmed []string
	for _, p := range tree(t, home) {
		if !strings.HasPrefix(p, ".local/state/xdg-agents") && p != ".local/state/" && p != ".local/" {
			trimmed = append(trimmed, p)
		}
	}
	if !reflect.DeepEqual(trimmed, before) {
		t.Errorf("undo didn't restore the home folder:\nbefore %v\nafter  %v", before, trimmed)
	}
}
