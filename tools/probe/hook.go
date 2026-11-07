package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// hookSettings is a settings file that runs this program as a hook on the
// events that carry transcript_path. It's passed with --settings, from
// outside the agent's own folders, so nothing is seeded into them.
func hookSettings(hookLog string) ([]byte, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	command := fmt.Sprintf("'%s' -record-hook '%s'", exe, hookLog)
	h := []map[string]any{{"type": "command", "command": command}}
	return json.Marshal(map[string]any{"hooks": map[string]any{
		"SessionStart": []map[string]any{{"hooks": h}},
		"PostToolUse":  []map[string]any{{"matcher": "*", "hooks": h}},
		"Stop":         []map[string]any{{"hooks": h}},
	}})
}

// recordHookCall is the hook: it reads the hook's JSON from stdin and
// appends only the event name and transcript_path. The rest of the payload
// (tool inputs among it) is never written. It always exits 0, so it can't
// block the agent.
func recordHookCall(file string) {
	var payload struct {
		Event      string `json:"hook_event_name"`
		Transcript string `json:"transcript_path"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&payload); err != nil {
		return
	}
	line, _ := json.Marshal(map[string]string{"event": payload.Event, "transcript_path": payload.Transcript})
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	_, _ = f.Write(append(line, '\n'))
	_ = f.Close()
}

// hookTranscripts returns the distinct transcript paths the hook recorded,
// as labels.
func hookTranscripts(root, hookLog string) []string {
	raw, err := os.ReadFile(hookLog)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var rec struct {
			Transcript string `json:"transcript_path"`
		}
		if json.Unmarshal([]byte(line), &rec) != nil || rec.Transcript == "" {
			continue
		}
		l := label(root, rec.Transcript)
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	return out
}
