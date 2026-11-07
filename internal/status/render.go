package status

import (
	"fmt"
	"io"
	"strings"

	"github.com/bahiamovementcenter/xdg-agents/internal/xdg"
)

// Marks for each state: symbols on a terminal, plain words otherwise
// (decision 17).
var marks = map[State][2]string{
	InPlace:            {"✓", "ok"},
	SymlinkedElsewhere: {"⚠", "elsewhere"},
	Stays:              {"~", "stays"},
	Legacy:             {"·", "legacy"},
	ToMove:             {"✗", "move"},
	Fixed:              {"⊘", "fixed"},
	NotAgents:          {"–", "other"},
	Unclassified:       {"!", "unknown"},
}

// Text writes the report as text. Plain uses words instead of symbols. It
// returns the first write error.
func Text(w io.Writer, r Report, home string, plain bool) error {
	var werr error
	printf := func(format string, args ...any) {
		if werr == nil {
			_, werr = fmt.Fprintf(w, format, args...)
		}
	}
	short := func(p string) string {
		if home != "" && (p == home || strings.HasPrefix(p, home+"/")) {
			return "~" + strings.TrimPrefix(p, home)
		}
		return p
	}
	mark := func(s State) string {
		if plain {
			return fmt.Sprintf("%-9s", marks[s][1])
		}
		return marks[s][0]
	}
	lock := " 🔒"
	if plain {
		lock = " (sensitive)"
	}

	printf("XDG directories\n")
	for _, cat := range xdg.Categories {
		d := r.XDG[cat]
		src := "(default)"
		switch d.Source {
		case "env":
			src = "(" + d.Var + ")"
		case "invalid":
			src = fmt.Sprintf("(%s ignored: %q isn't an absolute path)", d.Var, d.Ignored)
		case "unset":
			src = "(" + d.Var + " unset: no runtime directory)"
		}
		path := short(d.Path)
		if path == "" {
			path = "—"
		}
		printf("  %-8s %-24s %s\n", cat, path, src)
	}

	if len(r.Agents) == 0 {
		printf("\nNo agents with a layout are installed here.\n")
		return werr
	}

	for _, a := range r.Agents {
		printf("\n")
		var rootDesc []string
		for _, ro := range a.Roots {
			if !ro.Exists && !ro.FromEnv {
				continue
			}
			how := "default"
			if ro.Env != "" {
				how = ro.Env + " unset"
				if ro.FromEnv {
					how = ro.Env
				}
			}
			rootDesc = append(rootDesc, fmt.Sprintf("%s (%s)", short(ro.Path), how))
		}
		header := a.Name
		if !a.Installed {
			header += " — not installed"
		} else if len(rootDesc) > 0 {
			header += " — " + strings.Join(rootDesc, ", ")
		}
		printf("%s\n", header)

		line := func(it Item) {
			// Relative to the root it's in, if it's in one; otherwise the full
			// path (for ~/.claude.json, or a whole root such as the temp folder)
			name := short(it.Location)
			for _, ro := range a.Roots {
				if rel, ok := strings.CutPrefix(it.Location, ro.Path+"/"); ok {
					name = rel
				}
			}
			if it.Kind == "dir" {
				name += "/"
			}
			if it.Sensitive {
				name += lock
			}
			var rest []string
			if it.Target != "" && it.State != Stays && it.State != Fixed {
				rest = append(rest, "→ "+short(it.Target))
			}
			if it.Via != "" && it.State == ToMove {
				rest = append(rest, "via "+it.Via)
			}
			if it.Reason != "" {
				r := it.Reason
				if it.State == Stays && it.Via != "" {
					r += " (" + it.Via + ")"
				}
				rest = append(rest, r)
			}
			if it.Setting != "" {
				rest = append(rest, "or the "+it.Setting+" setting")
			}
			if !it.Present {
				rest = append(rest, "(not present)")
			}
			printf("%s\n", strings.TrimRight(fmt.Sprintf("    %s %-24s %s", mark(it.State), name, strings.Join(rest, "  ")), " "))
		}

		// Entries by category in the spec's order, then the other groups
		for _, cat := range xdg.Categories {
			var group []Item
			for _, it := range a.Items {
				if it.Category == cat && it.State != Legacy && it.State != NotAgents && it.State != Unclassified {
					group = append(group, it)
				}
			}
			if len(group) == 0 {
				continue
			}
			printf("  %s\n", cat)
			for _, it := range group {
				line(it)
			}
		}
		for _, g := range []struct {
			state State
			title string
		}{
			{Legacy, "legacy (no longer written; safe to delete)"},
			{NotAgents, "not " + a.Name + "'s"},
			{Unclassified, "unclassified (not in the layout: find out what it is, then add it)"},
		} {
			var group []Item
			for _, it := range a.Items {
				if it.State == g.state {
					group = append(group, it)
				}
			}
			if len(group) == 0 {
				continue
			}
			printf("  %s\n", g.title)
			for _, it := range group {
				line(it)
			}
		}

		var counts []string
		for _, s := range []State{InPlace, ToMove, SymlinkedElsewhere, Stays, Fixed, Legacy, NotAgents, Unclassified} {
			if n := a.Summary[s]; n > 0 {
				counts = append(counts, fmt.Sprintf("%d %s", n, strings.TrimSpace(marks[s][1])))
			}
		}
		if len(counts) > 0 {
			printf("  Summary: %s\n", strings.Join(counts, ", "))
		}
	}
	return werr
}
