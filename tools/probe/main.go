// probe runs an agent through a scripted session against the fake model
// (internal/fakemodel) and records what it leaves on disk: names, types and
// symlink targets only, never contents (decision 24 in docs/decisions.md).
//
// Each run gets a throwaway root with its own HOME, XDG directories, TMPDIR
// and project folder, and a clean environment, so nothing from the real
// home or session leaks in. The agent runs twice: with its defaults, and
// with its config variable set, to check that the variable moves what the
// layout says it does.
//
//	go run ./tools/probe -layout layout.json -out probe.json
//
// Only Claude Code is supported so far.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bahiamovementcenter/xdg-agents/internal/fakemodel"
)

// The variables under a throwaway root, and the label each is reported as.
var places = []struct{ label, rel string }{
	{"$XDG_CONFIG_HOME", "xdg/config"},
	{"$XDG_DATA_HOME", "xdg/data"},
	{"$XDG_STATE_HOME", "xdg/state"},
	{"$XDG_CACHE_HOME", "xdg/cache"},
	{"$XDG_RUNTIME_DIR", "xdg/runtime"},
	{"$TMPDIR", "tmp"},
	{"$PROJECT", "project"},
	{"$CLAUDE_CONFIG_DIR", "relocated"},
	{"~", "home"}, // last: the others aren't inside it, but keep it the fallback
}

// Entry is one thing found on disk after a run.
type Entry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"` // dir, file, symlink
	Target string `json:"target,omitempty"`
}

// Run is the result of one run of the agent.
type Run struct {
	Name     string            `json:"name"`
	Env      map[string]string `json:"env"` // the variables that were set, as labels
	ExitCode int               `json:"exit_code"`
	TimedOut bool              `json:"timed_out,omitempty"`
	Model    []fakemodel.Event `json:"model_requests"`
	Entries  []Entry           `json:"entries"`
}

// Report is the probe's output.
type Report struct {
	Agent   string   `json:"agent"`
	Version string   `json:"version"`
	Runs    []Run    `json:"runs"`
	Checks  []Check  `json:"checks"`
	Unknown []string `json:"unknown"` // under the agent's root, matched by no layout entry
}

// Check is one claim from the layout, tested against the runs.
type Check struct {
	Claim string `json:"claim"`
	OK    bool   `json:"ok"`
	Note  string `json:"note,omitempty"`
}

func main() {
	agent := flag.String("agent", "claude", "the agent's command (only claude is supported)")
	layoutPath := flag.String("layout", "", "optional: the agent's layout, exported as JSON, to compare with")
	out := flag.String("out", "", "write the report as JSON here (default: stdout)")
	timeout := flag.Duration("timeout", 2*time.Minute, "how long one run may take")
	keep := flag.Bool("keep", false, "keep the throwaway roots, and print where they are")
	flag.Parse()
	if *agent != "claude" {
		fail(fmt.Errorf("only claude is supported so far, not %q", *agent))
	}

	bin, err := exec.LookPath(*agent)
	if err != nil {
		fail(err)
	}
	version, _ := exec.Command(bin, "--version").Output()

	report := Report{Agent: *agent, Version: strings.TrimSpace(string(version))}
	for _, variant := range []struct {
		name     string
		relocate bool
	}{{"defaults", false}, {"CLAUDE_CONFIG_DIR set", true}} {
		run, root, err := runOnce(bin, variant.name, variant.relocate, *timeout)
		if err != nil {
			fail(fmt.Errorf("%s: %w", variant.name, err))
		}
		report.Runs = append(report.Runs, run)
		if *keep {
			fmt.Fprintf(os.Stderr, "probe: %s kept in %s\n", variant.name, root)
		} else {
			_ = os.RemoveAll(root)
		}
	}

	report.Checks = checks(report.Runs)
	if *layoutPath != "" {
		paths, err := layoutPaths(*layoutPath)
		if err != nil {
			fail(err)
		}
		report.Unknown = unknown(report.Runs[0], paths)
	}

	b, _ := json.MarshalIndent(report, "", "  ")
	if *out == "" {
		fmt.Println(string(b))
	} else if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		fail(err)
	}
	printSummary(report)
}

