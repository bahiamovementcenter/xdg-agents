// xdg-agents makes AI coding agents keep their files where the XDG Base
// Directory Specification says they belong (see AGENTS.md).
//
//	xdg-agents list
//	xdg-agents status [--check] [--json] [--all] [AGENT...]
//	xdg-agents plan [--json] [AGENT...]
//	xdg-agents apply [--yes] [AGENT...]
//	xdg-agents undo [--journal FILE]
//
// AGENT is a layout's id or a prefix of one ("claude" matches claude-code
// and claude-desktop); with none, every installed agent is shown. status and
// plan are read-only: they look at names, types and symlink targets, never at
// what a file holds, and change nothing. apply carries out the plan after
// checking it and asking first, recording each change so undo can reverse
// it. docs/status.md describes the report, docs/plan.md the plan and
// docs/apply.md applying it.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/bahiamovementcenter/xdg-agents/internal/apply"
	"github.com/bahiamovementcenter/xdg-agents/internal/layout"
	"github.com/bahiamovementcenter/xdg-agents/internal/plan"
	"github.com/bahiamovementcenter/xdg-agents/internal/status"
	"github.com/bahiamovementcenter/xdg-agents/internal/xdg"
)

const usage = `usage:
  xdg-agents list
  xdg-agents status [--check] [--json] [--all] [AGENT...]
  xdg-agents plan [--json] [AGENT...]
  xdg-agents apply [--yes] [AGENT...]
  xdg-agents undo [--journal FILE]

Options for all: --layouts DIR (default: agents)
`

// running finds which agent programs are running; tests replace it.
var running = apply.Running

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, os.Getenv))
}

// run is main without the process: it returns the exit status, so tests can
// drive it with any arguments, input and environment.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
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
	yes := fs.Bool("yes", false, "apply without asking")
	journal := fs.String("journal", "", "the journal to undo (default: the latest)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if cmd == "undo" {
		return undo(*journal, stdout, stderr, getenv)
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

	case "apply":
		chosen, err := choose(layouts, fs.Args())
		if err != nil {
			_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
			return 2
		}
		r := status.Compute(chosen, getenv, status.Options{GOOS: runtime.GOOS, UID: os.Getuid()})
		p := plan.Build(r, runtime.GOOS)
		opt := apply.Options{Getenv: getenv, Running: running, StateDir: stateDir(r), Now: time.Now}
		names := programNames(chosen)

		if problems := apply.Check(p, names, opt); len(problems) > 0 {
			_, _ = fmt.Fprintf(stderr, "xdg-agents: can't apply yet, so nothing was changed:\n  - %s\n", strings.Join(problems, "\n  - "))
			return 1
		}
		if err := plan.Text(stdout, p, getenv("HOME")); err != nil {
			return 2
		}
		if !*yes {
			_, _ = fmt.Fprint(stdout, "\nType yes to make these changes: ")
			answer, _ := bufio.NewReader(stdin).ReadString('\n')
			if strings.TrimSpace(answer) != "yes" {
				_, _ = fmt.Fprintln(stdout, "Nothing was changed.")
				return 1
			}
		}
		path, err := apply.Apply(p, names, opt)
		switch {
		case errors.Is(err, apply.ErrNothingToDo):
			_, _ = fmt.Fprintln(stdout, "Nothing to do.")
			return 0
		case err != nil:
			_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "Done. Every change is recorded in %s; `xdg-agents undo` reverses them.\n", path)
		return 0
	}
	_, _ = fmt.Fprint(stderr, usage)
	return 2
}

func undo(journal string, stdout, stderr io.Writer, getenv func(string) string) int {
	dir := filepath.Join(xdg.Resolve(getenv)[xdg.State].Path, "xdg-agents")
	if journal == "" {
		var err error
		if journal, err = apply.Latest(dir); err != nil {
			_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
			return 1
		}
	}
	if err := apply.Undo(journal); err != nil {
		_, _ = fmt.Fprintln(stderr, "xdg-agents:", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "Undone: %s\n", journal)
	return 0
}

// stateDir is where apply keeps its journals: $XDG_STATE_HOME/xdg-agents.
func stateDir(r status.Report) string {
	return filepath.Join(r.XDG[xdg.State].Path, "xdg-agents")
}

// programNames are the names each agent's process may run under: its
// layout id and its folder name (claude-code, claude).
func programNames(layouts []layout.Layout) map[string][]string {
	names := map[string][]string{}
	for _, l := range layouts {
		names[l.ID] = []string{l.ID, l.XDGName}
	}
	return names
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
