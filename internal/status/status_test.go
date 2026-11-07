package status

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bahiamovementcenter/xdg-agents/internal/layout"
)

func claudeLayout(t *testing.T) layout.Layout {
	t.Helper()
	l, err := layout.Load("../../agents/claude-code.ncl")
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// mkfs creates paths under root: a trailing / makes a folder.
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

func items(a Agent) map[string]Item {
	m := map[string]Item{}
	for _, it := range a.Items {
		m[it.Path] = it
	}
	return m
}

// A fake home with Claude Code's defaults: everything in ~/.claude
func TestDefaultHome(t *testing.T) {
	home := t.TempDir()
	mkfs(t, home,
		".claude/settings.json", ".claude/projects/", ".claude/cache/", ".claude/plugins/",
		".claude/.credentials.json", ".claude/todos/", ".claude/something-new.json", ".claude.json")
	env := map[string]string{"HOME": home}
	r := Compute([]layout.Layout{claudeLayout(t)}, func(k string) string { return env[k] }, Options{UID: 1000})
	if len(r.Agents) != 1 || !r.Agents[0].Installed {
		t.Fatalf("agents = %+v", r.Agents)
	}
	got := items(r.Agents[0])
	check := func(path string, state State, via, target string) {
		t.Helper()
		it, ok := got[path]
		if !ok {
			t.Errorf("%s missing", path)
			return
		}
		if it.State != state || (via != "" && it.Via != via) || (target != "" && it.Target != filepath.Join(home, target)) {
			t.Errorf("%s = %s via %q → %s; want %s via %q → ~/%s", path, it.State, it.Via, it.Target, state, via, target)
		}
	}
	check("settings.json", ToMove, "CLAUDE_CONFIG_DIR", ".config/claude/settings.json")
	check(".claude.json", ToMove, "CLAUDE_CONFIG_DIR", ".config/claude/.claude.json")
	check("projects", ToMove, "symlink", ".local/state/claude/projects")
	check("cache", ToMove, "symlink", ".cache/claude/cache")
	check("plugins", ToMove, "CLAUDE_CODE_PLUGIN_CACHE_DIR", ".local/share/claude/plugins")
	check(".credentials.json", Stays, "", ".config/claude/.credentials.json")
	check("todos", Legacy, "", "")
	check("something-new.json", Unclassified, "", "")
	if got[".claude.json"].Location != filepath.Join(home, ".claude.json") {
		t.Errorf(".claude.json found at %s, want ~/.claude.json while CLAUDE_CONFIG_DIR is unset", got[".claude.json"].Location)
	}
	if _, ok := got["agents"]; ok {
		t.Error("entries not on disk are listed without --all")
	}
	if !r.NeedsAttention() {
		t.Error("NeedsAttention = false with files to move")
	}
}

// After moving: CLAUDE_CONFIG_DIR at $XDG_CONFIG_HOME/claude, cache symlinked
// to its XDG place, projects symlinked somewhere else
func TestMovedHome(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config/claude")
	mkfs(t, home, ".config/claude/settings.json", ".config/claude/.claude.json", ".cache/claude/cache/", "elsewhere/projects/")
	if err := os.Symlink(filepath.Join(home, ".cache/claude/cache"), filepath.Join(cfg, "cache")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "elsewhere/projects"), filepath.Join(cfg, "projects")); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"HOME": home, "CLAUDE_CONFIG_DIR": cfg}
	r := Compute([]layout.Layout{claudeLayout(t)}, func(k string) string { return env[k] }, Options{})
	got := items(r.Agents[0])
	for path, want := range map[string]State{
		"settings.json": InPlace,
		".claude.json":  InPlace, // found inside CLAUDE_CONFIG_DIR, not in ~
		"cache":         InPlace, // a symlink to its target counts
		"projects":      SymlinkedElsewhere,
	} {
		if got[path].State != want {
			t.Errorf("%s = %s, want %s", path, got[path].State, want)
		}
	}
}

func TestNotInstalledIsSkipped(t *testing.T) {
	env := map[string]string{"HOME": t.TempDir()}
	r := Compute([]layout.Layout{claudeLayout(t)}, func(k string) string { return env[k] }, Options{})
	if len(r.Agents) != 0 {
		t.Errorf("agents = %+v, want none", r.Agents)
	}
	r = Compute([]layout.Layout{claudeLayout(t)}, func(k string) string { return env[k] }, Options{Named: true})
	if len(r.Agents) != 1 || r.Agents[0].Installed {
		t.Errorf("named but not installed: %+v", r.Agents)
	}
}

func TestTextPlain(t *testing.T) {
	home := t.TempDir()
	mkfs(t, home, ".claude/settings.json", ".claude/.credentials.json", ".claude.json")
	env := map[string]string{"HOME": home, "XDG_DATA_HOME": "relative/data"}
	r := Compute([]layout.Layout{claudeLayout(t)}, func(k string) string { return env[k] }, Options{})
	var b bytes.Buffer
	if err := Text(&b, r, home, true); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		`XDG_DATA_HOME ignored: "relative/data" isn't an absolute path`,
		"XDG_RUNTIME_DIR unset",
		"Claude Code — ~/.claude (CLAUDE_CONFIG_DIR unset)",
		"move      settings.json            → ~/.config/claude/settings.json  via CLAUDE_CONFIG_DIR",
		".credentials.json (sensitive)",
		"move      ~/.claude.json (sensitive)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