func runOnce(bin, name string, relocate bool, timeout time.Duration) (Run, string, error) {
	root, err := os.MkdirTemp("", "xdg-agents-probe-")
	if err != nil {
		return Run{}, "", err
	}
	for _, p := range places {
		if p.label == "$CLAUDE_CONFIG_DIR" && !relocate {
			continue
		}
		if err := os.MkdirAll(filepath.Join(root, p.rel), 0o700); err != nil {
			return Run{}, root, err
		}
	}

	project := filepath.Join(root, "project")
	model := &fakemodel.Server{Workdir: project}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Run{}, root, err
	}
	srv := &http.Server{Handler: model, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	// A clean environment: only what the agent needs, so nothing from the
	// real home or session leaks in
	env := map[string]string{
		"HOME":                filepath.Join(root, "home"),
		"XDG_CONFIG_HOME":     filepath.Join(root, "xdg/config"),
		"XDG_DATA_HOME":       filepath.Join(root, "xdg/data"),
		"XDG_STATE_HOME":      filepath.Join(root, "xdg/state"),
		"XDG_CACHE_HOME":      filepath.Join(root, "xdg/cache"),
		"XDG_RUNTIME_DIR":     filepath.Join(root, "xdg/runtime"),
		"TMPDIR":              filepath.Join(root, "tmp"),
		"ANTHROPIC_BASE_URL":  "http://" + ln.Addr().String(),
		"ANTHROPIC_API_KEY":   "probe-not-a-real-key",
		"DISABLE_AUTOUPDATER": "1",
		"DISABLE_TELEMETRY":   "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
	}
	if relocate {
		env["CLAUDE_CONFIG_DIR"] = filepath.Join(root, "relocated")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-p", "Run the probe.",
		"--allowedTools", "Bash Write TodoWrite TaskCreate TaskUpdate")
	cmd.Dir = project
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	labels := map[string]string{}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
		labels[k] = label(root, v)
	}
	labels["ANTHROPIC_BASE_URL"] = "http://127.0.0.1:<port> (fake model)"
	// The agent's output stays out of the report: only the exit status counts
	cmd.Stdout, cmd.Stderr = nil, nil

	run := Run{Name: name, Env: labels}
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case ctx.Err() != nil:
		run.TimedOut, run.ExitCode = true, -1
	case errors.As(err, &exit):
		run.ExitCode = exit.ExitCode()
	case err != nil:
		return run, root, err
	}
	run.Model = model.Log()
	run.Entries, err = list(root)
	return run, root, err
}

// list walks root and reports every entry by label, without opening files.
func list(root string) ([]Entry, error) {
	var entries []Entry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}
		for _, p := range places {
			if rel == p.rel || rel == "xdg" { // the places themselves
				return nil
			}
		}
		e := Entry{Path: label(root, path), Kind: "file"}
		switch {
		case d.Type()&fs.ModeSymlink != 0:
			e.Kind = "symlink"
			if t, err := os.Readlink(path); err == nil {
				e.Target = label(root, t)
			}
		case d.IsDir():
			e.Kind = "dir"
		}
		entries = append(entries, e)
		return nil
	})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, err
}

// label turns a path under root into "~/.claude/x", "$XDG_STATE_HOME/x" and
// so on, so reports don't depend on where the root was.
func label(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	for _, p := range places {
		if rel == p.rel {
			return p.label
		}
		if strings.HasPrefix(rel, p.rel+"/") {
			return p.label + "/" + strings.TrimPrefix(rel, p.rel+"/")
		}
	}
	return rel
}

