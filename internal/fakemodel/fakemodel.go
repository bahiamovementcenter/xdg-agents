// Package fakemodel is a scripted stand-in for a model API, so probes can
// run an agent through real sessions with no login, no network beyond
// localhost, and the same replies every time (decision 25 in
// docs/decisions.md).
//
// It speaks the Anthropic Messages API (POST /v1/messages, streaming or
// not). Each conversation follows one script: for the Nth assistant turn it
// calls the Nth tool from the script that the agent offers (a harmless shell
// command, writing a file in the probe's project, a to-do list), then ends
// with a text naming the tools it was offered and used. Tools are matched by
// purpose, from their names, so the script works whatever an agent calls
// them. The script's shape follows the faux provider in pi's ai package.
package fakemodel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// A step the script tries to make the agent take.
type step struct {
	purpose string
	names   *regexp.Regexp
}

// The script, in order. A step whose purpose the agent has no tool for is
// skipped.
var script = []step{
	{"shell", regexp.MustCompile(`(?i)^(bash|shell|run_shell_command|execute_command|exec_command)$`)},
	{"write", regexp.MustCompile(`(?i)^(write|write_file|create_file)$`)},
	{"todo", regexp.MustCompile(`(?i)^(todowrite|taskcreate|todo_write|write_todos|update_plan)$`)},
}

// Server is the fake model. Workdir is the probe's project folder: files
// the script writes go there, so nothing touches anything else.
type Server struct {
	Workdir string

	mu  sync.Mutex
	log []Event
}

// Event records what one request offered and what the script did, without
// any message content.
type Event struct {
	Path    string   `json:"path"`
	Model   string   `json:"model,omitempty"`
	Offered []string `json:"offered,omitempty"`
	Called  string   `json:"called,omitempty"`
	Purpose string   `json:"purpose,omitempty"`
}

// Log returns what the server has seen so far.
func (s *Server) Log() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Event(nil), s.log...)
}

func (s *Server) record(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = append(s.log, e)
}

type tool struct {
	Name        string          `json:"name"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type request struct {
	Model    string `json:"model"`
	Stream   bool   `json:"stream"`
	Tools    []tool `json:"tools"`
	Messages []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"messages"`
}

// A content block of the reply: text, or a tool call.
type block struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/messages":
		s.messages(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/messages/count_tokens":
		s.record(Event{Path: r.URL.Path})
		writeJSON(w, map[string]int{"input_tokens": 1})
	default:
		s.record(Event{Path: r.Method + " " + r.URL.Path})
		http.NotFound(w, r)
	}
}

func (s *Server) messages(w http.ResponseWriter, r *http.Request) {
	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	offered := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		offered = append(offered, t.Name)
	}
	sort.Strings(offered)

	// How far through the script this conversation is: the number of tool
	// calls the assistant has made so far. Counting assistant messages
	// instead fails with Claude Code, which merges consecutive assistant
	// turns into one message holding several tool calls.
	turn := 0
	for _, m := range req.Messages {
		if m.Role != "assistant" {
			continue
		}
		var blocks []struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(m.Content, &blocks) // a plain string has no tool calls
		for _, b := range blocks {
			if b.Type == "tool_use" {
				turn++
			}
		}
	}
	e := Event{Path: r.URL.Path, Model: req.Model, Offered: offered}

	var reply block
	stop := "end_turn"
	if t, purpose := s.nthTool(req.Tools, turn); t != nil {
		reply = block{Type: "tool_use", ID: fmt.Sprintf("toolu_probe_%d", turn), Name: t.Name, Input: s.input(t, purpose)}
		stop = "tool_use"
		e.Called, e.Purpose = t.Name, purpose
	} else {
		reply = block{Type: "text", Text: s.summary(offered)}
	}
	s.record(e)

	if req.Stream {
		stream(w, req.Model, reply, stop)
		return
	}
	writeJSON(w, map[string]any{
		"id": "msg_probe", "type": "message", "role": "assistant", "model": req.Model,
		"content": []block{reply}, "stop_reason": stop, "stop_sequence": nil,
		"usage": map[string]int{"input_tokens": 1, "output_tokens": 1},
	})
}

// nthTool returns the tool for the script's nth applicable step.
func (s *Server) nthTool(tools []tool, n int) (*tool, string) {
	i := 0
	for _, st := range script {
		for k := range tools {
			if st.names.MatchString(tools[k].Name) {
				if i == n {
					return &tools[k], st.purpose
				}
				i++
				break
			}
		}
	}
	return nil, ""
}

func (s *Server) summary(offered []string) string {
	var used []string
	for _, e := range s.Log() {
		if e.Called != "" {
			used = append(used, e.Called)
		}
	}
	return fmt.Sprintf("Probe finished. Tools offered: %s. Tools called: %s.",
		strings.Join(offered, ", "), strings.Join(used, ", "))
}

// input builds a tool call's arguments from the tool's own input schema:
// every required property gets a harmless value chosen by its name and type.
func (s *Server) input(t *tool, purpose string) json.RawMessage {
	var schema map[string]any
	_ = json.Unmarshal(t.InputSchema, &schema)
	v := s.value(schema, "", purpose)
	b, _ := json.Marshal(v)
	return b
}

func (s *Server) value(schema map[string]any, name, purpose string) any {
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		return enum[0]
	}
	typ, _ := schema["type"].(string)
	switch typ {
	case "object", "":
		out := map[string]any{}
		props, _ := schema["properties"].(map[string]any)
		required, _ := schema["required"].([]any)
		for _, r := range required {
			key, _ := r.(string)
			sub, _ := props[key].(map[string]any)
			out[key] = s.value(sub, key, purpose)
		}
		return out
	case "array":
		items, _ := schema["items"].(map[string]any)
		return []any{s.value(items, name, purpose)}
	case "boolean":
		return false
	case "integer", "number":
		return 1
	}
	switch strings.ToLower(name) {
	case "command", "cmd":
		return "echo probe"
	case "file_path", "path", "filename", "file":
		return s.Workdir + "/probe.txt"
	case "content", "text", "contents":
		return "probe\n"
	case "activeform":
		return "Probing"
	}
	return "probe " + purpose
}

func stream(w http.ResponseWriter, model string, b block, stop string) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	send := func(event string, data any) {
		j, _ := json.Marshal(data)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, j)
		if flusher != nil {
			flusher.Flush()
		}
	}
	send("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg_probe", "type": "message", "role": "assistant", "model": model,
		"content": []any{}, "stop_reason": nil, "stop_sequence": nil,
		"usage": map[string]int{"input_tokens": 1, "output_tokens": 1},
	}})
	start := map[string]any{"type": b.Type}
	var delta map[string]any
	if b.Type == "tool_use" {
		start["id"], start["name"], start["input"] = b.ID, b.Name, map[string]any{}
		delta = map[string]any{"type": "input_json_delta", "partial_json": string(b.Input)}
	} else {
		start["text"] = ""
		delta = map[string]any{"type": "text_delta", "text": b.Text}
	}
	send("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": start})
	send("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": delta})
	send("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
	send("message_delta", map[string]any{"type": "message_delta",
		"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]int{"output_tokens": 1}})
	send("message_stop", map[string]any{"type": "message_stop"})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
