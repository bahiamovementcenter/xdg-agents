package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bahiamovementcenter/xdg-agents/internal/apply"
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
	code := run(args, strings.NewReader(""), &out, &errOut, func(k string) string { return env[k] })
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

func TestPlanCommand(t *testing.T) {
	env := fakeHome(t)
	code, out, errOut := runWith(t, env, "plan")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"Plan for Claude Code (nothing has been changed)",
		"1. move  ~/.claude → ~/.config/claude",
		"move  ~/.claude.json → ~/.config/claude/.claude.json",
		"link  ~/.config/claude/projects → ~/.local/state/claude/projects",
		`export CLAUDE_CONFIG_DIR="$HOME/.config/claude"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("plan lacks %q:\n%s", want, out)
		}
	}
	// It changes nothing
	if _, err := os.Stat(filepath.Join(env["HOME"], ".claude")); err != nil {
		t.Errorf("~/.claude is gone after plan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(env["HOME"], ".config")); err == nil {
		t.Error("plan created ~/.config")
	}
}

// apply on a fake home: refuses until the variables are set, then moves
// after confirmation, and undo puts everything back
func TestApplyAndUndoCommands(t *testing.T) {
	running = func([]string) []string { return nil }
	defer func() { running = apply.Running }()
	env := fakeHome(t)
	home := env["HOME"]

	code, _, errOut := runWith(t, env, "apply", "--yes")
	if code != 1 || !strings.Contains(errOut, "CLAUDE_CONFIG_DIR must already be set") {
		t.Fatalf("apply without the variables: exit %d, %s", code, errOut)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); err != nil {
		t.Fatal("apply changed files although it refused")
	}

	env["CLAUDE_CONFIG_DIR"] = filepath.Join(home, ".config/claude")
	// Without --yes, anything but "yes" changes nothing
	var out, errBuf bytes.Buffer
	code = run([]string{"apply"}, strings.NewReader("no\n"), &out, &errBuf, func(k string) string { return env[k] })
	if code != 1 || !strings.Contains(out.String(), "Nothing was changed.") {
		t.Fatalf("apply answered no: exit %d\n%s%s", code, out.String(), errBuf.String())
	}
	out.Reset()
	code = run([]string{"apply"}, strings.NewReader("yes\n"), &out, &errBuf, func(k string) string { return env[k] })
	if code != 0 || !strings.Contains(out.String(), "Done.") {
		t.Fatalf("apply answered yes: exit %d\n%s%s", code, out.String(), errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config/claude/settings.json")); err != nil {
		t.Errorf("settings.json didn't move: %v", err)
	}

	code, outU, errU := runWith(t, env, "undo")
	if code != 0 || !strings.Contains(outU, "Undone:") {
		t.Fatalf("undo: exit %d %s %s", code, outU, errU)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude/settings.json")); err != nil {
		t.Errorf("undo didn't put settings.json back: %v", err)
	}
}

func TestApplyRefusesWhileAgentRuns(t *testing.T) {
	running = func([]string) []string { return []string{"claude"} }
	defer func() { running = apply.Running }()
	env := fakeHome(t)
	env["CLAUDE_CONFIG_DIR"] = filepath.Join(env["HOME"], ".config/claude")
	code, _, errOut := runWith(t, env, "apply", "--yes")
	if code != 1 || !strings.Contains(errOut, "Claude Code is running") {
		t.Errorf("exit %d, %s", code, errOut)
	}
}

// status ends by naming the next command: plan while files are to move,
// undo once an apply has been made
func TestStatusNextSteps(t *testing.T) {
	running = func([]string) []string { return nil }
	defer func() { running = apply.Running }()
	env := fakeHome(t)

	_, out, _ := runWith(t, env, "status", "claude-code")
	if !strings.Contains(out, "`xdg-agents plan claude-code` shows the") {
		t.Errorf("status with files to move doesn't suggest plan:\n%s", out)
	}
	if strings.Contains(out, "xdg-agents undo") {
		t.Errorf("status suggests undo before any apply:\n%s", out)
	}
	_, out, _ = runWith(t, env, "status", "--json")
	if strings.Contains(out, "Next:") {
		t.Error("--json output has the Next lines")
	}

	env["CLAUDE_CONFIG_DIR"] = filepath.Join(env["HOME"], ".config/claude")
	if code, _, errOut := runWith(t, env, "apply", "--yes"); code != 0 {
		t.Fatalf("apply: exit %d, %s", code, errOut)
	}
	_, out, _ = runWith(t, env, "status")
	if strings.Contains(out, "xdg-agents plan") {
		t.Errorf("status after apply still suggests plan:\n%s", out)
	}
	if !strings.Contains(out, "The last apply is recorded in ~/.local/state/xdg-agents/journal-") || !strings.Contains(out, "`xdg-agents undo` reverses it") {
		t.Errorf("status after apply doesn't point at undo:\n%s", out)
	}

	if code, _, errOut := runWith(t, env, "undo"); code != 0 {
		t.Fatalf("undo: exit %d, %s", code, errOut)
	}
	_, out, _ = runWith(t, env, "status")
	if strings.Contains(out, "xdg-agents undo") || !strings.Contains(out, "xdg-agents plan") {
		t.Errorf("status after undo:\n%s", out)
	}
}
