package main

import (
	"regexp"
	"sort"
	"strings"
)

// How to run an agent without hand-written setup (decision 26): everything
// here is guessed from the agent's own --help, and tried in turn.

// promptMark stands for the prompt in an invocation.
const promptMark = "{prompt}"

// invocations returns the ways to run the agent once, non-interactively,
// that its help text suggests, most likely first. The plain positional form
// comes last, as a fallback.
func invocations(help string) [][]string {
	type form struct {
		re   *regexp.Regexp
		args []string
	}
	forms := []form{
		{regexp.MustCompile(`(?m)(^|\s)-p, --print\b|--print\b`), []string{"-p", promptMark}},
		{regexp.MustCompile(`(?m)^\s+exec\b`), []string{"exec", promptMark}},
		{regexp.MustCompile(`(?m)^\s+run\b`), []string{"run", promptMark}},
		{regexp.MustCompile(`(?m)(^|\s)-p, --prompt\b`), []string{"-p", promptMark}},
		{regexp.MustCompile(`(?m)--prompt\b`), []string{"--prompt", promptMark}},
	}
	var out [][]string
	seen := map[string]bool{}
	for _, f := range forms {
		key := strings.Join(f.args, " ")
		if f.re.MatchString(help) && !seen[key] {
			seen[key] = true
			out = append(out, f.args)
		}
	}
	return append(out, []string{promptMark})
}

// permissionArgs returns the flags that let the agent use its tools without
// asking, from the first form its help mentions. A list of allowed tools
// comes first: blanket flags such as --dangerously-skip-permissions are
// refused by some agents when run as root.
func permissionArgs(help string) []string {
	tools := "Bash Write Edit ToolSearch TodoWrite TaskCreate TaskUpdate shell write_file"
	for _, f := range []struct {
		flag string
		args []string
	}{
		{"--allowedTools", []string{"--allowedTools", tools}},
		{"--allowed-tools", []string{"--allowed-tools", tools}},
		{"--dangerously-bypass-approvals-and-sandbox", []string{"--dangerously-bypass-approvals-and-sandbox"}},
		{"--full-auto", []string{"--full-auto"}},
		{"--yolo", []string{"--yolo"}},
		{"--dangerously-skip-permissions", []string{"--dangerously-skip-permissions"}},
	} {
		if strings.Contains(help, f.flag) {
			return f.args
		}
	}
	return nil
}

// supportsSettings reports whether the agent takes a --settings file, which
// the probe uses to install its transcript hook from outside the agent's
// own folders.
func supportsSettings(help string) bool {
	return regexp.MustCompile(`--settings\b`).MatchString(help)
}

var envName = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*_(?:HOME|CONFIG_DIR|CONFIG_HOME|DATA_DIR|DIR)\b`)

// configVars returns the variables that might move the agent's folders:
// any the layout names, any *_HOME or *_CONFIG_DIR named in the agent's
// help, and the usual ones derived from its command name (codex →
// CODEX_HOME). The XDG variables themselves are left out: every run already
// sets them.
func configVars(help, command string, fromLayout []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(v string) {
		if v == "" || seen[v] || strings.HasPrefix(v, "XDG_") {
			return
		}
		seen[v] = true
		out = append(out, v)
	}
	for _, v := range fromLayout {
		add(v)
	}
	found := envName.FindAllString(help, -1)
	sort.Strings(found)
	for _, v := range found {
		add(v)
	}
	name := strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(command, "_"))
	add(name + "_HOME")
	add(name + "_CONFIG_DIR")
	return out
}

// modelEnv points every common model API at the fake model at once, with a
// dummy key for each, so no agent needs telling which it speaks: the fake
// model tells the protocol from the request path.
func modelEnv(baseURL string) map[string]string {
	const key = "probe-not-a-real-key"
	return map[string]string{
		"ANTHROPIC_BASE_URL":     baseURL,
		"ANTHROPIC_API_KEY":      key,
		"OPENAI_BASE_URL":        baseURL + "/v1",
		"OPENAI_API_BASE":        baseURL + "/v1",
		"OPENAI_API_KEY":         key,
		"GOOGLE_GEMINI_BASE_URL": baseURL,
		"GEMINI_API_KEY":         key,
		"GOOGLE_API_KEY":         key,
		// Keep agents from calling home: updates, telemetry, error reports
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"DISABLE_AUTOUPDATER":                      "1",
		"DISABLE_TELEMETRY":                        "1",
		"DO_NOT_TRACK":                             "1",
	}
}
