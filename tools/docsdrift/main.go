// docsdrift compares an agent layout with the agent's documentation and
// reports what changed: paths or path-like environment variables the docs
// list that the layout doesn't cover, and ones the layout relies on that the
// docs no longer list. It only detects; deciding what a new path is stays a
// reviewed change to the layout.
//
//	nickel export agents/claude-code.ncl > layout.json
//	go run ./tools/docsdrift -layout layout.json \
//	  -directory-doc claude-directory.md -env-doc env-vars.md
//
// Exit status: 0 no drift, 1 drift found, 2 the check couldn't run (bad
// input, or a docs page whose format has changed).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	layoutPath := flag.String("layout", "", "the layout, exported as JSON (nickel export)")
	dirDoc := flag.String("directory-doc", "", "the agent's directory documentation page (Markdown)")
	envDoc := flag.String("env-doc", "", "the agent's environment variables page (Markdown)")
	flag.Parse()
	if *layoutPath == "" || *dirDoc == "" || *envDoc == "" {
		flag.Usage()
		os.Exit(2)
	}

	report, err := run(*layoutPath, *dirDoc, *envDoc)
	if err != nil {
		fmt.Fprintln(os.Stderr, "docsdrift:", err)
		os.Exit(2)
	}

	text := format(report)
	fmt.Print(text)
	// In GitHub Actions, also show the report on the run's summary page
	if summary := os.Getenv("GITHUB_STEP_SUMMARY"); summary != "" {
		if f, err := os.OpenFile(summary, os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			_, _ = io.WriteString(f, "## Docs drift\n\n"+text)
			_ = f.Close()
		}
	}
	if !report.Empty() {
		os.Exit(1)
	}
}

func run(layoutPath, dirDoc, envDoc string) (Report, error) {
	raw, err := os.ReadFile(layoutPath)
	if err != nil {
		return Report{}, err
	}
	var layout Layout
	if err := json.Unmarshal(raw, &layout); err != nil {
		return Report{}, fmt.Errorf("%s: %w", layoutPath, err)
	}

	dir, err := os.ReadFile(dirDoc)
	if err != nil {
		return Report{}, err
	}
	docPaths, err := ExtractDocPaths(string(dir))
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", dirDoc, err)
	}

	env, err := os.ReadFile(envDoc)
	if err != nil {
		return Report{}, err
	}
	docEnv, err := ExtractDocEnv(string(env))
	if err != nil {
		return Report{}, fmt.Errorf("%s: %w", envDoc, err)
	}

	return Compare(layout, docPaths, docEnv), nil
}

func format(r Report) string {
	if r.Empty() {
		return "No drift: the layout covers every path and path-like variable the docs list.\n"
	}
	var b strings.Builder
	section := func(title string, items []string) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s:\n", title)
		for _, i := range items {
			fmt.Fprintf(&b, "- `%s`\n", i)
		}
		b.WriteString("\n")
	}
	section("Paths the docs list that the layout doesn't cover (add them, or list them under untracked)", r.NewPaths)
	section("Layout paths the docs no longer list (removed, renamed, or now undocumented)", r.GonePaths)
	section("Path-like variables the docs list that the layout doesn't mention (use them, or list them under untracked_env)", r.NewEnv)
	section("Variables in the layout that the docs no longer list", r.GoneEnv)
	return b.String()
}
