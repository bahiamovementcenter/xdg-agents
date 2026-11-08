package apply

import (
	"strings"
	"testing"
)

func TestMatchProcess(t *testing.T) {
	want := map[string]bool{"openclaw": true, "claude-code": true, "claude": true}
	for _, c := range []struct {
		comm, cmdline, found string
	}{
		// OpenClaw's gateway from Nix, as seen on a real machine
		{"node", "/nix/store/iz3f…-nodejs-22.23.1/bin/node /nix/store/4lzv…-openclaw-gateway-2026.7.1/lib/openclaw/dist/index.js gateway --port 3000", "openclaw"},
		{"claude", "claude", "claude"},
		{"node", "node /usr/lib/node_modules/@anthropic-ai/claude-code/cli.js", "claude-code"},
		{"node", "node /home/u/.local/bin/claude", "claude"},
		// A file named after an agent, given as an argument, isn't the agent
		{"vim", "vim /home/u/.config/claude/settings.json", ""},
		{"node", "node server.js --data /srv/openclaw", ""},
		// The program itself may sit in a folder named after the agent
		{"cli", "/opt/codex/bin/cli", ""},
		{"python3", "/usr/bin/python3 /home/u/tools/openclaw/run.py", "openclaw"},
	} {
		if got := matchProcess(c.comm, strings.Fields(c.cmdline), want); got != c.found {
			t.Errorf("%s %q: found %q, want %q", c.comm, c.cmdline, got, c.found)
		}
	}
}
