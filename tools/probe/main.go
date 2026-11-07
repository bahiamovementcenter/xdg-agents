// probe runs any agent through a scripted session against the fake model
// (internal/fakemodel) and records what it leaves on disk: names, types and
// symlink targets only, never contents (decisions 24 to 26 in
// docs/decisions.md).
//
// Nothing is written per agent. From the agent's own --help, the probe works
// out how to run it once non-interactively and how to let it use tools, and
// tries each way until one reaches the fake model. Every common model
// variable points at the fake model at once, which tells the protocol from
// the request path. Each candidate *_HOME / *_CONFIG_DIR variable (from the
// layout, the help, or the command's name) gets a run of its own, to see
// what it moves.
//
// Every run gets a throwaway root with its own HOME, XDG directories, TMPDIR
// and project folder, and a clean environment. An agent that takes a
// --settings file also gets a hook that records the transcript_path it
// passes, so the agent itself says where its transcript is.
//
//	go run ./tools/probe -agent claude -layout layout.json -out probe.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Report is the probe's output.
type Report struct {
	Agent   string `json:"agent"`
	Version string `json:"version"`
	// The invocation that reached the fake model, or none
	Invocation []string `json:"invocation"`
	// Every invocation tried, and how far it got
	Attempts []Attempt `json:"attempts"`
	// The default run, then one per config variable tried
	Runs      []Run      `json:"runs"`
	Protocols []string   `json:"protocols"`
	Moves     []Move     `json:"moves"`
	Checks    []Check    `json:"checks"`
	Unknown   []string   `json:"unknown"`
	XDG       []XDGUsage `json:"xdg"`
}

// Attempt is one way of running the agent that was tried.
type Attempt struct {
	Args     []string `json:"args"`
	Requests int      `json:"model_requests"`
	ExitCode int      `json:"exit_code"`
	TimedOut bool     `json:"timed_out,omitempty"`
}

// Move is what setting one variable did to what the default run created in
// the home folder.
type Move struct {
	Var    string   `json:"var"`
	Moved  []string `json:"moved"`  // gone from ~ and present under the variable's folder
	Stayed []string `json:"stayed"` // still created in ~
}

// XDGUsage is what the default run put in one XDG directory.
type XDGUsage struct {
	Dir     string   `json:"dir"`
	Entries []string `json:"entries"`
}

// Check is one claim, tested against the runs.
type Check struct {
	Claim string `json:"claim"`
	OK    bool   `json:"ok"`
	Note  string `json:"note,omitempty"`
}

func main() {
	agent := flag.String("agent", "", "the agent's command, such as claude or codex")
	layoutPath := flag.String("layout", "", "optional: the agent's layout, exported as JSON, to compare with")
	out := flag.String("out", "", "write the report as JSON here (default: stdout)")
	timeout := flag.Duration("timeout", 2*time.Minute, "how long one run may take")
	maxVars := flag.Int("max-vars", 4, "how many candidate config variables to try")
	keep := flag.Bool("keep", false, "keep the throwaway roots, and print where they are")
	recordHook := flag.String("record-hook", "", "internal: run as the agent's hook, appending to this file")
	flag.Parse()
	if *recordHook != "" {
		recordHookCall(*recordHook)
		return
	}
	if *agent == "" {
		fail(fmt.Errorf("-agent is required"))
	}
	bin, err := exec.LookPath(*agent)
	if err != nil {
		fail(err)
	}

	var lay *layout
	if *layoutPath != "" {
		if lay, err = loadLayout(*layoutPath); err != nil {
			fail(err)
		}
	}

	p := &prober{bin: bin, timeout: *timeout, keep: *keep}
	help := p.help()
	report := Report{Agent: *agent, Version: p.version()}

	// Find a way to run the agent once that reaches the fake model
	perm := permissionArgs(help)
	settings := supportsSettings(help)
	var def *Run
	for _, inv := range invocations(help) {
		run, err := p.run("defaults", inv, perm, settings, nil)
		if err != nil {
			fail(err)
		}
		report.Attempts = append(report.Attempts, Attempt{Args: inv, Requests: len(run.Model), ExitCode: run.ExitCode, TimedOut: run.TimedOut})
		if len(run.Model) > 0 {
			report.Invocation = inv
			def = &run
			break
		}
		if def == nil {
			def = &run // keep a listing even if no session started
		}
	}
	report.Runs = append(report.Runs, *def)
	report.Protocols = protocols(*def)

	// Try each config variable on its own run
	vars := configVars(help, filepath.Base(*agent), lay.envVars())
	if len(vars) > *maxVars {
		vars = vars[:*maxVars]
	}
	inv := report.Invocation
	if inv == nil {
		inv = []string{promptMark}
	}
	for _, v := range vars {
		run, err := p.run(v+" set", inv, perm, settings, &v)
		if err != nil {
			fail(err)
		}
		report.Runs = append(report.Runs, run)
		report.Moves = append(report.Moves, move(*def, run, v))
	}

	report.XDG = xdgUsage(*def)
	report.Checks = checks(report, lay)
	if lay != nil {
		report.Unknown = lay.unknown(*def)
	}

	b, _ := json.MarshalIndent(report, "", "  ")
	if *out == "" {
		fmt.Println(string(b))
	} else if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		fail(err)
	}
	printSummary(report)
}

