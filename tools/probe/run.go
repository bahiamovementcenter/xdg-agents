package main

import (
	"context"
	"errors"
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

// The places under a throwaway root, and the label each is reported as.
// relocated is where a config variable points, in runs that set one.
var places = []struct{ label, rel string }{
	{"$XDG_CONFIG_HOME", "xdg/config"},
	{"$XDG_DATA_HOME", "xdg/data"},
	{"$XDG_STATE_HOME", "xdg/state"},
	{"$XDG_CACHE_HOME", "xdg/cache"},
	{"$XDG_RUNTIME_DIR", "xdg/runtime"},
	{"$TMPDIR", "tmp"},
	{"$PROJECT", "project"},
	{"$VAR", "relocated"},
	{"~", "home"},
}

// The probe's prompt. The fake model ignores it; it's there for agents that
// need one.
const prompt = "Run the probe."

// Entry is one thing found on disk after a run.
type Entry struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"` // dir, file, symlink
	Target string `json:"target,omitempty"`
}

// Run is the result of one run of the agent.
type Run struct {
	Name     string            `json:"name"`
	Args     []string          `json:"args"`
	Env      map[string]string `json:"env"` // the variables set, as labels
	ExitCode int               `json:"exit_code"`
	TimedOut bool              `json:"timed_out,omitempty"`
	Model    []fakemodel.Event `json:"model_requests"`
	// The transcript paths the agent passed to its hooks, as labels
	Transcripts []string `json:"transcripts,omitempty"`
	Entries     []Entry  `json:"entries"`
}

func (r Run) has(prefix string) bool {
	for _, e := range r.Entries {
		if e.Path == prefix || strings.HasPrefix(e.Path, prefix+"/") {
			return true
		}
	}
	return false
}

// tops returns the names the run created directly in the home folder.
func (r Run) tops() []string {
	var out []string
	for _, e := range r.Entries {
		if rest, ok := strings.CutPrefix(e.Path, "~/"); ok && !strings.Contains(rest, "/") {
			out = append(out, "~/"+rest)
		}
	}
	return out
}

func topOf(path string) string {
	rest := strings.TrimPrefix(path, "~/")
	first, _, _ := strings.Cut(rest, "/")
	return "~/" + first
}

type prober struct {
	bin     string
	timeout time.Duration
	keep    bool
}

func (p *prober) version() string {
	out, _ := p.quick("--version")
	line, _, _ := strings.Cut(strings.TrimSpace(out), "\n")
	return line
}

// help returns the agent's --help text, run in a throwaway home.
func (p *prober) help() string {
	out, _ := p.quick("--help")
	return out
}

func (p *prober) quick(arg string) (string, error) {
	root, err := os.MkdirTemp("", "xdg-agents-probe-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(root) }()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, arg)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + root, "TMPDIR=" + root, "NO_COLOR=1"}
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// run runs the agent once in a fresh throwaway root. With configVar set, that
// variable points at root/relocated.
func (p *prober) run(name string, inv, perm []string, settings bool, configVar *string) (Run, error) {
	root, err := os.MkdirTemp("", "xdg-agents-probe-")
	if err != nil {
		return Run{}, err
	}
	if p.keep {
		defer func() { _, _ = os.Stderr.WriteString("probe: " + name + " kept in " + root + "\n") }()
	} else {
		defer func() { _ = os.RemoveAll(root) }()
	}
	for _, pl := range places {
		if pl.label == "$VAR" && configVar == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Join(root, pl.rel), 0o700); err != nil {
			return Run{}, err
		}
	}
	project := filepath.Join(root, "project")
	own := filepath.Join(root, "probe") // the probe's own files; list() skips them
	if err := os.MkdirAll(own, 0o700); err != nil {
		return Run{}, err
	}

	model := &fakemodel.Server{Workdir: project}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Run{}, err
	}
	srv := &http.Server{Handler: model, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	env := map[string]string{
		"HOME":            filepath.Join(root, "home"),
		"XDG_CONFIG_HOME": filepath.Join(root, "xdg/config"),
		"XDG_DATA_HOME":   filepath.Join(root, "xdg/data"),
		"XDG_STATE_HOME":  filepath.Join(root, "xdg/state"),
		"XDG_CACHE_HOME":  filepath.Join(root, "xdg/cache"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "xdg/runtime"),
		"TMPDIR":          filepath.Join(root, "tmp"),
		"NO_COLOR":        "1",
	}
	for k, v := range modelEnv("http://" + ln.Addr().String()) {
		env[k] = v
	}
	if configVar != nil {
		env[*configVar] = filepath.Join(root, "relocated")
	}

	args := make([]string, 0, len(inv)+len(perm)+2)
	for _, a := range inv {
		args = append(args, strings.ReplaceAll(a, promptMark, prompt))
	}
	args = append(args, perm...)
	hookLog := filepath.Join(own, "hooks.jsonl")
	if settings {
		b, err := hookSettings(hookLog)
		if err != nil {
			return Run{}, err
		}
		file := filepath.Join(own, "settings.json")
		if err := os.WriteFile(file, b, 0o600); err != nil {
			return Run{}, err
		}
		args = append(args, "--settings", file)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.bin, args...)
	cmd.Dir = project
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	labels := map[string]string{}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
		if strings.HasPrefix(v, "http://127.0.0.1:") {
			v = "(fake model)"
		}
		labels[k] = label(root, v)
	}
	// The agent's output stays out of the report: only the exit status counts
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil

	run := Run{Name: name, Args: inv, Env: labels}
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case ctx.Err() != nil:
		run.TimedOut, run.ExitCode = true, -1
	case errors.As(err, &exit):
		run.ExitCode = exit.ExitCode()
	case err != nil:
		return run, err
	}
	run.Model = model.Log()
	run.Transcripts = hookTranscripts(root, hookLog)
	run.Entries, err = list(root)
	return run, err
}

// list walks root and reports every entry by label, without opening files.
func list(root string) ([]Entry, error) {
	var entries []Entry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." || rel == "xdg" {
			return nil
		}
		if rel == "probe" {
			return fs.SkipDir
		}
		for _, p := range places {
			if rel == p.rel {
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

// protocols returns the model APIs the run used.
func protocols(r Run) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range r.Model {
		if e.Protocol != "" && !seen[e.Protocol] {
			seen[e.Protocol] = true
			out = append(out, e.Protocol)
		}
	}
	sort.Strings(out)
	return out
}

// move compares a run with a variable set against the default run: of what
// the default run created directly in ~, what's gone (and the variable's
// folder has something) and what's still created.
func move(def, withVar Run, v string) Move {
	m := Move{Var: v}
	relocated := withVar.has("$VAR")
	for _, t := range def.tops() {
		switch {
		case withVar.has(t):
			m.Stayed = append(m.Stayed, t)
		case relocated:
			m.Moved = append(m.Moved, t)
		}
	}
	return m
}

func xdgUsage(r Run) []XDGUsage {
	var out []XDGUsage
	for _, dir := range []string{"$XDG_CONFIG_HOME", "$XDG_DATA_HOME", "$XDG_STATE_HOME", "$XDG_CACHE_HOME", "$XDG_RUNTIME_DIR"} {
		u := XDGUsage{Dir: dir}
		for _, e := range r.Entries {
			if rest, ok := strings.CutPrefix(e.Path, dir+"/"); ok && !strings.Contains(rest, "/") {
				u.Entries = append(u.Entries, rest)
			}
		}
		out = append(out, u)
	}
	return out
}
