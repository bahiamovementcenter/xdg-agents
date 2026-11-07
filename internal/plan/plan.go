// Package plan turns a status report into the exact steps that would put an
// agent's files where XDG says they belong: what to move, what to symlink,
// and which variables to set. It's read-only: building a plan changes
// nothing (AGENTS.md: "report" and "plan" are read-only, and "apply" only
// runs a plan the user has seen).
package plan

import (
	"path/filepath"
	"strings"

	"github.com/bahiamovementcenter/xdg-agents/internal/status"
)

// Kind is what a step does.
type Kind string

const (
	Move    Kind = "move"    // rename From to To
	Symlink Kind = "symlink" // create a symlink at From pointing to To
	SetEnv  Kind = "env"     // set Var to Value wherever the agent is started
	Note    Kind = "note"    // nothing to do; Reason says why
)

// Step is one step of a plan.
type Step struct {
	Kind   Kind   `json:"kind"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Var    string `json:"var,omitempty"`
	Value  string `json:"value,omitempty"`
	Path   string `json:"path,omitempty"` // for notes: what it's about
	Reason string `json:"reason,omitempty"`
}

// Agent is one agent's plan.
type Agent struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Steps []Step `json:"steps"` // moves and symlinks, in order
	Env   []Step `json:"env"`   // variables to set
	Notes []Step `json:"notes"`
}

// Plan is the whole plan.
type Plan struct {
	// Set when plans aren't made on this system, with the reason
	Refused string  `json:"refused,omitempty"`
	Agents  []Agent `json:"agents"`
}

// Build makes the plan for a status report. On macOS it refuses (ADR-0004:
// report only there).
func Build(r status.Report, goos string) Plan {
	if goos == "darwin" {
		return Plan{Refused: "on macOS xdg-agents only reports: ~/Library is the platform's own convention (ADR-0004)"}
	}
	var p Plan
	for _, a := range r.Agents {
		if a.Installed {
			p.Agents = append(p.Agents, agent(a))
		}
	}
	return p
}

func agent(a status.Agent) Agent {
	out := Agent{ID: a.ID, Name: a.Name}
	note := func(path, reason string) {
		out.Notes = append(out.Notes, Step{Kind: Note, Path: path, Reason: reason})
	}

	// Where each root will be once the plan has run
	newLoc := map[string]string{}
	moved := map[string]bool{}
	for _, ro := range a.Roots {
		newLoc[ro.Name] = ro.Path
		if !ro.Exists || ro.Env == "" || ro.Path == ro.Target {
			continue
		}
		if ro.Target == "" {
			note(ro.Path, "can't move: the "+ro.Category+" directory is unknown")
			continue
		}
		if ro.Category == "runtime" {
			// Temporary files: nothing to carry over, new ones go to the new place
			out.Env = append(out.Env, Step{Kind: SetEnv, Var: ro.Env, Value: ro.Target})
			newLoc[ro.Name] = ro.Target
			continue
		}
		if exists(ro.Target) {
			note(ro.Path, "can't move to "+ro.Target+": it already exists. Merge the two by hand, then plan again")
			continue
		}
		out.Steps = append(out.Steps, Step{Kind: Move, From: ro.Path, To: ro.Target})
		out.Env = append(out.Env, Step{Kind: SetEnv, Var: ro.Env, Value: ro.Target})
		newLoc[ro.Name], moved[ro.Name] = ro.Target, true
	}

	// Files the root's variable moves that live outside it (~/.claude.json)
	for _, it := range a.Items {
		ro := root(a, it.Root)
		if it.State == status.ToMove && moved[it.Root] && it.Via == ro.Env && !under(it.Location, ro.Path) {
			out.Steps = append(out.Steps, Step{Kind: Move, From: it.Location, To: it.Target})
		}
	}

	for _, it := range a.Items {
		ro := root(a, it.Root)
		// Where it will be once its root has moved
		loc := it.Location
		if rel, ok := strings.CutPrefix(it.Location, ro.Path+"/"); ok {
			loc = filepath.Join(newLoc[it.Root], rel)
		}
		switch it.State {
		case status.ToMove:
			switch {
			case it.Via == ro.Env && ro.Env != "":
				// Moves with its root, handled above
			case it.Target == "":
				note(loc, it.Reason)
			case it.Via == "symlink":
				out.Steps = append(out.Steps,
					Step{Kind: Move, From: loc, To: it.Target},
					Step{Kind: Symlink, From: loc, To: it.Target})
			default: // its own variable
				out.Steps = append(out.Steps, Step{Kind: Move, From: loc, To: it.Target})
				out.Env = append(out.Env, Step{Kind: SetEnv, Var: it.Via, Value: it.Target})
			}
			if it.Setting != "" {
				note(loc, "can also be moved with the "+it.Setting+" setting, set per project in .claude/settings.local.json (decision 9)")
			}
		case status.Stays:
			note(loc, it.Reason)
		case status.SymlinkedElsewhere:
			note(loc, "already a symlink, but not to "+it.Target+" ("+it.Reason+"): decide by hand whether to change it")
		case status.Legacy:
			note(loc, "legacy: no longer written, safe to delete (the plan doesn't delete anything)")
		case status.Unclassified, status.NotAgents:
			if moved[it.Root] {
				note(loc, "not in the layout: it moves with its root, so check what it is first")
			} else {
				note(loc, "not in the layout: left where it is")
			}
		}
	}
	return out
}

func root(a status.Agent, name string) status.RootInfo {
	for _, ro := range a.Roots {
		if ro.Name == name {
			return ro
		}
	}
	return status.RootInfo{}
}

func under(p, dir string) bool {
	return p == dir || strings.HasPrefix(p, dir+"/")
}
