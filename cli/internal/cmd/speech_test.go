package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const liveID = "22222222-3333-4444-8555-666666666666"

// fakeLiveAPI answers the speech session routes; the returned func lists the
// calls it saw so far. connected makes the session hold an open connection:
// its end is then requested, not settled.
func fakeLiveAPI(t *testing.T, connected bool) func() []seen {
	t.Helper()
	var mu sync.Mutex
	var calls []seen
	ends := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, seen{r.Method, r.URL.Path, b})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/speech_sessions":
			resp := map[string]any{
				"sessionId": liveID, "url": "wss://live.example/v1/speech_sessions", "token": "eyJ.live.token",
				"tokenExpiresAt": time.Now().Add(time.Minute).UTC().Format(time.RFC3339), "actualDurationSeconds": 600,
				"durationCapped": false, "billingMode": "reserve", "links": map[string]any{"chat": "https://atmee.example/chat/x"},
			}
			if asked, ok := b["maxDurationSeconds"].(float64); ok {
				resp["requestedDurationSeconds"] = asked
				if asked > 600 {
					resp["durationCapped"] = true
				} else {
					resp["actualDurationSeconds"] = asked
				}
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/speech_sessions/"+liveID:
			resp := map[string]any{
				"sessionId": liveID, "status": "completed", "actualDurationSeconds": 300, "billingMode": "reserve",
				"connectedAt": "2026-10-23T10:00:00Z", "endsAt": "2026-10-23T10:05:00Z", "endedAt": "2026-10-23T10:02:00Z", "endReason": "ended_by_api",
				"billedSeconds": 120, "chargedMicrocredits": 120,
			}
			if connected && ends > 0 {
				resp["status"], resp["endRequestedAt"], resp["endedAt"], resp["endReason"] = "active", "2026-10-23T10:01:30Z", nil, nil
				delete(resp, "billedSeconds")
				delete(resp, "chargedMicrocredits")
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/speech_sessions/"+liveID+"/end":
			if connected {
				_ = json.NewEncoder(w).Encode(map[string]any{"sessionId": liveID, "status": "active", "alreadyEnded": false,
					"endRequestedAt": "2026-10-23T10:01:30Z"})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"sessionId": liveID, "status": "completed", "alreadyEnded": ends > 0,
					"endedAt": "2026-10-23T10:02:00Z", "endReason": "ended_by_api"})
			}
			ends++
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_0123456789abcdef")
	return func() []seen {
		mu.Lock()
		defer mu.Unlock()
		return append([]seen(nil), calls...)
	}
}

