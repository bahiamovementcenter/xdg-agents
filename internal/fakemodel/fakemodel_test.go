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

// post sends a request whose history holds the given number of earlier
// tool calls, one assistant message each.
func post(t *testing.T, s *Server, toolCalls int, stream bool) *http.Response {
	t.Helper()
	msgs := []any{map[string]any{"role": "user", "content": "probe"}}
	for i := 0; i < toolCalls; i++ {
		msgs = append(msgs,
			map[string]any{"role": "assistant", "content": []map[string]string{{"type": "tool_use"}}},
			map[string]any{"role": "user", "content": []map[string]string{{"type": "tool_result"}}})
	}
	body, _ := json.Marshal(map[string]any{
		"model": "m", "stream": stream, "messages": msgs, "tools": json.RawMessage(tools),
	})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(string(body))))
	return rec.Result()
}

func TestScriptFollowsToolsInOrder(t *testing.T) {
	s := &Server{Workdir: "/probe/project"}
	want := []struct{ name, input string }{
		{"Bash", `{"command":"echo probe"}`},
		{"Write", `{"content":"probe\n","file_path":"/probe/project/probe.txt"}`},
		{"TodoWrite", `{"todos":[{"activeForm":"Probing","content":"probe\n","status":"pending"}]}`},
	}
	for turn, w := range want {
		var msg struct {
			StopReason string  `json:"stop_reason"`
			Content    []block `json:"content"`
		}
		if err := json.NewDecoder(post(t, s, turn, false).Body).Decode(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.StopReason != "tool_use" || msg.Content[0].Name != w.name || string(msg.Content[0].Input) != w.input {
			t.Errorf("turn %d: got %s %s (%s), want %s %s", turn, msg.Content[0].Name, msg.Content[0].Input, msg.StopReason, w.name, w.input)
		}
	}
	// After the script: a closing text naming what was offered and called
	var last struct {
		StopReason string  `json:"stop_reason"`
		Content    []block `json:"content"`
	}
	if err := json.NewDecoder(post(t, s, len(want), false).Body).Decode(&last); err != nil {
		t.Fatal(err)
	}
	if last.StopReason != "end_turn" || !strings.Contains(last.Content[0].Text, "Tools called: Bash, Write, TodoWrite") {
		t.Errorf("closing reply = %+v", last)
	}
}

func TestStreamingEvents(t *testing.T) {
	resp := post(t, &Server{Workdir: "/p"}, 0, true)
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
	post(t, s, 0, false)
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
		case "path", "model", "offered", "called", "purpose":
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
