// Package xdg resolves the XDG Base Directory variables as basedir 0.8 says
// (see the table in AGENTS.md): an unset or empty variable takes its
// default, a value that isn't an absolute path is invalid and ignored, and
// XDG_RUNTIME_DIR has no default.
package xdg

import "path/filepath"

// Category is one of the spec's kinds of file.
type Category string

const (
	Config  Category = "config"
	Data    Category = "data"
	State   Category = "state"
	Cache   Category = "cache"
	Runtime Category = "runtime"
)

// Categories in the spec's order, which is also the order reports use.
var Categories = []Category{Config, Data, State, Cache, Runtime}

// Dir is one resolved directory.
type Dir struct {
	Var  string `json:"var"`
	Path string `json:"path"` // empty when there's none (XDG_RUNTIME_DIR unset)
	// Where the value came from: "env", "default", "invalid" (the
	// variable held a relative path, so the default was used) or "unset"
	// (no value and no default)
	Source string `json:"source"`
	// The ignored value, when Source is "invalid"
	Ignored string `json:"ignored,omitempty"`
}

// Dirs are the resolved user directories, one per category.
type Dirs map[Category]Dir

var vars = map[Category]struct {
	name, def string // def is relative to HOME; "" means no default
}{
	Config:  {"XDG_CONFIG_HOME", ".config"},
	Data:    {"XDG_DATA_HOME", ".local/share"},
	State:   {"XDG_STATE_HOME", ".local/state"},
	Cache:   {"XDG_CACHE_HOME", ".cache"},
	Runtime: {"XDG_RUNTIME_DIR", ""},
}

// Resolve resolves the directories from getenv, so callers and tests can
// supply any environment.
func Resolve(getenv func(string) string) Dirs {
	home := getenv("HOME")
	dirs := Dirs{}
	for cat, v := range vars {
		d := Dir{Var: v.name}
		val := getenv(v.name)
		switch {
		case val != "" && filepath.IsAbs(val):
			d.Path, d.Source = filepath.Clean(val), "env"
		case val != "":
			d.Ignored, d.Source = val, "invalid"
		default:
			d.Source = "default"
		}
		if d.Path == "" {
			if v.def == "" || home == "" {
				if d.Source == "default" {
					d.Source = "unset"
				}
			} else {
				d.Path = filepath.Join(home, v.def)
			}
		}
		dirs[cat] = d
	}
	return dirs
}