func checks(runs []Run) []Check {
	has := func(r Run, prefix string) bool {
		for _, e := range r.Entries {
			if e.Path == prefix || strings.HasPrefix(e.Path, prefix+"/") {
				return true
			}
		}
		return false
	}
	def, moved := runs[0], runs[1]
	var cs []Check
	add := func(claim string, ok bool, note string) { cs = append(cs, Check{claim, ok, note}) }
	for _, r := range runs {
		add(fmt.Sprintf("%s: the agent finished", r.Name), r.ExitCode == 0 && !r.TimedOut,
			fmt.Sprintf("exit status %d", r.ExitCode))
		called := 0
		for _, e := range r.Model {
			if e.Called != "" {
				called++
			}
		}
		add(fmt.Sprintf("%s: the agent used the fake model's tools", r.Name), called > 0,
			fmt.Sprintf("%d tool calls", called))
	}
	add("by default, files go to ~/.claude", has(def, "~/.claude"), "")
	add("by default, ~/.claude.json is written", has(def, "~/.claude.json"), "")
	add("with CLAUDE_CONFIG_DIR set, nothing goes to ~/.claude", !has(moved, "~/.claude"), "")
	add("with CLAUDE_CONFIG_DIR set, ~/.claude.json moves into it",
		!has(moved, "~/.claude.json") && has(moved, "$CLAUDE_CONFIG_DIR/.claude.json"), "")
	// What the agent puts in each XDG directory by itself. Not a pass or fail:
	// a layout records it, and anything new here is worth a look
	for _, x := range []string{"$XDG_CONFIG_HOME", "$XDG_DATA_HOME", "$XDG_STATE_HOME", "$XDG_CACHE_HOME", "$XDG_RUNTIME_DIR"} {
		var top []string
		for _, e := range def.Entries {
			if rest, ok := strings.CutPrefix(e.Path, x+"/"); ok && !strings.Contains(rest, "/") {
				top = append(top, rest)
			}
		}
		note := "nothing"
		if len(top) > 0 {
			note = strings.Join(top, ", ")
		}
		add(fmt.Sprintf("by default, what the agent puts in %s", x), true, note)
	}
	return cs
}

// layoutPaths returns the layout's paths under its home root.
func layoutPaths(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l struct {
		Paths []struct {
			Root string `json:"root"`
			Path string `json:"path"`
		} `json:"paths"`
		Untracked []struct {
			Path string `json:"path"`
		} `json:"untracked"`
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var paths []string
	for _, p := range l.Paths {
		if p.Root == "home" {
			paths = append(paths, p.Path)
		}
	}
	for _, u := range l.Untracked {
		paths = append(paths, u.Path)
	}
	return paths, nil
}

// unknown lists what the default run left under ~/.claude (and
// ~/.claude.json) that no layout path covers. A layout path covers itself
// and everything inside it.
func unknown(r Run, paths []string) []string {
	covered := func(rel string) bool {
		for _, p := range paths {
			if rel == p || strings.HasPrefix(rel, p+"/") {
				return true
			}
		}
		return false
	}
	var out []string
	for _, e := range r.Entries {
		var rel string
		switch {
		case e.Path == "~/.claude.json":
			rel = ".claude.json"
		case strings.HasPrefix(e.Path, "~/.claude/"):
			rel = strings.TrimPrefix(e.Path, "~/.claude/")
		default:
			continue
		}
		if !covered(rel) {
			// Report the outermost uncovered folder once, not its contents
			parent := false
			for _, o := range out {
				if strings.HasPrefix(rel, o+"/") {
					parent = true
				}
			}
			if !parent {
				out = append(out, rel)
			}
		}
	}
	return out
}

func printSummary(r Report) {
	fmt.Fprintf(os.Stderr, "\nprobe: %s %s\n", r.Agent, r.Version)
	for _, c := range r.Checks {
		mark := "ok  "
		if !c.OK {
			mark = "FAIL"
		}
		note := ""
		if c.Note != "" {
			note = " (" + c.Note + ")"
		}
		fmt.Fprintf(os.Stderr, "  %s %s%s\n", mark, c.Claim, note)
	}
	if len(r.Unknown) > 0 {
		fmt.Fprintf(os.Stderr, "  not in the layout: %s\n", strings.Join(r.Unknown, ", "))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "probe:", err)
	os.Exit(2)
}
