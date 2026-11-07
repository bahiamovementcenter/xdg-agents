package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// A page shaped like the real one: a section to skip, a reference table with
// a Scope column, an application data table, bullets, and enough filler rows
// to pass the minimum.
func directoryPage(extraRows ...string) string {
	var b strings.Builder
	b.WriteString("# Explore the .claude directory\n\n## What's not shown\n\n")
	b.WriteString("| File | Location |\n| - | - |\n| `elsewhere.json` | System |\n\n")
	b.WriteString("## File reference\n\n| File | Scope | What it does |\n| - | - | - |\n")
	b.WriteString("| [`CLAUDE.md`](#ce) | Project and global | Instructions |\n")
	b.WriteString("| [`settings.local.json`](#ce) | Project only | Overrides |\n")
	b.WriteString("| [`~/.claude.json`](#ce) | Global only | App state |\n\n")
	b.WriteString("## Application data\n\n| Path under `~/.claude/` | Contents |\n| - | - |\n")
	b.WriteString("| `projects/<project>/<session>.jsonl` | Transcripts |\n")
	b.WriteString("| `todos/`, `statsig/` | Legacy |\n")
	for i := 0; i < minDocPaths; i++ {
		fmt.Fprintf(&b, "| `filler-%02d/` | Filler |\n", i)
	}
	for _, r := range extraRows {
		b.WriteString(r + "\n")
	}
	b.WriteString("\n* **`sessions/`**: one file per running session\n")
	b.WriteString("* `jobs/` and `daemon/`: background state, see `not-this/`\n")
	b.WriteString("* **macOS**: `/private/tmp/claude-<uid>/`\n")
	b.WriteString("* The system prompt\n\n")
	b.WriteString("## Related resources\n\n| `after.md` | Not a path list |\n")
	return b.String()
}

