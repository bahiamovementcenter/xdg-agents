package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHome makes a temporary HOME holding Claude Code's default files.
func fakeHome(t *testing.T) map[string]string {
	t.Helper()
	home := t.TempDir()
	for _, p := range []string{".claude/projects", ".claude/cache"} {
		if err := os.MkdirAll(filepath.Join(home, p), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{".claude/settings.json", ".claude.json"} {
		if err := os.WriteFile(filepath.Join(home, p), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return map[string]string{"HOME": home}
}

func runWith(t *testing.T, env map[string]string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut, func(k string) string { return env[k] })
	return code, out.String(), errOut.String()
}

// The first done check in AGENTS.md: status on a machine with Claude Code,
// no flags, lists each file with its category, target, and how to move it
func TestStatusDoneCheck(t *testing.T) {
	env := fakeHome(t)
	code, out, errOut := runWith(t, env, "status")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"Claude Code — ~/.claude (CLAUDE_CONFIG_DIR unset)",
		"  config\n",
		"settings.json            → ~/.config/claude/settings.json  via CLAUDE_CONFIG_DIR",
		"  state\n",
		"projects/ (sensitive)    → ~/.local/state/claude/projects  via symlink",
		"cache/                   → ~/.cache/claude/cache  via symlink",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Claude desktop app") {
		t.Error("an agent that isn't installed was listed")
	}
}

func TestStatusCheckAndJSON(t *testing.T) {
	env := fakeHome(t)
	if code, _, _ := runWith(t, env, "status", "--check"); code != 1 {
		t.Errorf("--check with files to move: exit %d, want 1", code)
	}
	code, out, _ := runWith(t, env, "status", "--json", "claude-code")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	var r struct {
		Agents []struct {
			ID    string `json:"id"`
			Items []struct {
				Path  string `json:"path"`
				State string `json:"state"`
			} `json:"items"`
		} `json:"agents"`
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if len(r.Agents) != 1 || r.Agents[0].ID != "claude-code" || len(r.Agents[0].Items) == 0 {
		t.Errorf("json = %+v", r)
	}
}

func TestAgentNames(t *testing.T) {
	env := fakeHome(t)
	// A prefix matches both Claude layouts; named agents show even when
	// not installed
	_, out, _ := runWith(t, env, "status", "claude")
	if !strings.Contains(out, "Claude Code —") || !strings.Contains(out, "Claude desktop app — not installed") {
		t.Errorf("prefix claude:\n%s", out)
	}
	code, _, errOut := runWith(t, env, "status", "claud3")
	if code != 2 || !strings.Contains(errOut, `no agent "claud3"; known: claude-code, claude-desktop`) {
		t.Errorf("unknown agent: exit %d, %q", code, errOut)
	}
}

func TestList(t *testing.T) {
	code, out, _ := runWith(t, fakeHome(t), "list")
	if code != 0 || !strings.Contains(out, "claude-code") || !strings.Contains(out, "installed") || !strings.Contains(out, "not installed") {
		t.Errorf("list: exit %d\n%s", code, out)
	}
}

func TestUsage(t *testing.T) {
	if code, _, errOut := runWith(t, nil); code != 2 || !strings.Contains(errOut, "usage") {
		t.Errorf("no arguments: exit %d, %q", code, errOut)
	}
}
