package main

import (
	"reflect"
	"testing"
)

// Shortened from real help texts: the forms each agent's --help uses.
const (
	claudeHelp = `Usage: claude [options] [command] [prompt]
Options:
  -p, --print                      Print response and exit (useful for pipes)
  --allowedTools, --allowed-tools <tools...>
  --dangerously-skip-permissions   Bypass all permission checks
  --settings <file-or-json>        Path to a settings JSON file
Environment: CLAUDE_CONFIG_DIR`
	codexHelp = `Usage: codex [OPTIONS] [PROMPT]
Commands:
  exec        Run Codex non-interactively
  login       Manage login
Options:
      --full-auto   Low-friction sandboxed automatic execution
      --dangerously-bypass-approvals-and-sandbox
CODEX_HOME sets the config folder`
	plainHelp = `usage: agent [prompt]`
)

func TestInvocations(t *testing.T) {
	for _, c := range []struct {
		help string
		want [][]string
	}{
		{claudeHelp, [][]string{{"-p", promptMark}, {promptMark}}},
		{codexHelp, [][]string{{"exec", promptMark}, {promptMark}}},
		{plainHelp, [][]string{{promptMark}}},
	} {
		if got := invocations(c.help); !reflect.DeepEqual(got, c.want) {
			t.Errorf("invocations = %v, want %v", got, c.want)
		}
	}
}

func TestPermissionArgs(t *testing.T) {
	if got := permissionArgs(claudeHelp); len(got) != 2 || got[0] != "--allowedTools" {
		t.Errorf("claude: %v, want an --allowedTools list (not the blanket flag, refused as root)", got)
	}
	if got := permissionArgs(codexHelp); !reflect.DeepEqual(got, []string{"--dangerously-bypass-approvals-and-sandbox"}) {
		t.Errorf("codex: %v", got)
	}
	if got := permissionArgs(plainHelp); got != nil {
		t.Errorf("plain: %v, want none", got)
	}
}

func TestConfigVars(t *testing.T) {
	got := configVars(claudeHelp, "claude", []string{"CLAUDE_CONFIG_DIR"})
	want := []string{"CLAUDE_CONFIG_DIR", "CLAUDE_HOME"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("claude: %v, want %v", got, want)
	}
	got = configVars(codexHelp+" XDG_CONFIG_HOME", "codex", nil)
	want = []string{"CODEX_HOME", "CODEX_CONFIG_DIR"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("codex: %v, want %v (XDG variables left out)", got, want)
	}
}

func TestSupportsSettings(t *testing.T) {
	if !supportsSettings(claudeHelp) || supportsSettings(codexHelp) {
		t.Error("supportsSettings wrong")
	}
}
