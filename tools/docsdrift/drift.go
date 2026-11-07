package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Layout is the part of an exported agent layout (agents/*.ncl) the check
// needs.
type Layout struct {
	Roots        map[string]Root `json:"roots"`
	Paths        []Entry         `json:"paths"`
	Untracked    []Untracked     `json:"untracked"`
	UntrackedEnv []UntrackedEnv  `json:"untracked_env"`
	Settings     []Setting       `json:"settings"`
	// Same shape as UntrackedEnv: a name and a reason
	UntrackedSettings []UntrackedEnv `json:"untracked_settings"`
}

type Setting struct {
	Name string `json:"name"`
}

type Root struct {
	Env string `json:"env"`
}

type Entry struct {
	Root       string `json:"root"`
	Path       string `json:"path"`
	Env        string `json:"env"`
	Documented bool   `json:"documented"`
}

type Untracked struct {
	Path string `json:"path"`
}

type UntrackedEnv struct {
	Name string `json:"name"`
}

// The docs check compares only the "home" root (CLAUDE_CONFIG_DIR): the
// directory page documents nothing else as paths under ~/.claude.
const homeRoot = "home"

// Below these counts the page has probably been restructured, and an empty
// diff would be a false all-clear.
const (
	minDocPaths = 20
	minEnvVars  = 50
	minSettings = 50
)

var (
	backtick  = regexp.MustCompile("`([^`]+)`")
	envRow    = regexp.MustCompile("^\\| `([A-Z][A-Z0-9_]*)` \\|")
	pathLike  = regexp.MustCompile(`(_DIR|_DIRS|_PATH|_FILE|TMPDIR|_HOME)$`)
	bulletRow = regexp.MustCompile(`^\s*[*-] `)
	// camelCase setting names that take a path, such as autoMemoryDirectory.
	// Matching descriptions instead picks up timeouts and toggles that merely
	// mention a directory.
	pathSetting = regexp.MustCompile(`(Dir|Directory|Directories|Path|Paths|File|Folder|Location|Home)$`)
)

// The sections of the directory page that list what is stored under
// ~/.claude. Everything else on the page (project files, files kept
// elsewhere, how to clear data) is ignored.
var directorySections = []string{"## File reference", "## Application data"}

// ExtractDocPaths returns the paths under ~/.claude that the directory page
// lists, normalized (see normalize), sorted and without duplicates. It reads
// the first cell of table rows, skipping rows whose Scope column says
// "Project only", and the backticked names that start a bullet point.
func ExtractDocPaths(md string) ([]string, error) {
	found := map[string]bool{}
	inSection := false
	scopeCol := -1
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "## ") {
			inSection = false
			for _, s := range directorySections {
				if strings.HasPrefix(line, s) {
					inSection = true
				}
			}
			continue
		}
		if !inSection {
			continue
		}
		switch {
		case strings.HasPrefix(line, "|"):
			cells := splitRow(line)
			if isSeparator(cells) {
				continue
			}
			if !strings.Contains(cells[0], "`") {
				// A header row: remember where the scope column is
				scopeCol = -1
				for i, c := range cells {
					if strings.EqualFold(strings.TrimSpace(c), "Scope") {
						scopeCol = i
					}
				}
				continue
			}
			if scopeCol >= 0 && scopeCol < len(cells) &&
				strings.Contains(strings.ToLower(cells[scopeCol]), "project only") {
				continue
			}
			for _, m := range backtick.FindAllStringSubmatch(cells[0], -1) {
				if p, ok := normalize(m[1]); ok {
					found[p] = true
				}
			}
		case bulletRow.MatchString(line):
			// Only names before the first colon: "`jobs/` and `daemon/`: ..."
			head, _, _ := strings.Cut(bulletRow.ReplaceAllString(line, ""), ":")
			head = strings.TrimPrefix(strings.TrimSpace(head), "**")
			if !strings.HasPrefix(head, "`") {
				continue
			}
			for _, m := range backtick.FindAllStringSubmatch(head, -1) {
				if p, ok := normalize(m[1]); ok {
					found[p] = true
				}
			}
		default:
			if strings.TrimSpace(line) == "" {
				scopeCol = -1
			}
		}
	}
	paths := sortedKeys(found)
	if len(paths) < minDocPaths {
		return paths, fmt.Errorf("found only %d paths on the directory page (expected at least %d): its format has probably changed, so update ExtractDocPaths", len(paths), minDocPaths)
	}
	return paths, nil
}

// ExtractDocEnv returns the path-like environment variables (names ending in
// _DIR, _PATH, _FILE and so on) that the env-vars page lists.
func ExtractDocEnv(md string) ([]string, error) {
	all := map[string]bool{}
	pathy := map[string]bool{}
	for _, line := range strings.Split(md, "\n") {
		m := envRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		all[m[1]] = true
		if pathLike.MatchString(m[1]) {
			pathy[m[1]] = true
		}
	}
	if len(all) < minEnvVars {
		return nil, fmt.Errorf("found only %d variables on the env-vars page (expected at least %d): its format has probably changed, so update ExtractDocEnv", len(all), minEnvVars)
	}
	return sortedKeys(pathy), nil
}

// ExtractSchemaSettings returns the top-level settings in a JSON Schema for
// the agent's settings file whose names say they take a path. Nested ones
// (marketplace sources, sandbox binaries) configure other things, not where
// the agent stores its own files.
func ExtractSchemaSettings(schema []byte) ([]string, error) {
	var s struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schema, &s); err != nil {
		return nil, fmt.Errorf("not a JSON Schema: %w", err)
	}
	if len(s.Properties) < minSettings {
		return nil, fmt.Errorf("found only %d settings in the schema (expected at least %d): its format has probably changed, so update ExtractSchemaSettings", len(s.Properties), minSettings)
	}
	found := map[string]bool{}
	for name := range s.Properties {
		if pathSetting.MatchString(name) {
			found[name] = true
		}
	}
	return sortedKeys(found), nil
}

// normalize turns a name from the docs into a path relative to ~/.claude:
// "~/.claude/backups/" becomes "backups", "~/.claude.json" becomes
// ".claude.json". It rejects things that aren't paths under ~/.claude.
func normalize(s string) (string, bool) {
	s = strings.TrimSpace(s)
	switch {
	case s == "~/.claude.json":
		return ".claude.json", true
	case s == "~/.claude" || s == "~/.claude/":
		return "", false
	case strings.HasPrefix(s, "~/.claude/"):
		s = strings.TrimPrefix(s, "~/.claude/")
	case strings.HasPrefix(s, "~") || strings.HasPrefix(s, "/") || strings.Contains(s, " "):
		return "", false
	}
	s = strings.TrimSuffix(s, "/")
	if s == "" {
		return "", false
	}
	return s, true
}

// Report is what changed between the docs and a layout.
type Report struct {
	// Listed in the docs but covered by no layout entry and not untracked
	NewPaths []string
	// Layout entries marked documented that the docs no longer list
	GonePaths []string
	// Path-like variables in the docs that the layout neither uses nor lists
	// as untracked
	NewEnv []string
	// Variables the layout uses or lists that the docs no longer list
	GoneEnv []string
	// Path-like settings in the schema that the layout neither uses nor
	// lists as untracked
	NewSettings []string
	// Settings the layout uses or lists that the schema no longer lists
	GoneSettings []string
}

func (r Report) Empty() bool {
	return len(r.NewPaths)+len(r.GonePaths)+len(r.NewEnv)+len(r.GoneEnv)+
		len(r.NewSettings)+len(r.GoneSettings) == 0
}

// Compare checks a layout against the paths and variables from the docs.
// A docs path is covered by a layout path equal to it or containing it
// ("projects" covers "projects/<project>/memory"); a layout path is still
// documented if some docs path is equal to it or inside it ("plugins" by
// "plugins/.trash").
func Compare(l Layout, docPaths, docEnv []string) Report {
	var r Report

	var layoutPaths []string
	for _, e := range l.Paths {
		if e.Root == homeRoot {
			layoutPaths = append(layoutPaths, e.Path)
		}
	}
	var covering []string
	covering = append(covering, layoutPaths...)
	for _, u := range l.Untracked {
		covering = append(covering, u.Path)
	}
	for _, d := range docPaths {
		if !anyContains(covering, d) {
			r.NewPaths = append(r.NewPaths, d)
		}
	}
	for _, e := range l.Paths {
		if e.Root != homeRoot || !e.Documented {
			continue
		}
		documented := false
		for _, d := range docPaths {
			if contains(e.Path, d) {
				documented = true
				break
			}
		}
		if !documented {
			r.GonePaths = append(r.GonePaths, e.Path)
		}
	}
	for _, u := range l.Untracked {
		if !anyContainedBy(docPaths, u.Path) {
			r.GonePaths = append(r.GonePaths, u.Path+" (untracked)")
		}
	}

	known := map[string]bool{}
	for _, root := range l.Roots {
		if root.Env != "" {
			known[root.Env] = true
		}
	}
	for _, e := range l.Paths {
		if e.Env != "" {
			known[e.Env] = true
		}
	}
	for _, u := range l.UntrackedEnv {
		known[u.Name] = true
	}
	inDocs := map[string]bool{}
	for _, v := range docEnv {
		inDocs[v] = true
		if !known[v] {
			r.NewEnv = append(r.NewEnv, v)
		}
	}
	for _, v := range sortedKeys(known) {
		if !inDocs[v] {
			r.GoneEnv = append(r.GoneEnv, v)
		}
	}

	sort.Strings(r.NewPaths)
	sort.Strings(r.GonePaths)
	return r
}

// CompareSettings checks the layout's settings against the path-like
// settings from the agent's settings schema, and adds the result to r.
func CompareSettings(r Report, l Layout, schemaSettings []string) Report {
	known := map[string]bool{}
	for _, s := range l.Settings {
		known[s.Name] = true
	}
	for _, u := range l.UntrackedSettings {
		known[u.Name] = true
	}
	inSchema := map[string]bool{}
	for _, s := range schemaSettings {
		inSchema[s] = true
		if !known[s] {
			r.NewSettings = append(r.NewSettings, s)
		}
	}
	for _, s := range sortedKeys(known) {
		if !inSchema[s] {
			r.GoneSettings = append(r.GoneSettings, s)
		}
	}
	return r
}

// contains reports whether path p is q or a folder that q is inside,
// comparing whole components ("plugins" contains "plugins/.trash" but not
// "plugins-old").
func contains(p, q string) bool {
	return p == q || strings.HasPrefix(q, p+"/")
}

func anyContains(ps []string, q string) bool {
	for _, p := range ps {
		if contains(p, q) {
			return true
		}
	}
	return false
}

func anyContainedBy(qs []string, p string) bool {
	for _, q := range qs {
		if contains(p, q) {
			return true
		}
	}
	return false
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	return strings.Split(line, "|")
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if strings.Trim(strings.TrimSpace(c), ":-") != "" {
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
