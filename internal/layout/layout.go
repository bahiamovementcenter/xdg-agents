// Package layout loads agent layouts (agents/*.ncl) through go-nickel, which
// also runs their contract (agents/schema.ncl), into Go values.
package layout

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nickel-lang/go-nickel"

	"github.com/bahiamovementcenter/xdg-agents/internal/xdg"
)

// Layout mirrors agents/schema.ncl.
type Layout struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	XDGName   string          `json:"xdg_name"`
	Sources   []string        `json:"sources"`
	Roots     map[string]Root `json:"roots"`
	Paths     []Entry         `json:"paths"`
	Untracked []struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
	} `json:"untracked"`
	Settings []Setting `json:"settings"`
}

type Root struct {
	Default      string       `json:"default"`
	DefaultMacOS string       `json:"default_macos"`
	Env          string       `json:"env"`
	Category     xdg.Category `json:"category"`
}

type Entry struct {
	Root        string       `json:"root"`
	Path        string       `json:"path"`
	DefaultPath string       `json:"default_path"`
	Kind        string       `json:"kind"`
	Category    xdg.Category `json:"category"`
	Move        string       `json:"move"` // with_root, env, symlink, fixed
	Env         string       `json:"env"`
	Cleanup     string       `json:"cleanup"`
	Sensitive   bool         `json:"sensitive"`
	Legacy      bool         `json:"legacy"`
	Documented  bool         `json:"documented"`
	Holds       string       `json:"holds"`
	Note        string       `json:"note"`
}

type Setting struct {
	Name   string `json:"name"`
	Moves  string `json:"moves"`
	ForXDG string `json:"for_xdg"` // yes, with_caveat, no
	Note   string `json:"note"`
}

// schemaFile is the contract every layout imports; it isn't a layout itself.
const schemaFile = "schema.ncl"

// LoadDir evaluates every layout in dir, in name order. A layout that fails
// its contract is an error naming the file, with Nickel's message.
func LoadDir(dir string) ([]Layout, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.ncl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var layouts []Layout
	for _, f := range files {
		if filepath.Base(f) == schemaFile {
			continue
		}
		l, err := Load(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		layouts = append(layouts, l)
	}
	if len(layouts) == 0 {
		return nil, fmt.Errorf("no layouts in %s", dir)
	}
	return layouts, nil
}

// Load evaluates one layout.
func Load(path string) (Layout, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return Layout{}, err
	}
	// go-nickel evaluates source text, and Nickel looks for imports (here
	// schema.ncl) in the current directory, so evaluate from the layout's
	// own folder
	wd, err := os.Getwd()
	if err != nil {
		return Layout{}, err
	}
	if err := os.Chdir(filepath.Dir(path)); err != nil {
		return Layout{}, err
	}
	defer func() { _ = os.Chdir(wd) }()

	expr, err := nickel.NewContext().EvalDeep(string(src))
	if err != nil {
		return Layout{}, err
	}
	var l Layout
	if err := expr.ConvertTo(&l); err != nil {
		return Layout{}, err
	}
	return l, nil
}
