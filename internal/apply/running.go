package apply

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Running looks through /proc for processes running one of names. It never
// counts the current process. Where there's no /proc (macOS), it finds
// nothing; apply only plans and runs on Linux anyway.
func Running(names []string) []string {
	want := map[string]bool{}
	for _, n := range names {
		if n != "" {
			want[n] = true
		}
	}
	self := os.Getpid()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var found []string
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid == self {
			continue
		}
		comm, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		cmd, _ := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if n := matchProcess(strings.TrimSpace(string(comm)), strings.Split(string(cmd), "\x00"), want); n != "" && !seen[n] {
			seen[n] = true
			found = append(found, n)
		}
	}
	return found
}

// Programs that run an agent as a script: the script, not the program,
// names the agent.
var interpreters = map[string]bool{"node": true, "nodejs": true, "bun": true, "deno": true, "python": true, "python3": true}

// matchProcess returns which wanted name a process is running, or "". It
// looks at the process name, then at every folder and file name in the
// program's path, and, when the program is an interpreter, in the script's
// path: an agent run that way shows only there, often only as a folder, as
// in `node …/lib/openclaw/dist/index.js gateway`. Any other argument, such
// as a file an editor has open, is ignored.
func matchProcess(comm string, args []string, want map[string]bool) string {
	if want[comm] {
		return comm
	}
	if len(args) == 0 {
		return ""
	}
	paths := []string{args[0]}
	if len(args) > 1 && (interpreters[comm] || interpreters[filepath.Base(args[0])]) {
		paths = append(paths, args[1])
	}
	for _, p := range paths {
		for _, part := range strings.Split(p, "/") {
			if want[part] {
				return part
			}
		}
	}
	return ""
}
