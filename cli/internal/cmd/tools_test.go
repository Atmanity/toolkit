package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeToolsAPI answers /v1/session, recording each request body.
func fakeToolsAPI(t *testing.T) *[]map[string]any {
	t.Helper()
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/session" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"billingMode": "reserve", "roomName": "rk-1", "serverUrl": "wss://example.livekit.cloud",
			"sessionId": "11111111-2222-4333-8444-555555555555", "userToken": "tok",
		})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_0123456789abcdef")
	return &bodies
}

const sampleTools = `[
  {"type": "function", "name": "show_slide", "description": "Put slide n up.",
   "parameters": {"type": "object", "properties": {"n": {"type": "integer"}}, "required": ["n"]},
   "x-atmee": {"mode": "notify"}},
  {"type": "function", "name": "ask_quiz", "description": "Ask the trainee a quiz question and wait for the answer.",
   "parameters": {"type": "object", "properties": {"topic": {"type": "string"}}},
   "x-atmee": {"mode": "await", "timeoutSeconds": 20}}
]`

func writeIn(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const avatarID = "11111111-2222-4333-8444-555555555555"

func sameJSON(t *testing.T, got any, want string) bool {
	t.Helper()
	var w any
	_ = json.Unmarshal([]byte(want), &w)
	g, _ := json.Marshal(got)
	wj, _ := json.Marshal(w)
	return string(g) == string(wj)
}

func TestChatSendsToolsAndInstructionsAsWritten(t *testing.T) {
	isolate(t)
	bodies := fakeToolsAPI(t)
	recordBrowse(t)
	tools := writeIn(t, t.TempDir(), "tools.json", sampleTools)

	out, errOut, code := run(t, "  Show slides as you teach them.\n", "avatars", "session", "chat", avatarID,
		"--tools", tools, "--tool-instructions", "-")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	b := (*bodies)[0]
	if _, top := b["tools"]; top || b["instructions"] != nil {
		t.Fatalf("tools and instructions go inside appTools: %v", b)
	}
	app, _ := b["appTools"].(map[string]any)
	if app["instructions"] != "Show slides as you teach them." {
		t.Fatalf("instructions: %#v", app)
	}
	if !sameJSON(t, app["tools"], sampleTools) {
		t.Fatalf("tools are sent as written: %v", app["tools"])
	}
	if len(app) != 2 {
		t.Fatalf("appTools carries only tools and instructions: %v", app)
	}
	if jsonOf(t, out)["roomName"] != "rk-1" {
		t.Fatalf("stdout stays the API's JSON: %q", out)
	}
}

func TestChatToolFileMistakesAreRefusedBeforeAnythingIsSent(t *testing.T) {
	isolate(t)
	bodies := fakeToolsAPI(t)
	recordBrowse(t)
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"not JSON", []string{"--tools", writeIn(t, dir, "bad.json", "{")}, "not a JSON array"},
		{"an object", []string{"--tools", writeIn(t, dir, "obj.json", `{"name":"x"}`)}, "not a JSON array"},
		{"empty array", []string{"--tools", writeIn(t, dir, "none.json", `[]`)}, "empty"},
		{"not an object", []string{"--tools", writeIn(t, dir, "s.json", `["a"]`)}, "not an object with a string name"},
		{"no name", []string{"--tools", writeIn(t, dir, "nn.json", `[{"type":"function"}]`)}, "tool 1 is not an object with a string name"},
		{"a number name", []string{"--tools", writeIn(t, dir, "num.json", `[{"name":1}]`)}, "not an object with a string name"},
		{"missing file", []string{"--tools", filepath.Join(dir, "nope.json")}, "--tools"},
		{"empty instructions", []string{"--tools", writeIn(t, dir, "ok.json", sampleTools), "--tool-instructions", writeIn(t, dir, "empty.txt", " \n")}, "is empty"},
		{"instructions without tools", []string{"--tool-instructions", writeIn(t, dir, "i.txt", "Use them.")}, "--tool-instructions needs --tools"},
	}
	for _, tc := range cases {
		_, errOut, code := run(t, "", append([]string{"avatars", "session", "chat", avatarID}, tc.args...)...)
		if code != 2 || !strings.Contains(errOut, tc.want) {
			t.Errorf("%s: %d %q", tc.name, code, errOut)
		}
	}
	if len(*bodies) != 0 {
		t.Fatalf("%d requests were sent", len(*bodies))
	}
}

// The rules (names, counts, values) are the API's: a tool the API would
// refuse is sent as written, and its invalid_tools answer is the error.
func TestChatLeavesToolRulesToTheAPI(t *testing.T) {
	isolate(t)
	bodies := fakeToolsAPI(t)
	recordBrowse(t)
	odd := `[{"type":"web","name":"Show-Slide","x-atmee":{"mode":"later","timeoutSeconds":99}}]`
	if _, errOut, code := run(t, "", "avatars", "session", "chat", avatarID, "--tools", writeIn(t, t.TempDir(), "odd.json", odd)); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	app, _ := (*bodies)[0]["appTools"].(map[string]any)
	if !sameJSON(t, app["tools"], odd) || app["instructions"] != nil {
		t.Fatalf("sent %v", (*bodies)[0])
	}
}

func TestChatWithoutToolFlagsSendsNoToolFields(t *testing.T) {
	isolate(t)
	bodies := fakeToolsAPI(t)
	recordBrowse(t)
	if _, errOut, code := run(t, "", "avatars", "session", "chat", avatarID); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, k := range []string{"appTools", "tools", "instructions"} {
		if _, ok := (*bodies)[0][k]; ok {
			t.Errorf("%s sent: %v", k, (*bodies)[0])
		}
	}
}

// LiveKit Meet answers no atmee.tool RPC: with tools the command says so.
func TestChatWithToolsWarnsThatMeetDoesNotAnswerToolCalls(t *testing.T) {
	isolate(t)
	fakeToolsAPI(t)
	recordBrowse(t)
	tools := writeIn(t, t.TempDir(), "tools.json", sampleTools)
	for _, extra := range [][]string{{"--open"}, nil} {
		args := append([]string{"avatars", "session", "chat", avatarID, "--tools", tools}, extra...)
		_, errOut, code := run(t, "", args...)
		if code != 0 || !strings.Contains(errOut, "LiveKit Meet") || !strings.Contains(errOut, "atmee.tool") {
			t.Fatalf("%v: %d %q", extra, code, errOut)
		}
	}
	if _, errOut, _ := run(t, "", "avatars", "session", "chat", avatarID, "--open"); strings.Contains(errOut, "atmee.tool") {
		t.Fatalf("no tools, no warning: %q", errOut)
	}
}
