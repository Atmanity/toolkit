package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

const profileID = "11111111-2222-4333-8444-555555555555"

const detailJSON = `{"id":"` + profileID + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private",` +
	`"createdAt":"2026-10-16T00:00:00Z","updatedAt":"2026-10-16T00:00:00Z","stages":{}}`

// recorded is one request the stub saw.
type recorded struct {
	Method, Path, ContentType string
	Body                      map[string]any
	Raw                       []byte
}

// profileAPI stubs the Avatar API: GET /knowledge answers knowledge, PATCH
// answers patch(n) for the n-th PATCH (nil: the detail), the rest the detail.
func profileAPI(t *testing.T, knowledge string, patch func(n int) (int, string)) *[]recorded {
	t.Helper()
	var mu sync.Mutex
	var got []recorded
	patches := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/account" { // the create's v2 access lookup
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		rec := recorded{Method: r.Method, Path: r.URL.Path, ContentType: r.Header.Get("Content-Type"), Raw: raw}
		_ = json.Unmarshal(raw, &rec.Body)
		mu.Lock()
		got = append(got, rec)
		mu.Unlock()
		switch {
		case r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"avatarId":"` + profileID + `","kind":"conversational","status":"building","statusUrl":"/v1/avatars/` + profileID + `"}`))
		case r.URL.Path == "/v1/avatars/"+profileID+"/knowledge":
			_, _ = w.Write([]byte(knowledge))
		case r.Method == http.MethodPatch && patch != nil:
			patches++
			status, body := patch(patches)
			if status != 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(body))
				return
			}
			_, _ = w.Write([]byte(detailJSON))
		default:
			_, _ = w.Write([]byte(detailJSON))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	return &got
}

func TestWhoAndWhatPrompts(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	got := profileAPI(t, "", nil)
	persona := func(i int) map[string]any { p, _ := (*got)[i].Body["persona"].(map[string]any); return p }
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "M", "--who-prompt", "A gardener", "--what-prompt", "Plan a bed", "--json"); code != 0 {
		t.Fatalf("create: %d %s", code, errOut)
	}
	if p := persona(0); p["whoPrompt"] != "A gardener" || p["whatPrompt"] != "Plan a bed" || p["personality"] != nil {
		t.Fatalf("create persona: %v", p)
	}
	// The older names still work, and send the current ones.
	if _, errOut, code := run(t, "", "avatars", "create", "--name", "M", "--personality", "A coach", "--what", "Teach", "--json"); code != 0 {
		t.Fatalf("create aliases: %d %s", code, errOut)
	}
	if p := persona(1); p["whoPrompt"] != "A coach" || p["whatPrompt"] != "Teach" || len(p) != 2 {
		t.Fatalf("create alias persona: %v", p)
	}
	if _, errOut, code := run(t, `{"who_prompt":"A baker","scenario":"Bake"}`, "avatars", "create", "--name", "M", "--config", "-", "--json"); code != 0 {
		t.Fatalf("config: %d %s", code, errOut)
	}
	if p := persona(2); p["whoPrompt"] != "A baker" || p["whatPrompt"] != "Bake" {
		t.Fatalf("config persona: %v", p)
	}
	if _, errOut, code := run(t, `{"who_prompt":"A","personality":"B"}`, "avatars", "create", "--name", "M", "--config", "-"); code != 2 {
		t.Fatalf("config with both names: %d %s", code, errOut)
	}
	if _, errOut, code := run(t, "", "avatars", "update", profileID, "--who-prompt", "Who", "--scenario", "What", "--json"); code != 0 {
		t.Fatalf("update: %d %s", code, errOut)
	}
	if b := (*got)[3].Body; b["whoPrompt"] != "Who" || b["whatPrompt"] != "What" || len(b) != 2 {
		t.Fatalf("update body: %v", b)
	}
	if len(*got) != 4 {
		t.Fatalf("requests: %v", *got)
	}
	out, _, _ := run(t, "", "avatars", "update", "--help")
	if !strings.Contains(out, "--who-prompt") || strings.Contains(out, "--personality") {
		t.Fatalf("help shows the current names only:\n%s", out)
	}
}

func TestUpdateShareFields(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	got := profileAPI(t, "", nil)
	if _, errOut, code := run(t, "", "avatars", "update", profileID, "--title", "Ada", "--subtitle", "Chess", "--tag", " chess ", "--tag", "Coach", "--tag", "CHESS",
		"--social", "YouTube=https://youtube.com/@ada", "--social", "instagram=https://instagram.com/ada", "--statement", "fictional", "--monetize", "off", "--json"); code != 0 {
		t.Fatalf("update: %d %s", code, errOut)
	}
	want := map[string]any{
		"title": "Ada", "subtitle": "Chess", "tags": []any{"chess", "Coach"},
		"socialLinks":   []any{map[string]any{"type": "youtube", "url": "https://youtube.com/@ada"}, map[string]any{"type": "instagram", "url": "https://instagram.com/ada"}},
		"statementType": "fictional_verification", "monetization": map[string]any{"enabled": false},
	}
	if b := (*got)[0].Body; !reflect.DeepEqual(b, want) {
		t.Fatalf("body:\n got %v\nwant %v", b, want)
	}
	if _, errOut, code := run(t, "", "avatars", "update", profileID, "--clear", "title", "--clear", "subtitle", "--clear", "tags", "--clear", "socials", "--clear", "statement", "--json"); code != 0 {
		t.Fatalf("clear: %d %s", code, errOut)
	}
	want = map[string]any{"title": "", "subtitle": "", "tags": []any{}, "socialLinks": []any{}, "statementType": ""}
	if b := (*got)[1].Body; !reflect.DeepEqual(b, want) {
		t.Fatalf("clear body: %v", b)
	}
	if _, errOut, code := run(t, "", "avatars", "update", profileID, "--monetize", "on", "--confirm-sponsor-withdrawal", "--json"); code != 0 {
		t.Fatalf("monetize: %d %s", code, errOut)
	}
	if b := (*got)[2].Body; !reflect.DeepEqual(b["monetization"], map[string]any{"enabled": true, "confirmSponsorWithdrawal": true}) {
		t.Fatalf("monetize body: %v", b)
	}
	for _, bad := range [][]string{
		{"--tag", "a", "--tag", "b", "--tag", "c", "--tag", "d", "--tag", "e", "--tag", "f"},
		{"--tag", "!!"},
		{"--social", "myspace=https://myspace.com/x"},
		{"--social", "youtube=http://youtube.com/x"},
		{"--social", "youtube"},
		{"--social", "youtube=https://a", "--social", "youtube=https://b"},
		{"--social", "youtube=https://" + strings.Repeat("a", 500)},
		{"--statement", "robot"},
		{"--monetize", "maybe"},
		{"--monetize", "off", "--confirm-sponsor-withdrawal"},
		{"--confirm-sponsor-withdrawal"},
		{"--title", " "},
		{"--title", "Ada", "--clear", "title"},
		{"--tag", "chess", "--clear", "tags"},
		{"--statement", "self", "--clear", "statement"},
	} {
		if _, errOut, code := run(t, "", append([]string{"avatars", "update", profileID}, bad...)...); code != 2 {
			t.Errorf("%v: %d %s", bad, code, errOut)
		}
	}
	if len(*got) != 3 {
		t.Fatalf("bad input was sent: %v", (*got)[3:])
	}
}

func TestUpdateAnswerMergesTheQuestionnaire(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	got := profileAPI(t, `{"questions":["Q1?","Q2?","Q3?"],"questionnaire":{"Q1?":"A1","Q2?":"A2"},"files":[],"integrationIds":[]}`, nil)
	if _, errOut, code := run(t, "", "avatars", "update", profileID, "--answer", "3=New", "--answer", "Q2?=", "--answer", "Q1?=Changed = yes", "--facts", "Basil.", "--json"); code != 0 {
		t.Fatalf("update: %d %s", code, errOut)
	}
	if len(*got) != 2 || (*got)[0].Method != http.MethodGet || (*got)[1].Method != http.MethodPatch {
		t.Fatalf("reads the current answers first: %v", *got)
	}
	want := map[string]any{"facts": "Basil.", "questionnaire": map[string]any{"Q1?": "Changed = yes", "Q3?": "New"}}
	if b := (*got)[1].Body; !reflect.DeepEqual(b["knowledge"], want) || len(b) != 1 {
		t.Fatalf("body: %v", b)
	}
	if _, errOut, code := run(t, "", "avatars", "update", profileID, "--clear", "questionnaire", "--json"); code != 0 {
		t.Fatalf("clear: %d %s", code, errOut)
	}
	if b := (*got)[2]; b.Method != http.MethodPatch || !reflect.DeepEqual(b.Body["knowledge"], map[string]any{"questionnaire": map[string]any{}}) {
		t.Fatalf("clear body: %v", b)
	}
	for _, bad := range [][]string{{"--answer", "Q9?=x"}, {"--answer", "4=x"}, {"--answer", "no equals"}, {"--answer", "1=" + strings.Repeat("x", 2001)}} {
		if _, errOut, code := run(t, "", append([]string{"avatars", "update", profileID}, bad...)...); code != 2 {
			t.Errorf("%v: %d %s", bad, code, errOut)
		}
	}
	if _, _, code := run(t, "", "avatars", "update", profileID, "--answer", "1=x", "--clear", "questionnaire"); code != 2 {
		t.Errorf("--answer with --clear questionnaire: %d", code)
	}
	for _, r := range (*got)[3:] {
		if r.Method == http.MethodPatch {
			t.Fatalf("bad --answer was sent: %v", r)
		}
	}
	out, errOut, code := run(t, "", "avatars", "questionnaire", profileID)
	if code != 0 || !strings.Contains(out, "1. Q1?\n   A1\n2. Q2?\n   A2\n3. Q3?\n   (not answered)") {
		t.Fatalf("questionnaire: %d %s\n%s", code, errOut, out)
	}
}

func TestMonetizeAsksBeforeWithdrawingTheSponsorPool(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	var got *[]recorded
	got = profileAPI(t, "", func(int) (int, string) {
		m, _ := (*got)[len(*got)-1].Body["monetization"].(map[string]any)
		if m["confirmSponsorWithdrawal"] != true {
			return 409, `{"error":"sponsor_pool_not_empty","message":"the sponsor pool holds credits","sponsorPoolMicrocredits":12345}`
		}
		return 0, ""
	})
	_, errOut, code := run(t, "n\n", "avatars", "update", profileID, "--monetize", "on")
	if code == 0 || !strings.Contains(errOut, "123.45 credits") || !strings.Contains(errOut, "--confirm-sponsor-withdrawal") || len(*got) != 1 {
		t.Fatalf("declined: %d %s %v", code, errOut, *got)
	}
	if _, errOut, code := run(t, "y\n", "avatars", "update", profileID, "--monetize", "on", "--json"); code != 0 {
		t.Fatalf("accepted: %d %s", code, errOut)
	}
	if len(*got) != 3 || !reflect.DeepEqual((*got)[2].Body["monetization"], map[string]any{"enabled": true, "confirmSponsorWithdrawal": true}) {
		t.Fatalf("resent with the confirmation: %v", *got)
	}
}

func TestProfileRefusalsSayWhatToDo(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	var answer string
	profileAPI(t, "", func(int) (int, string) { return 409, answer })
	for body, wants := range map[string][]string{
		`{"error":"public_requirements_missing","message":"public needs more","missing":["title","tags","thumbnail"]}`: {
			"409 public_requirements_missing", "give them with:", "--title ... --tag ...", "atmee avatars thumbnail set " + profileID},
		`{"error":"payouts_not_ready","message":"no payouts","payoutsStatus":"pending"}`:       {"Creator Studio", "payouts: pending", "atmee account"},
		`{"error":"monetization_limit_reached","message":"cap","cap":2,"monetizingAvatars":2}`: {"(2 of 2)"},
		`{"error":"monetization_eligibility_not_met","message":"not yet"}`:                     {"atmee avatars monetization " + profileID},
		`{"error":"statement_required","message":"x"}`:                                         {"--statement fictional"},
		`{"error":"identity_not_verified","message":"x"}`:                                      {"identity check"},
		`{"error":"avatar_is_earning","message":"x"}`:                                          {"--monetize off"},
		`{"error":"moderation_blocked","message":"x","moderationState":"held"}`:                {"moderation (held)"},
	} {
		answer = body
		_, errOut, code := run(t, "", "avatars", "update", profileID, "--visibility", "public")
		if code != 1 {
			t.Errorf("%s: code %d", body, code)
		}
		for _, w := range wants {
			if !strings.Contains(errOut, w) {
				t.Errorf("%s: no %q in %s", body, w, errOut)
			}
		}
	}
}

func TestThumbnail(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	got := profileAPI(t, "", nil)
	img := filepath.Join(t.TempDir(), "thumb.png")
	if err := os.WriteFile(img, []byte("\x89PNG fake"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := run(t, "", "avatars", "thumbnail", "set", profileID, img)
	if code != 0 || !strings.Contains(out, "Thumbnail set.") {
		t.Fatalf("set: %d %s %s", code, out, errOut)
	}
	r := (*got)[0]
	if r.Method != http.MethodPut || r.Path != "/v1/avatars/"+profileID+"/thumbnail" || !strings.HasPrefix(r.ContentType, "multipart/form-data") ||
		!strings.Contains(string(r.Raw), `name="file"; filename="thumb.png"`) {
		t.Fatalf("set request: %s %s %s", r.Method, r.Path, r.ContentType)
	}
	if _, errOut, code := run(t, "n\n", "avatars", "thumbnail", "remove", profileID); code != 2 || !strings.Contains(errOut, "aborted") || len(*got) != 1 {
		t.Fatalf("remove asks first: %d %s", code, errOut)
	}
	if out, errOut, code := run(t, "", "avatars", "thumbnail", "remove", profileID, "--yes"); code != 0 || !strings.Contains(out, "Thumbnail removed.") {
		t.Fatalf("remove: %d %s", code, errOut)
	}
	if r := (*got)[1]; r.Method != http.MethodDelete || r.Path != "/v1/avatars/"+profileID+"/thumbnail" {
		t.Fatalf("remove request: %s %s", r.Method, r.Path)
	}
	if _, _, code := run(t, "", "avatars", "thumbnail", "set", profileID, filepath.Join(t.TempDir(), "missing.png")); code == 0 || len(*got) != 2 {
		t.Fatalf("a missing file was sent: %d", code)
	}
}

func TestMonetizationShowsEligibilityAndEarnings(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/avatars/"+profileID+"/monetization" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"monetized":false,"eligible":false,"last30Days":{"conversations":12,"minutes":40.5,"uniqueUsers":3},` +
			`"requirements":{"conversations":10,"minutes":60,"uniqueUsers":5,"meetsConversations":true,"meetsMinutes":false,"meetsUniqueUsers":false},` +
			`"accountCap":{"monetizingAvatars":1,"cap":3,"atCap":false},` +
			`"earnings":{"allTimeMicrocredits":123456,"currentCycleMicrocredits":250,"allTimeSessions":40,"currentCycleSessions":1}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	out, errOut, code := run(t, "", "avatars", "monetization", profileID)
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	for _, want := range []string{"Monetized    no", "Eligible     no", "✓ 12 of 10 conversations · ✗ 40.50 of 60 minutes · ✗ 3 of 5 people",
		"1 monetized · plan cap 3", "2.50 credits this cycle (1 session) · 1,234.56 credits all time (40 sessions)"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in:\n%s", want, out)
		}
	}
	out, _, _ = run(t, "", "avatars", "monetization", profileID, "--json")
	if !strings.Contains(out, `"allTimeMicrocredits": 123456`) {
		t.Fatalf("--json: %s", out)
	}
}