func checks(r Report, lay *layout) []Check {
	var cs []Check
	add := func(claim string, ok bool, note string) { cs = append(cs, Check{claim, ok, note}) }

	add("a session reached the fake model", r.Invocation != nil, strings.Join(r.Invocation, " "))
	if len(r.Protocols) > 0 {
		add("the fake model answers the agent's protocol", answered(r.Protocols), strings.Join(r.Protocols, ", "))
	}
	for _, run := range r.Runs {
		add(fmt.Sprintf("%s: the agent finished", run.Name), run.ExitCode == 0 && !run.TimedOut,
			fmt.Sprintf("exit status %d", run.ExitCode))
		for _, t := range run.Transcripts {
			add(fmt.Sprintf("%s: the transcript the agent reported exists", run.Name), run.has(t), t)
		}
	}
	for _, m := range r.Moves {
		if len(m.Moved) > 0 {
			add(fmt.Sprintf("%s moves files out of ~", m.Var), true,
				fmt.Sprintf("moved %s; stayed %s", joinOr(m.Moved), joinOr(m.Stayed)))
		}
	}
	if lay == nil {
		return cs
	}
	// The layout's claims: each root with a variable moves its default
	// location, along with the entries whose own default location is
	// elsewhere in ~ (such as ~/.claude.json)
	for _, root := range lay.Roots {
		if root.Env == "" || !strings.HasPrefix(root.Default, "~/") {
			continue
		}
		var m *Move
		for i := range r.Moves {
			if r.Moves[i].Var == root.Env {
				m = &r.Moves[i]
			}
		}
		if m == nil {
			continue
		}
		top := topOf(root.Default)
		add(fmt.Sprintf("layout: %s moves %s", root.Env, root.Default), contains(m.Moved, top) && !contains(m.Stayed, top), "")
		for _, e := range lay.Paths {
			if e.DefaultPath != "" && strings.HasPrefix(e.DefaultPath, "~/") && contains(r.Runs[0].tops(), topOf(e.DefaultPath)) {
				add(fmt.Sprintf("layout: %s moves %s too", root.Env, e.DefaultPath), contains(m.Moved, topOf(e.DefaultPath)), "")
			}
		}
	}
	return cs
}

// answered reports whether every protocol the agent used is one the fake
// model answers.
func answered(ps []string) bool {
	for _, p := range ps {
		if p != "anthropic" {
			return false
		}
	}
	return true
}

func joinOr(xs []string) string {
	if len(xs) == 0 {
		return "nothing"
	}
	return strings.Join(xs, ", ")
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
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
	for _, x := range r.XDG {
		fmt.Fprintf(os.Stderr, "  in %s: %s\n", x.Dir, joinOr(x.Entries))
	}
	if len(r.Unknown) > 0 {
		fmt.Fprintf(os.Stderr, "  not in the layout: %s\n", strings.Join(r.Unknown, ", "))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "probe:", err)
	os.Exit(2)
}
