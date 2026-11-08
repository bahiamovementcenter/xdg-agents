package plan

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// Text writes the plan as text. Paths in ~ are shortened, and variable
// values are written with $HOME so they can be pasted into a shell profile.
// It returns the first write error.
func Text(w io.Writer, p Plan, home string) error {
	var werr error
	printf := func(format string, args ...any) {
		if werr == nil {
			_, werr = fmt.Fprintf(w, format, args...)
		}
	}
	short := func(s string) string {
		if home != "" && (s == home || strings.HasPrefix(s, home+"/")) {
			return "~" + strings.TrimPrefix(s, home)
		}
		return s
	}
	shellValue := func(s string) string {
		if home != "" && (s == home || strings.HasPrefix(s, home+"/")) {
			return `"$HOME` + strings.TrimPrefix(s, home) + `"`
		}
		return `"` + s + `"`
	}

	if p.Refused != "" {
		printf("No plan: %s.\n", p.Refused)
		return werr
	}
	if len(p.Agents) == 0 {
		printf("No agents with a layout are installed here, so there's nothing to plan.\n")
		return werr
	}
	for i, a := range p.Agents {
		if i > 0 {
			printf("\n")
		}
		printf("Plan for %s (nothing has been changed)\n", a.Name)
		if len(a.Steps) == 0 && len(a.Env) == 0 {
			printf("  Nothing to do: everything that can move is in place.\n")
		} else {
			printf("  Quit %s first.\n", a.Name)
			n := 0
			for _, s := range a.Steps {
				switch s.Kind {
				case Move:
					n++
					printf("  %2d. move  %s → %s\n", n, short(s.From), short(s.To))
				case Symlink:
					printf("      link  %s → %s\n", short(s.From), short(s.To))
				}
			}
			if len(a.Env) > 0 {
				printf("  Then set these wherever %s is started (your shell profile, for example):\n", a.Name)
				for _, e := range a.Env {
					printf("      export %s=%s\n", e.Var, shellValue(e.Value))
				}
			}
		}
		if len(a.Notes) > 0 {
			printf("  Notes:\n")
			for _, n := range a.Notes {
				printf("    - %s: %s\n", short(n.Path), n.Reason)
			}
		}
	}
	printf("\n`xdg-agents apply` carries out this plan, reversibly, after checking it and asking (docs/apply.md).\n")
	return werr
}