func TestExtractDocPaths(t *testing.T) {
	got, err := ExtractDocPaths(directoryPage())
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, p := range got {
		set[p] = true
	}
	for _, want := range []string{
		"CLAUDE.md", ".claude.json", "projects/<project>/<session>.jsonl",
		"todos", "statsig", "sessions", "jobs", "daemon", "filler-00",
	} {
		if !set[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, unwanted := range []string{
		"settings.local.json", // project only
		"elsewhere.json",      // section that isn't about ~/.claude
		"after.md",            // section after the lists
		"not-this",            // after the bullet's colon
		"~/.claude",           // the table header
	} {
		if set[unwanted] {
			t.Errorf("unexpected %q in %v", unwanted, got)
		}
	}
}

func TestExtractDocPathsFailsOnRestructuredPage(t *testing.T) {
	_, err := ExtractDocPaths("# A redesigned page\n\nNo tables any more.\n")
	if err == nil || !strings.Contains(err.Error(), "format has probably changed") {
		t.Fatalf("want a format error, got %v", err)
	}
}

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"~/.claude/backups/": "backups",
		"~/.claude.json":     ".claude.json",
		"plans/":             "plans",
		"rules/*.md":         "rules/*.md",
	}
	for in, want := range cases {
		if got, ok := normalize(in); !ok || got != want {
			t.Errorf("normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"~/.claude", "~/.claude/", "/etc/claude-code", "~/.config/x", "claude purge"} {
		if got, ok := normalize(in); ok {
			t.Errorf("normalize(%q) = %q; want rejected", in, got)
		}
	}
}

func envPage(names ...string) string {
	var b strings.Builder
	b.WriteString("| Variable | Purpose |\n| - | - |\n")
	for i := 0; i < minEnvVars; i++ {
		fmt.Fprintf(&b, "| `FILLER_%02d` | Not a path |\n", i)
	}
	for _, n := range names {
		fmt.Fprintf(&b, "| `%s` | Something |\n", n)
	}
	return b.String()
}

func TestExtractDocEnv(t *testing.T) {
	got, err := ExtractDocEnv(envPage("CLAUDE_CONFIG_DIR", "CLAUDE_CODE_TMPDIR", "SOME_FILE", "ANTHROPIC_PROFILE"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"CLAUDE_CODE_TMPDIR", "CLAUDE_CONFIG_DIR", "SOME_FILE"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, err := ExtractDocEnv("| `ONLY_ONE` | x |\n"); err == nil {
		t.Error("want a format error for a page with too few variables")
	}
}

func layout(paths ...string) Layout {
	l := Layout{Roots: map[string]Root{"home": {Env: "CLAUDE_CONFIG_DIR"}}}
	for _, p := range paths {
		l.Paths = append(l.Paths, Entry{Root: "home", Path: p, Documented: true})
	}
	return l
}

func TestCompareNoDrift(t *testing.T) {
	l := layout("projects", "plugins", ".claude.json")
	docs := []string{"projects/<project>/memory", "plugins/.trash", ".claude.json"}
	if r := Compare(l, docs, []string{"CLAUDE_CONFIG_DIR"}); !r.Empty() {
		t.Errorf("want no drift, got %+v", r)
	}
}

func TestCompareFindsChanges(t *testing.T) {
	l := layout("projects", "removed")
	l.Untracked = []Untracked{{Path: "skipped"}}
	l.UntrackedEnv = []UntrackedEnv{{Name: "OLD_FILE"}}
	docs := []string{"projects/x", "projects-old", "brand-new", "skipped/inner"}
	r := Compare(l, docs, []string{"CLAUDE_CONFIG_DIR", "NEW_DIR"})

	// "projects" must not cover "projects-old": components, not prefixes
	if want := []string{"brand-new", "projects-old"}; !reflect.DeepEqual(r.NewPaths, want) {
		t.Errorf("NewPaths = %v, want %v", r.NewPaths, want)
	}
	if want := []string{"removed"}; !reflect.DeepEqual(r.GonePaths, want) {
		t.Errorf("GonePaths = %v, want %v", r.GonePaths, want)
	}
	if want := []string{"NEW_DIR"}; !reflect.DeepEqual(r.NewEnv, want) {
		t.Errorf("NewEnv = %v, want %v", r.NewEnv, want)
	}
	if want := []string{"OLD_FILE"}; !reflect.DeepEqual(r.GoneEnv, want) {
		t.Errorf("GoneEnv = %v, want %v", r.GoneEnv, want)
	}
}

func TestCompareIgnoresUndocumentedAndOtherRoots(t *testing.T) {
	l := layout("projects")
	l.Paths = append(l.Paths,
		Entry{Root: "home", Path: "hooks", Documented: false},
		Entry{Root: "managed", Path: "managed-settings.json", Documented: true},
	)
	if r := Compare(l, []string{"projects"}, nil); len(r.GonePaths) != 0 {
		t.Errorf("GonePaths = %v, want none", r.GonePaths)
	}
}

func settingsSchema(names ...string) []byte {
	props := map[string]any{}
	for i := 0; i < minSettings; i++ {
		props[fmt.Sprintf("filler%02d", i)] = map[string]any{"type": "boolean"}
	}
	for _, n := range names {
		props[n] = map[string]any{"type": "string"}
	}
	b, _ := json.Marshal(map[string]any{"properties": props})
	return b
}

func TestExtractSchemaSettings(t *testing.T) {
	got, err := ExtractSchemaSettings(settingsSchema("autoMemoryDirectory", "plansDirectory", "model", "cleanupPeriodDays"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"autoMemoryDirectory", "plansDirectory"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, err := ExtractSchemaSettings([]byte(`{"properties": {"x": {}}}`)); err == nil {
		t.Error("want a format error for a schema with too few settings")
	}
	if _, err := ExtractSchemaSettings([]byte(`not json`)); err == nil {
		t.Error("want an error for input that isn't JSON")
	}
}

func TestCompareSettings(t *testing.T) {
	l := Layout{
		Settings:          []Setting{{Name: "autoMemoryDirectory"}, {Name: "removedDirectory"}},
		UntrackedSettings: []UntrackedEnv{{Name: "ignoredPath"}},
	}
	r := CompareSettings(Report{}, l, []string{"autoMemoryDirectory", "ignoredPath", "newCacheDir"})
	if want := []string{"newCacheDir"}; !reflect.DeepEqual(r.NewSettings, want) {
		t.Errorf("NewSettings = %v, want %v", r.NewSettings, want)
	}
	if want := []string{"removedDirectory"}; !reflect.DeepEqual(r.GoneSettings, want) {
		t.Errorf("GoneSettings = %v, want %v", r.GoneSettings, want)
	}
	if r.Empty() {
		t.Error("a report with settings drift must not be empty")
	}
}

// Cases a line-by-line parser got wrong or only got right by accident.
func TestExtractDocPathsMarkdownEdgeCases(t *testing.T) {
	page := directoryPage(
		"| `pipe\\|dir/` | A pipe inside a code span doesn't split the cell |",
		"| [`linked/`](#x), `second/` | Code in a link, then more code |",
	) + "\n## Application data, continued\n\n" +
		"* [`enableArtifact`](#x) isn't set to `false`\n" +
		"* `no-colon/` is mentioned without a definition\n" +
		"* `defined/`: has a colon\n"
	got, err := ExtractDocPaths(page)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, p := range got {
		set[p] = true
	}
	for _, want := range []string{"pipe|dir", "linked", "second", "defined"} {
		if !set[want] {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	for _, unwanted := range []string{"pipe", "enableArtifact", "false", "no-colon"} {
		if set[unwanted] {
			t.Errorf("unexpected %q in %v", unwanted, got)
		}
	}
}
