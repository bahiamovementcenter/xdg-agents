package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// layout is the part of an exported agent layout the probe compares with.
type layout struct {
	Roots map[string]struct {
		Default string `json:"default"`
		Env     string `json:"env"`
	} `json:"roots"`
	Paths []struct {
		Root        string `json:"root"`
		Path        string `json:"path"`
		DefaultPath string `json:"default_path"`
	} `json:"paths"`
	Untracked []struct {
		Path string `json:"path"`
	} `json:"untracked"`
}

func loadLayout(path string) (*layout, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l layout
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &l, nil
}

// envVars returns the variables the layout says move its roots.
func (l *layout) envVars() []string {
	if l == nil {
		return nil
	}
	var out []string
	for _, r := range l.Roots {
		if r.Env != "" {
			out = append(out, r.Env)
		}
	}
	return out
}

// unknown lists what the run left in the layout's home-folder roots (and at
// entries' own default paths) that no layout path covers. A layout path
// covers itself and everything inside it; only the outermost uncovered
// folder is listed.
func (l *layout) unknown(r Run) []string {
	var out []string
	for name, root := range l.Roots {
		if !strings.HasPrefix(root.Default, "~/") {
			continue
		}
		var paths []string
		for _, p := range l.Paths {
			if p.Root == name {
				paths = append(paths, p.Path)
			}
		}
		for _, u := range l.Untracked {
			paths = append(paths, u.Path)
		}
		covered := func(rel string) bool {
			for _, p := range paths {
				if rel == p || strings.HasPrefix(rel, p+"/") {
					return true
				}
			}
			return false
		}
		for _, e := range r.Entries {
			rel, ok := strings.CutPrefix(e.Path, root.Default+"/")
			if !ok || covered(rel) {
				continue
			}
			nested := false
			for _, o := range out {
				if strings.HasPrefix(root.Default+"/"+rel, o+"/") {
					nested = true
				}
			}
			if !nested {
				out = append(out, root.Default+"/"+rel)
			}
		}
	}
	// Files in ~ the layout gives an own default location to are covered
	for _, t := range r.tops() {
		if t == "~" {
			continue
		}
		known := false
		for _, root := range l.Roots {
			if t == topOf(root.Default) {
				known = true
			}
		}
		for _, p := range l.Paths {
			if p.DefaultPath != "" && t == topOf(p.DefaultPath) {
				known = true
			}
		}
		if !known {
			out = append(out, t)
		}
	}
	return out
}
