package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadLayoutsFromRepo(t *testing.T) {
	layouts, err := loadLayouts("agents")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]Layout{}
	for _, l := range layouts {
		ids[l.ID] = l
	}
	for _, want := range []string{"claude-code", "claude-desktop"} {
		l, ok := ids[want]
		if !ok {
			t.Fatalf("layout %q not loaded (got %v)", want, layouts)
		}
		if len(l.Paths) == 0 || len(l.Roots) == 0 {
			t.Errorf("layout %q has %d paths and %d roots", want, len(l.Paths), len(l.Roots))
		}
	}
	if _, ok := ids[""]; ok {
		t.Error("schema.ncl was loaded as a layout")
	}
}

// A layout that breaks its contract must fail to load, naming the file
func TestLoadLayoutsRejectsContractViolation(t *testing.T) {
	dir := t.TempDir()
	schema, err := os.ReadFile(filepath.Join("agents", "schema.ncl"))
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
	_, err = loadLayouts(dir)
	if err == nil || !strings.Contains(err.Error(), "bad.ncl") || !strings.Contains(err.Error(), "contract broken") {
		t.Fatalf("want a contract error naming bad.ncl, got %v", err)
	}
}

func TestLoadLayoutsEmptyFolder(t *testing.T) {
	if _, err := loadLayouts(t.TempDir()); err == nil {
		t.Error("want an error for a folder with no layouts")
	}
}
