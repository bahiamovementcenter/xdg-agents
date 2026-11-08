package plan

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bahiamovementcenter/xdg-agents/internal/layout"
	"github.com/bahiamovementcenter/xdg-agents/internal/status"
)

func report(t *testing.T, env map[string]string) status.Report {
	t.Helper()
	l, err := layout.Load("../../agents/claude-code.ncl")
	if err != nil {
		t.Fatal(err)
	}
	return status.Compute([]layout.Layout{l}, func(k string) string { return env[k] }, status.Options{UID: 1000})
}

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
		if err := os.WriteFile(full, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanDefaultHome(t *testing.T) {
	home := t.TempDir()
	mkfs(t, home, ".claude/settings.json", ".claude/projects/", ".claude/plugins/",
		".claude/.credentials.json", ".claude/todos/", ".claude/mystery/", ".claude.json")
	env := map[string]string{"HOME": home, "XDG_RUNTIME_DIR": filepath.Join(home, "run")}
	p := Build(report(t, env), "linux")
	if len(p.Agents) != 1 {
		t.Fatalf("agents = %+v", p.Agents)
	}
	a := p.Agents[0]
	h := func(rel string) string { return filepath.Join(home, rel) }
	want := []Step{
		{Kind: Move, From: h(".claude"), To: h(".config/claude")},
		{Kind: Move, From: h(".claude.json"), To: h(".config/claude/.claude.json")},
	}
	if len(a.Steps) < 2 || a.Steps[0] != want[0] || a.Steps[1] != want[1] {
		t.Fatalf("first steps = %+v, want the root then ~/.claude.json", a.Steps)
	}
	has := func(steps []Step, s Step) bool {
		for _, x := range steps {
			if x == s {
				return true
			}
		}
		return false
	}
	// Folders are symlinked out from the root's new place
	for _, s := range []Step{
		{Kind: Move, From: h(".config/claude/projects"), To: h(".local/state/claude/projects")},
		{Kind: Symlink, From: h(".config/claude/projects"), To: h(".local/state/claude/projects")},
		{Kind: Move, From: h(".config/claude/plugins"), To: h(".local/share/claude/plugins")},
	} {
		if !has(a.Steps, s) {
			t.Errorf("steps lack %+v:\n%+v", s, a.Steps)
		}
	}
	for _, e := range []Step{
		{Kind: SetEnv, Var: "CLAUDE_CONFIG_DIR", Value: h(".config/claude")},
		{Kind: SetEnv, Var: "CLAUDE_CODE_PLUGIN_CACHE_DIR", Value: h(".local/share/claude/plugins")},
	} {
		if !has(a.Env, e) {
			t.Errorf("env lacks %+v: %+v", e, a.Env)
		}
	}
	notes := map[string]string{}
	for _, n := range a.Notes {
		notes[strings.TrimPrefix(n.Path, h(".config/claude")+"/")] = n.Reason
	}
	for path, want := range map[string]string{
		".credentials.json": "can only move with its whole root",
		"todos":             "safe to delete",
		"mystery":           "it moves with its root, so check what it is first",
		"projects":          "autoMemoryDirectory",
	} {
		if !strings.Contains(notes[path], want) {
			t.Errorf("note for %s = %q, want it to mention %q", path, notes[path], want)
		}
	}
}

// With the root already where it belongs, only what's left is planned
func TestPlanAfterRootMoved(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config/claude")
	mkfs(t, home, ".config/claude/settings.json", ".config/claude/cache/")
	env := map[string]string{"HOME": home, "CLAUDE_CONFIG_DIR": cfg}
	a := Build(report(t, env), "linux").Agents[0]
	want := []Step{
		{Kind: Move, From: filepath.Join(cfg, "cache"), To: filepath.Join(home, ".cache/claude/cache")},
		{Kind: Symlink, From: filepath.Join(cfg, "cache"), To: filepath.Join(home, ".cache/claude/cache")},
	}
	if len(a.Steps) != 2 || a.Steps[0] != want[0] || a.Steps[1] != want[1] {
		t.Errorf("steps = %+v, want only cache's move and symlink", a.Steps)
	}
	for _, e := range a.Env {
		if e.Var == "CLAUDE_CONFIG_DIR" {
			t.Errorf("CLAUDE_CONFIG_DIR planned again: %+v", e)
		}
	}
}

// A target that already exists is never overwritten
func TestPlanTargetExists(t *testing.T) {
	home := t.TempDir()
	mkfs(t, home, ".claude/settings.json", ".config/claude/other")
	a := Build(report(t, map[string]string{"HOME": home}), "linux").Agents[0]
	for _, s := range a.Steps {
		if s.Kind == Move && s.From == filepath.Join(home, ".claude") {
			t.Fatalf("plans to move onto an existing folder: %+v", s)
		}
	}
	found := false
	for _, n := range a.Notes {
		found = found || strings.Contains(n.Reason, "already exists")
	}
	if !found {
		t.Errorf("no note about the existing target: %+v", a.Notes)
	}
}

func TestPlanRefusedOnMacOS(t *testing.T) {
	home := t.TempDir()
	mkfs(t, home, ".claude/settings.json")
	p := Build(report(t, map[string]string{"HOME": home}), "darwin")
	if p.Refused == "" || len(p.Agents) != 0 {
		t.Errorf("macOS plan = %+v, want refused (ADR-0004)", p)
	}
}

func TestText(t *testing.T) {
	home := t.TempDir()
	mkfs(t, home, ".claude/settings.json", ".claude/cache/")
	p := Build(report(t, map[string]string{"HOME": home}), "linux")
	var b bytes.Buffer
	if err := Text(&b, p, home); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"Plan for Claude Code (nothing has been changed)",
		"Quit Claude Code first.",
		"1. move  ~/.claude → ~/.config/claude",
		"link  ~/.config/claude/cache → ~/.cache/claude/cache",
		`export CLAUDE_CONFIG_DIR="$HOME/.config/claude"`,
		"isn't built yet",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
