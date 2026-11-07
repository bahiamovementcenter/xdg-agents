// Package fakemodel is a scripted stand-in for a model API, so probes can
// run an agent through real sessions with no login, no network beyond
// localhost, and the same replies every time (decision 25 in
// docs/decisions.md).
//
// It speaks the Anthropic Messages API (POST /v1/messages, streaming or
// not). Each conversation follows one script: a harmless shell command,
// writing a file in the probe's project, a to-do list, then a text naming the
// tools it was offered and used. Tools are matched by purpose, from their
// names, so the script works whatever an agent calls them. A step whose tool
// isn't offered is skipped, unless the agent offers a tool search (Claude
// Code's ToolSearch loads some tools only on request): then the script
// searches for it first. The script's shape follows the faux provider in
// pi's ai package.
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

// A step the script tries to make the agent take. Search is what to look
// for with the agent's tool search when no tool for the step is offered.
type step struct {
	purpose string
	names   *regexp.Regexp
	search  string
}

// The script, in order. A step whose purpose the agent has no tool for is
// skipped.
var script = []step{
	{"shell", regexp.MustCompile(`(?i)^(bash|shell|run_shell_command|execute_command|exec_command)$`), ""},
	{"write", regexp.MustCompile(`(?i)^(write|write_file|create_file)$`), ""},
	{"todo", regexp.MustCompile(`(?i)^(todowrite|taskcreate|todo_write|write_todos|update_plan)$`), "todo task list"},
}

// Tools that load other tools on request.
var searchTool = regexp.MustCompile(`(?i)^(toolsearch|tool_search|search_tools)$`)

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
	Path string `json:"path"`
	// The model API the request used, told from its path: anthropic,
	// openai-chat, openai-responses or gemini. Only anthropic is answered
	// so far; the others are recorded and refused, so a probe reports which
	// protocol an agent speaks instead of guessing
	Protocol string   `json:"protocol,omitempty"`
	Model    string   `json:"model,omitempty"`
	Offered  []string `json:"offered,omitempty"`
	Called   string   `json:"called,omitempty"`
	Purpose  string   `json:"purpose,omitempty"`
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

// Protocol tells which model API a request path belongs to, or "".
func Protocol(path string) string {
	switch {
	case strings.HasSuffix(path, "/messages"), strings.HasSuffix(path, "/messages/count_tokens"):
		return "anthropic"
	case strings.HasSuffix(path, "/chat/completions"):
		return "openai-chat"
	case strings.HasSuffix(path, "/responses"):
		return "openai-responses"
	case strings.Contains(path, ":generateContent"), strings.Contains(path, ":streamGenerateContent"):
		return "gemini"
	}
	return ""
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proto := Protocol(r.URL.Path)
	switch {
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/messages/count_tokens"):
		s.record(Event{Path: r.URL.Path, Protocol: proto})
		writeJSON(w, map[string]int{"input_tokens": 1})
	case r.Method == http.MethodPost && proto == "anthropic":
		s.messages(w, r)
	case proto != "":
		s.record(Event{Path: r.URL.Path, Protocol: proto})
		http.Error(w, "fakemodel: the "+proto+" protocol isn't answered yet", http.StatusNotImplemented)
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

	// The tools already called in this conversation, from its history. The
	// script works out where it is from these rather than by counting turns:
	// Claude Code merges consecutive assistant turns into one message, and a
	// tool search changes the tools on offer mid-conversation.
	var called []string
	for _, m := range req.Messages {
		if m.Role != "assistant" {
			continue
		}
		var blocks []struct {
			Type string `json:"type"`
			Name string `json:"name"`
		}
		_ = json.Unmarshal(m.Content, &blocks) // a plain string has no tool calls
		for _, b := range blocks {
			if b.Type == "tool_use" {
				called = append(called, b.Name)
			}
		}
	}
	e := Event{Path: r.URL.Path, Protocol: "anthropic", Model: req.Model, Offered: offered}

	var reply block
	stop := "end_turn"
	if t, purpose, query := next(req.Tools, called); t != nil {
		input := s.input(t, purpose)
		if query != "" {
			input = searchInput(t, query)
		}
		reply = block{Type: "tool_use", ID: fmt.Sprintf("toolu_probe_%d", len(called)), Name: t.Name, Input: input}
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

// next returns the tool for the script's next step, given the tools already
// called. For a step whose tool isn't offered, it returns the agent's tool
// search with what to search for, once; if the search didn't bring the tool
// in, the step is skipped.
func next(tools []tool, called []string) (*tool, string, string) {
	done := func(re *regexp.Regexp) bool {
		for _, c := range called {
			if re.MatchString(c) {
				return true
			}
		}
		return false
	}
	find := func(re *regexp.Regexp) *tool {
		for k := range tools {
			if re.MatchString(tools[k].Name) {
				return &tools[k]
			}
		}
		return nil
	}
	searched := done(searchTool)
	for _, st := range script {
		if done(st.names) {
			continue
		}
		if t := find(st.names); t != nil {
			return t, st.purpose, ""
		}
		if st.search != "" && !searched {
			if t := find(searchTool); t != nil {
				return t, "search", st.search
			}
		}
	}
	return nil, "", ""
}

// searchInput fills a tool search's string arguments with the query.
func searchInput(t *tool, query string) json.RawMessage {
	var schema struct {
		Required   []string                  `json:"required"`
		Properties map[string]map[string]any `json:"properties"`
	}
	_ = json.Unmarshal(t.InputSchema, &schema)
	in := map[string]any{}
	for _, k := range schema.Required {
		switch schema.Properties[k]["type"] {
		case "integer", "number":
			in[k] = 5
		default:
			in[k] = query
		}
	}
	if len(schema.Required) == 0 {
		in["query"] = query
	}
	b, _ := json.Marshal(in)
	return b
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
