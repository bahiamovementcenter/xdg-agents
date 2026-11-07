package apply

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Running looks through /proc for processes whose program is one of names,
// by the process name or the first two words of its command line (an agent
// run by an interpreter, such as `node …/claude`, shows as its second word).
// It never counts the current process. Where there's no /proc (macOS), it
// finds nothing; apply only plans and runs on Linux anyway.
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
		var candidates []string
		if comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm")); err == nil {
			candidates = append(candidates, strings.TrimSpace(string(comm)))
		}
		if cmd, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline")); err == nil {
			args := strings.Split(string(cmd), "\x00")
			for i := 0; i < len(args) && i < 2; i++ {
				candidates = append(candidates, filepath.Base(args[i]))
			}
		}
		for _, c := range candidates {
			if want[c] && !seen[c] {
				seen[c] = true
				found = append(found, c)
			}
		}
	}
	return found
}
