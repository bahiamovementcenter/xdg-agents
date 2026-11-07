package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const agentsDir = "../../agents"

func TestLoadRepositoryLayouts(t *testing.T) {
	layouts, err := LoadDir(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Layout{}
	for _, l := range layouts {
		byID[l.ID] = l
	}
	cc, ok := byID["claude-code"]
	if !ok {
		t.Fatalf("claude-code not loaded: %v", layouts)
	}
	if _, ok := byID["claude-desktop"]; !ok {
		t.Error("claude-desktop not loaded")
	}
	if cc.XDGName != "claude" || cc.Roots["home"].Env != "CLAUDE_CONFIG_DIR" || cc.Roots["home"].Category != "config" {
		t.Errorf("claude-code roots = %+v", cc.Roots)
	}
	// Enums arrive as plain strings, and defaults are filled in
	var settingsJSON, claudeJSON *Entry
	for i := range cc.Paths {
		switch cc.Paths[i].Path {
		case "settings.json":
			settingsJSON = &cc.Paths[i]
		case ".claude.json":
			claudeJSON = &cc.Paths[i]
		}
	}
	if settingsJSON == nil || settingsJSON.Category != "config" || settingsJSON.Move != "with_root" || !settingsJSON.Documented {
		t.Errorf("settings.json = %+v", settingsJSON)
	}
	if claudeJSON == nil || claudeJSON.DefaultPath != "~/.claude.json" || !claudeJSON.Sensitive {
		t.Errorf(".claude.json = %+v", claudeJSON)
	}
}

func TestLoadRejectsContractViolation(t *testing.T) {
	dir := t.TempDir()
	schema, err := os.ReadFile(filepath.Join(agentsDir, "schema.ncl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.ncl"), schema, 0o644); err != nil {
		t.Fatal(err)
	}
	// A file marked for symlinking: the contract forbids it
	bad := `let { Layout, .. } = import "schema.ncl" in
{
  id = "bad", name = "Bad", xdg_name = "bad", sources = [],
  roots = { home = { default = "~/.bad", category = 'config } },
  paths = [ { root = "home", path = "x", kind = 'file, category = 'state, move = 'symlink, holds = "h" } ],
} | Layout
`
	if err := os.WriteFile(filepath.Join(dir, "bad.ncl"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "bad.ncl") || !strings.Contains(err.Error(), "contract broken") {
		t.Fatalf("want a contract error naming bad.ncl, got %v", err)
	}
}

func TestLoadDirEmpty(t *testing.T) {
	if _, err := LoadDir(t.TempDir()); err == nil {
		t.Error("want an error for a folder with no layouts")
	}
}
