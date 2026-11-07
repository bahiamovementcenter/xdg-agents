package fakemodel

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const tools = `[
  {"name": "Bash", "input_schema": {"type": "object", "required": ["command"],
    "properties": {"command": {"type": "string"}, "timeout": {"type": "number"}}}},
  {"name": "Read", "input_schema": {"type": "object"}},
  {"name": "Write", "input_schema": {"type": "object", "required": ["file_path", "content"],
    "properties": {"file_path": {"type": "string"}, "content": {"type": "string"}}}},
  {"name": "TodoWrite", "input_schema": {"type": "object", "required": ["todos"],
    "properties": {"todos": {"type": "array", "items": {"type": "object",
      "required": ["content", "status", "activeForm"],
      "properties": {"content": {"type": "string"}, "activeForm": {"type": "string"},
        "status": {"type": "string", "enum": ["pending", "in_progress", "completed"]}}}}}}}
]`

// post sends a request offering toolsJSON, whose history holds earlier
// calls to the named tools, one assistant message each.
func post(t *testing.T, s *Server, toolsJSON string, called []string, stream bool) *http.Response {
	t.Helper()
	msgs := []any{map[string]any{"role": "user", "content": "probe"}}
	for _, name := range called {
		msgs = append(msgs,
			map[string]any{"role": "assistant", "content": []map[string]string{{"type": "tool_use", "name": name}}},
			map[string]any{"role": "user", "content": []map[string]string{{"type": "tool_result"}}})
	}
	body, _ := json.Marshal(map[string]any{
		"model": "m", "stream": stream, "messages": msgs, "tools": json.RawMessage(toolsJSON),
	})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body))))
	return rec.Result()
}

type reply struct {
	StopReason string  `json:"stop_reason"`
	Content    []block `json:"content"`
}

func decode(t *testing.T, resp *http.Response) reply {
	t.Helper()
	var r reply
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestScriptFollowsToolsInOrder(t *testing.T) {
	s := &Server{Workdir: "/probe/project"}
	want := []struct{ name, input string }{
		{"Bash", `{"command":"echo probe"}`},
		{"Write", `{"content":"probe\n","file_path":"/probe/project/probe.txt"}`},
		{"TodoWrite", `{"todos":[{"activeForm":"Probing","content":"probe\n","status":"pending"}]}`},
	}
	var called []string
	for turn, w := range want {
		msg := decode(t, post(t, s, tools, called, false))
		if msg.StopReason != "tool_use" || msg.Content[0].Name != w.name || string(msg.Content[0].Input) != w.input {
			t.Errorf("turn %d: got %s %s (%s), want %s %s", turn, msg.Content[0].Name, msg.Content[0].Input, msg.StopReason, w.name, w.input)
		}
		called = append(called, w.name)
	}
	// After the script: a closing text naming what was offered and called
	last := decode(t, post(t, s, tools, called, false))
	if last.StopReason != "end_turn" || !strings.Contains(last.Content[0].Text, "Tools called: Bash, Write, TodoWrite") {
		t.Errorf("closing reply = %+v", last)
	}
}

func TestStreamingEvents(t *testing.T) {
	resp := post(t, &Server{Workdir: "/p"}, tools, nil, true)
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	var events []string
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if e, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
			events = append(events, e)
		}
	}
	want := "message_start content_block_start content_block_delta content_block_stop message_delta message_stop"
	if got := strings.Join(events, " "); got != want {
		t.Errorf("events = %s, want %s", got, want)
	}
}

func TestLogHasNoContent(t *testing.T) {
	s := &Server{Workdir: "/p"}
	post(t, s, tools, nil, false)
	got := s.Log()[0]
	if got.Called != "Bash" || got.Purpose != "shell" || len(got.Offered) != 4 {
		t.Errorf("log = %+v", got)
	}
	// Only these fields may ever be logged: no prompts, replies or inputs
	var fields map[string]any
	b, _ := json.Marshal(got)
	_ = json.Unmarshal(b, &fields)
	for k := range fields {
		switch k {
		case "path", "protocol", "model", "offered", "called", "purpose":
		default:
			t.Errorf("log has field %q", k)
		}
	}
}

