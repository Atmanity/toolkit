package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func fakeSessionAPI(t *testing.T) { fakeSessionAPIWithLinks(t, nil) }

// fakeSessionAPIWithLinks answers the start with links (nil: none, as a
// server that predates them). It answers nothing else, so the command makes
// no other call.
func fakeSessionAPIWithLinks(t *testing.T, links map[string]any) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/session" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		body := map[string]any{
			"billingMode": "reserve", "roomName": "rk-1", "serverUrl": "wss://example.livekit.cloud",
			"sessionId": "11111111-2222-4333-8444-555555555555", "userToken": "tok.en/with+chars",
		}
		if links != nil {
			body["links"] = links
		}
		_ = json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_0123456789abcdef")
}

func recordBrowse(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	old := browse
	browse = func(u string) error { opened = append(opened, u); return nil }
	t.Cleanup(func() { browse = old })
	return &opened
}

func TestSessionChatOpenJoinsTheRoomInTheBrowser(t *testing.T) {
	isolate(t)
	fakeSessionAPI(t)
	opened := recordBrowse(t)

	out, errOut, code := run(t, "", "avatars", "session", "chat", "11111111-2222-4333-8444-555555555555", "--open")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if len(*opened) != 1 {
		t.Fatalf("opened %v", *opened)
	}
	u, err := url.Parse((*opened)[0])
	if err != nil || u.Host != "meet.livekit.io" || u.Path != "/custom" {
		t.Fatalf("link: %v %v", (*opened)[0], err)
	}
	if q := u.Query(); q.Get("liveKitUrl") != "wss://example.livekit.cloud" || q.Get("token") != "tok.en/with+chars" {
		t.Fatalf("link carries the server and token: %v", q)
	}
	if !strings.Contains(errOut, "join within 60 seconds") || !strings.Contains(errOut, (*opened)[0]) {
		t.Fatalf("the link and the deadline are printed: %q", errOut)
	}
	var m map[string]any
	if json.Unmarshal([]byte(out), &m) != nil || m["roomName"] != "rk-1" {
		t.Fatalf("stdout stays the API's JSON: %q", out)
	}
}

func TestSessionChatWithoutOpenOpensNothing(t *testing.T) {
	isolate(t)
	fakeSessionAPI(t)
	opened := recordBrowse(t)
	if _, errOut, code := run(t, "", "avatars", "session", "chat", "11111111-2222-4333-8444-555555555555"); code != 0 || len(*opened) != 0 {
		t.Fatalf("%d %q %v", code, errOut, *opened)
	}
}

func TestSessionChatWithoutOpenSaysHowToJoin(t *testing.T) {
	isolate(t)
	fakeSessionAPI(t)
	recordBrowse(t)
	_, errOut, code := run(t, "", "avatars", "session", "chat", "11111111-2222-4333-8444-555555555555")
	// The note carries this session's own link, not advice to start another.
	if code != 0 || !strings.Contains(errOut, "within 60 seconds") ||
		!strings.Contains(errOut, "https://meet.livekit.io/custom?") || !strings.Contains(errOut, "token=tok.en") {
		t.Fatalf("%d %q", code, errOut)
	}
}

func TestSessionChatPrintsTheWebsiteChatPage(t *testing.T) {
	isolate(t)
	page := "https://www.atmee.ai/avatars/11111111-2222-4333-8444-555555555555?lang=en"
	fakeSessionAPIWithLinks(t, map[string]any{"chat": page})
	recordBrowse(t)
	out, errOut, code := run(t, "", "avatars", "session", "chat", "11111111-2222-4333-8444-555555555555")
	if code != 0 || !strings.Contains(errOut, "meet.livekit.io") || !strings.Contains(errOut, page) {
		t.Fatalf("both links on stderr: %d %q", code, errOut)
	}
	var m map[string]any
	if json.Unmarshal([]byte(out), &m) != nil || m["links"].(map[string]any)["chat"] != page {
		t.Fatalf("stdout is the API's session JSON, links included: %q", out)
	}
}

func TestSessionChatWithoutLinksPrintsNoChatPage(t *testing.T) {
	isolate(t)
	fakeSessionAPI(t)
	recordBrowse(t)
	if _, errOut, code := run(t, "", "avatars", "session", "chat", "11111111-2222-4333-8444-555555555555"); code != 0 || strings.Contains(errOut, "chat page") {
		t.Fatalf("%d %q", code, errOut)
	}
}
