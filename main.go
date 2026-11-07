// xdg-agents makes AI coding agents keep their files where the XDG Base
// Directory Specification says they belong (see AGENTS.md).
//
//	xdg-agents list
//	xdg-agents status [--check] [--json] [--all] [AGENT...]
//	xdg-agents plan [--json] [AGENT...]
//
// AGENT is a layout's id or a prefix of one ("claude" matches claude-code
// and claude-desktop); with none, every installed agent is shown. status and
// plan are read-only: they look at names, types and symlink targets, never at
// what a file holds, and change nothing. docs/status.md describes the
// report, docs/plan.md the plan.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/bahiamovementcenter/xdg-agents/internal/layout"
	"github.com/bahiamovementcenter/xdg-agents/internal/plan"
	"github.com/bahiamovementcenter/xdg-agents/internal/status"
)

const usage = `usage:
  xdg-agents list
  xdg-agents status [--check] [--json] [--all] [AGENT...]
  xdg-agents plan [--json] [AGENT...]

Options for all: --layouts DIR (default: agents)
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))
}

// run is main without the process: it returns the exit status, so tests can
// drive it with any arguments and environment.
func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, args := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("layouts", "agents", "the folder of agent layouts")
	check := fs.Bool("check", false, "exit 1 if anything shown is out of place")
	asJSON := fs.Bool("json", false, "write the report as JSON")
	all := fs.Bool("all", false, "also list layout entries that aren't on disk")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	layouts, err := layout.LoadDir(*dir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
		return 2
	}

	switch cmd {
	case "list":
		r := status.Compute(layouts, getenv, status.Options{Named: true, GOOS: runtime.GOOS, UID: os.Getuid()})
		for _, a := range r.Agents {
			installed := "not installed"
			if a.Installed {
				installed = "installed"
			}
			_, _ = fmt.Fprintf(stdout, "%-16s %-24s %s\n", a.ID, a.Name, installed)
		}
		return 0

	case "status":
		chosen, err := choose(layouts, fs.Args())
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
			return 2
		}
		r := status.Compute(chosen, getenv, status.Options{
			All: *all, GOOS: runtime.GOOS, UID: os.Getuid(), Named: len(fs.Args()) > 0,
		})
		if *asJSON {
			b, _ := json.MarshalIndent(r, "", "  ")
			_, _ = fmt.Fprintln(stdout, string(b))
		} else if err := status.Text(stdout, r, getenv("HOME"), plain(stdout, getenv)); err != nil {
			_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
			return 2
		}
		if *check && r.NeedsAttention() {
			return 1
		}
		return 0

	case "plan":
		chosen, err := choose(layouts, fs.Args())
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
			return 2
		}
		r := status.Compute(chosen, getenv, status.Options{GOOS: runtime.GOOS, UID: os.Getuid()})
		p := plan.Build(r, runtime.GOOS)
		if *asJSON {
			b, _ := json.MarshalIndent(p, "", "  ")
			_, _ = fmt.Fprintln(stdout, string(b))
		} else if err := plan.Text(stdout, p, getenv("HOME")); err != nil {
			_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
			return 2
		}
		return 0
	}
	_, _ = fmt.Fprint(stderr, usage)
	return 2
}

// choose picks the layouts named by id or id prefix; none means all.
func choose(layouts []layout.Layout, names []string) ([]layout.Layout, error) {
	if len(names) == 0 {
		return layouts, nil
	}
	var chosen []layout.Layout
	seen := map[string]bool{}
	for _, n := range names {
		found := false
		for _, l := range layouts {
			if strings.HasPrefix(l.ID, n) {
				found = true
				if !seen[l.ID] {
					seen[l.ID] = true
					chosen = append(chosen, l)
				}
			}
		}
		if !found {
			ids := make([]string, 0, len(layouts))
			for _, l := range layouts {
				ids = append(ids, l.ID)
			}
			sort.Strings(ids)
			return nil, fmt.Errorf("no agent %q; known: %s", n, strings.Join(ids, ", "))
		}
	}
	return chosen, nil
}

// plain reports whether to use words instead of symbols: when the output
// isn't a terminal, or NO_COLOR is set.
func plain(w io.Writer, getenv func(string) string) bool {
	if getenv("NO_COLOR") != "" {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return true
	}
	fi, err := f.Stat()
	return err != nil || fi.Mode()&os.ModeCharDevice == 0
}