func TestLiveStartSendsTheOptionsAndPrintsNoTokenWithoutJSON(t *testing.T) {
	isolate(t)
	calls := fakeLiveAPI(t, false)
	out, errOut, code := run(t, "", "speech", "start", "--avatar", avatarID, "--max-seconds", "300", "--language", "fr-FR")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	b := calls()[0].body
	if b["avatarId"] != avatarID || b["maxDurationSeconds"] != float64(300) || b["language"] != "fr-FR" || b["settings"] != nil {
		t.Fatalf("body: %v", b)
	}
	if strings.Contains(out+errOut, "eyJ.live.token") {
		t.Fatalf("the token was printed without --json: %q", out)
	}
	for _, want := range []string{liveID, "wss://live.example/v1/speech_sessions", "expires", "--json", "5m0s from now (as asked)"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestLiveStartSaysWhenTheLengthWasCapped(t *testing.T) {
	isolate(t)
	fakeLiveAPI(t, false)
	out := ok(t, "speech", "start", "--avatar", avatarID, "--max-seconds", "1800")
	if !strings.Contains(out, "10m0s from now (capped: 30m0s asked") {
		t.Fatalf("capped: %q", out)
	}
	// Nothing asked, nothing compared.
	if out := ok(t, "speech", "start", "--avatar", avatarID); !strings.Contains(out, "10m0s from now\n") {
		t.Fatalf("default length: %q", out)
	}
}

func TestLiveStartJSONCarriesTheToken(t *testing.T) {
	isolate(t)
	calls := fakeLiveAPI(t, false)
	out, errOut, code := run(t, "", "speech", "start", "--avatar", avatarID, "--json")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if m := jsonOf(t, out); m["token"] != "eyJ.live.token" || m["sessionId"] != liveID || m["tokenExpiresAt"] == nil ||
		m["actualDurationSeconds"] != float64(600) || m["durationCapped"] != false {
		t.Fatalf("json: %v", m)
	}
	if b := calls()[0].body; len(b) != 1 || b["avatarId"] != avatarID {
		t.Fatalf("only the avatar: %v", b)
	}
}

func TestLiveStartPinsSettingsFromFiles(t *testing.T) {
	dir := isolate(t)
	calls := fakeLiveAPI(t, false)
	settings := filepath.Join(dir, "pinned.json")
	// input items keep fields the CLI does not model; null pins nothing and is sent as given.
	if err := os.WriteFile(settings, []byte(`{"user_name": "Sam", "scenario": null,
		"input": [{"role": "developer", "type": "message", "content": [{"type": "input_text", "text": "Be brief."}]}],
		"delegation": {"type": "responses", "responses": {"tool_choice": "auto"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ok(t, "speech", "start", "--avatar", avatarID, "--settings", settings)
	s, _ := calls()[0].body["settings"].(map[string]any)
	item := s["input"].([]any)[0].(map[string]any)
	if s["user_name"] != "Sam" || item["type"] != "message" || item["content"].([]any)[0].(map[string]any)["type"] != "input_text" ||
		s["delegation"].(map[string]any)["type"] != "responses" {
		t.Fatalf("settings: %v", s)
	}
	if v, present := s["scenario"]; !present || v != nil {
		t.Fatalf("null scenario: %v", s)
	}

	// --instructions alone, from stdin, trimmed.
	if _, errOut, code := run(t, "  Speak like a pirate.\n", "speech", "start", "--avatar", avatarID, "--instructions", "-"); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	c := calls()
	if s := c[len(c)-1].body["settings"].(map[string]any); len(s) != 1 || s["instructions"] != "Speak like a pirate." {
		t.Fatalf("instructions: %v", s)
	}

	// Both together merge.
	instr := filepath.Join(dir, "instructions.txt")
	_ = os.WriteFile(instr, []byte("Keep it short."), 0o600)
	ok(t, "speech", "start", "--avatar", avatarID, "--settings", settings, "--instructions", instr)
	c = calls()
	if s := c[len(c)-1].body["settings"].(map[string]any); s["instructions"] != "Keep it short." || s["user_name"] != "Sam" {
		t.Fatalf("merged: %v", s)
	}
}

func TestLiveStartRefusesBadOptionsLocally(t *testing.T) {
	dir := isolate(t)
	calls := fakeLiveAPI(t, false)
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	typo := write("typo.json", `{"instruction": "x"}`)
	list := write("list.json", `["x"]`)
	pinned := write("pinned.json", `{"instructions": "x"}`)
	empty := write("empty.txt", "  \n")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "--avatar is required"},
		{[]string{"--max-seconds", "30"}, "60 to 3600"},
		{[]string{"--max-seconds", "4000"}, "60 to 3600"},
		{[]string{"--avatar", "not-a-uuid"}, "--avatar"},
		{[]string{"--settings", typo}, `"instruction" is not a setting`},
		{[]string{"--settings", list}, "JSON object"},
		{[]string{"--settings", filepath.Join(dir, "missing.json")}, "could not read"},
		{[]string{"--settings", pinned, "--instructions", empty}, "both pin the instructions"},
		{[]string{"--instructions", empty}, "is empty"},
		{[]string{"--settings", "-", "--instructions", "-"}, "both read stdin"},
	} {
		args := append([]string{"speech", "start"}, tc.args...)
		if tc.args != nil && tc.args[0] != "--avatar" {
			args = append(args, "--avatar", avatarID)
		}
		if _, errOut, code := run(t, "", args...); code != 2 || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: %d %q, want %q", tc.args, code, errOut, tc.want)
		}
	}
	if c := calls(); len(c) != 0 {
		t.Fatalf("sent %v", c)
	}
}

func TestLiveGetAndEnd(t *testing.T) {
	isolate(t)
	calls := fakeLiveAPI(t, false)
	out, errOut, code := run(t, "", "speech", "get", liveID)
	if code != 0 || !strings.Contains(out, "completed") || !strings.Contains(out, "ended_by_api") || !strings.Contains(out, "2m0s") ||
		!strings.Contains(out, "5m0s granted") || strings.Contains(out, "End requested") {
		t.Fatalf("get: %d %q %q", code, out, errOut)
	}
	if m := jsonOf(t, ok(t, "speech", "get", liveID, "--json")); m["billedSeconds"] != float64(120) {
		t.Fatalf("get --json: %v", m)
	}
	if out := ok(t, "speech", "end", liveID); !strings.Contains(out, "Ended speech session") || !strings.Contains(out, "completed") {
		t.Fatalf("end: %q", out)
	}
	if out := ok(t, "speech", "end", liveID); !strings.Contains(out, "had already ended") {
		t.Fatalf("end: %q", out)
	}
	if m := jsonOf(t, ok(t, "speech", "end", liveID, "--json")); m["alreadyEnded"] != true {
		t.Fatalf("end --json: %v", m)
	}
	c := calls()
	last := c[len(c)-1]
	if last.method != http.MethodPost || last.path != "/v1/speech_sessions/"+liveID+"/end" {
		t.Fatalf("end call: %v", last)
	}
	if _, errOut, code := run(t, "", "speech", "get"); code != 2 || !strings.Contains(errOut, "missing SESSION_ID") {
		t.Fatalf("missing id: %d %q", code, errOut)
	}
	if _, errOut, code := run(t, "", "speech", "end", "nope"); code == 0 || !strings.Contains(errOut, "not an avatar or session id") {
		t.Fatalf("bad id: %d %q", code, errOut)
	}
}

// With a connection open an end is requested: the session stays active until
// the connection's next usage report, billed until then.
func TestLiveEndWithAConnectionOpenIsRequested(t *testing.T) {
	isolate(t)
	fakeLiveAPI(t, true)
	out := ok(t, "speech", "end", liveID)
	if !strings.Contains(out, "Ending speech session "+liveID) || !strings.Contains(out, "next usage report") ||
		!strings.Contains(out, "billed until then") || strings.Contains(out, "Ended speech session") {
		t.Fatalf("end: %q", out)
	}
	if m := jsonOf(t, ok(t, "speech", "end", liveID, "--json")); m["status"] != "active" || m["endRequestedAt"] == nil {
		t.Fatalf("end --json: %v", m)
	}
	out = ok(t, "speech", "get", liveID)
	if !strings.Contains(out, "active") || !strings.Contains(out, "End requested") || !strings.Contains(out, "next usage report") ||
		strings.Contains(out, "Ended ") {
		t.Fatalf("get: %q", out)
	}
}