// Claude Code merges consecutive assistant turns into one message with
// several tool calls; the script must still move on.
func TestMergedAssistantTurns(t *testing.T) {
	s := &Server{Workdir: "/p"}
	body := `{"model":"m","tools":` + tools + `,"messages":[
	  {"role":"user","content":"probe"},
	  {"role":"assistant","content":[{"type":"tool_use","name":"Bash"},{"type":"tool_use","name":"Write"}]},
	  {"role":"user","content":[{"type":"tool_result"},{"type":"tool_result"}]}]}`
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body)))
	var msg struct {
		Content []block `json:"content"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&msg); err != nil {
		t.Fatal(err)
	}
	if msg.Content[0].Name != "TodoWrite" {
		t.Errorf("third step = %+v, want TodoWrite", msg.Content[0])
	}
}

// Claude Code offers its task tools only through ToolSearch: the script
// searches for them, calls one once it's offered, and gives up after one
// search that doesn't bring one in.
func TestToolSearchUnlocksTodo(t *testing.T) {
	const searchOnly = `[
	  {"name": "Bash", "input_schema": {"type": "object", "required": ["command"], "properties": {"command": {"type": "string"}}}},
	  {"name": "Write", "input_schema": {"type": "object"}},
	  {"name": "ToolSearch", "input_schema": {"type": "object", "required": ["query", "max_results"],
	    "properties": {"query": {"type": "string"}, "max_results": {"type": "number"}}}}
	]`
	const afterSearch = `[
	  {"name": "Bash", "input_schema": {"type": "object"}},
	  {"name": "ToolSearch", "input_schema": {"type": "object"}},
	  {"name": "TaskCreate", "input_schema": {"type": "object", "required": ["subject"], "properties": {"subject": {"type": "string"}}}}
	]`
	s := &Server{Workdir: "/p"}

	got := decode(t, post(t, s, searchOnly, []string{"Bash", "Write"}, false)).Content[0]
	if got.Name != "ToolSearch" || string(got.Input) != `{"max_results":5,"query":"todo task list"}` {
		t.Errorf("without a to-do tool: got %s %s, want a ToolSearch for it", got.Name, got.Input)
	}

	got = decode(t, post(t, s, afterSearch, []string{"Bash", "Write", "ToolSearch"}, false)).Content[0]
	if got.Name != "TaskCreate" || string(got.Input) != `{"subject":"probe todo"}` {
		t.Errorf("after the search: got %s %s, want TaskCreate", got.Name, got.Input)
	}

	r := decode(t, post(t, s, searchOnly, []string{"Bash", "Write", "ToolSearch"}, false))
	if r.StopReason != "end_turn" {
		t.Errorf("a search that found nothing must not repeat: got %+v", r.Content[0])
	}
}

func TestProtocolFromPath(t *testing.T) {
	for path, want := range map[string]string{
		"/v1/messages":                           "anthropic",
		"/v1/chat/completions":                   "openai-chat",
		"/chat/completions":                      "openai-chat",
		"/v1/responses":                          "openai-responses",
		"/v1beta/models/g:generateContent":       "gemini",
		"/v1beta/models/g:streamGenerateContent": "gemini",
		"/healthz":                               "",
	} {
		if got := Protocol(path); got != want {
			t.Errorf("Protocol(%q) = %q, want %q", path, got, want)
		}
	}
	// Protocols not answered yet are recorded and refused, not guessed
	s := &Server{}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}")))
	if rec.Code != http.StatusNotImplemented || s.Log()[0].Protocol != "openai-chat" {
		t.Errorf("got %d %+v", rec.Code, s.Log())
	}
}
