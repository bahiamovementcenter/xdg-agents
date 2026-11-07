// xdg-agents makes AI coding agents keep their files where the XDG Base
// Directory Specification says they belong (see AGENTS.md).
//
// For now it only loads the agent layouts: it evaluates every agents/*.ncl
// (or every .ncl file in the folder given as its first argument) through
// go-nickel, which also runs agents/schema.ncl's contracts, and prints a
// line per layout. `status`, `plan` and `apply` build on this.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nickel-lang/go-nickel"
)

// Layout is the part of a layout this program uses so far.
type Layout struct {
	ID    string            `json:"id"`
	Name  string            `json:"name"`
	Roots map[string]object `json:"roots"`
	Paths []object          `json:"paths"`
}

// The fields of roots and paths aren't read yet; decoding them as generic
// objects keeps the count right without fixing their Go shape early.
type object = map[string]any

// schemaFile is the contract every layout imports; it isn't a layout itself.
const schemaFile = "schema.ncl"

// loadLayouts evaluates every layout in dir, in name order. A layout that
// fails its contract is an error naming the file, with Nickel's message.
func loadLayouts(dir string) ([]Layout, error) {
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
		l, err := loadLayout(f)
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

func loadLayout(path string) (Layout, error) {
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

func main() {
	dir := "agents"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	layouts, err := loadLayouts(dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "xdg-agents:", err)
		os.Exit(1)
	}
	for _, l := range layouts {
		roots := make([]string, 0, len(l.Roots))
		for r := range l.Roots {
			roots = append(roots, r)
		}
		sort.Strings(roots)
		fmt.Printf("%s (%s): %d paths under %s\n", l.ID, l.Name, len(l.Paths), strings.Join(roots, ", "))
	}
}
