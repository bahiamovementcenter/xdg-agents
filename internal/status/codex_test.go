package status

import (
	"path/filepath"
	"testing"

	"github.com/bahiamovementcenter/xdg-agents/internal/layout"
)

// Codex's defaults: everything in ~/.codex, which CODEX_HOME moves whole
func TestCodexDefaultHome(t *testing.T) {
	l, err := layout.Load("../../agents/codex.ncl")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	mkfs(t, home, ".codex/config.toml", ".codex/work.config.toml", ".codex/auth.json",
		".codex/history.jsonl", ".codex/sessions/", ".codex/log/", ".codex/packages/standalone/",
		".codex/state_5.sqlite")
	env := map[string]string{"HOME": home}
	r := Compute([]layout.Layout{l}, func(k string) string { return env[k] }, Options{UID: 1000})
	a := r.Agents[0]
	if !a.Installed {
		t.Fatal("codex not found installed")
	}
	got := items(a)
	for path, want := range map[string]struct {
		state  State
		target string
	}{
		"config.toml":         {ToMove, ".config/codex/config.toml"},
		"*.config.toml":       {ToMove, ".config/codex/*.config.toml"},
		"auth.json":           {Stays, ".config/codex/auth.json"},
		"history.jsonl":       {Stays, ".config/codex/history.jsonl"},
		"sessions":            {ToMove, ".local/state/codex/sessions"},
		"log":                 {ToMove, ".local/state/codex/log"},
		"packages/standalone": {ToMove, ".cache/codex/packages/standalone"},
	} {
		it, ok := got[path]
		if !ok {
			t.Errorf("%s missing", path)
			continue
		}
		if it.State != want.state || it.Target != filepath.Join(home, want.target) {
			t.Errorf("%s: %s → %s, want %s → %s", path, it.State, it.Target, want.state, want.target)
		}
	}
	// The profile matches its pattern; the undocumented database doesn't
	if it, ok := got["work.config.toml"]; ok {
		t.Errorf("profile reported on its own: %+v", it)
	}
	if it := got["state_5.sqlite"]; it.State != Unclassified {
		t.Errorf("state_5.sqlite = %+v, want unclassified", it)
	}
	if got["log"].Setting != "log_dir" {
		t.Errorf("log's setting = %q", got["log"].Setting)
	}
}
