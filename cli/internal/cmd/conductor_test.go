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
	"testing"
)

const conductorAvatar = "11111111-2222-4333-8444-555555555555"

const testGraph = `{"start": "greet", "nodes": [
  {"id": "greet", "instructions": "Greet them.", "edges": [{"to": "help", "when": "They said why."}, {"to": "greet", "weight": 1}]},
  {"id": "help", "instructions": "Help.", "edges": [{"to": "bye", "speak": true}]},
  {"id": "bye", "instructions": "Say goodbye.", "emotion": null}
]}`

type seen struct {
	method, path string
	body         map[string]any
}

// conductorServer answers like the API: PUT echoes the graph back as the
// Conductor, GET returns it, DELETE 204; status/body override every answer.
func conductorServer(t *testing.T, status int, errBody string) *[]seen {
	t.Helper()
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	var reqs []seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		s := seen{method: r.Method, path: r.URL.Path}
		_ = json.Unmarshal(raw, &s.body)
		reqs = append(reqs, s)
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(errBody))
			return
		}
		switch r.Method {
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		case http.MethodPut:
			out := map[string]any{"avatarId": conductorAvatar, "graph": s.body["graph"], "nodes": 3, "edges": 3}
			if v, ok := s.body["speakAfterS"]; ok {
				out["speakAfterS"] = v
			}
			_ = json.NewEncoder(w).Encode(out)
		default:
			_, _ = w.Write([]byte(`{"avatarId":"` + conductorAvatar + `","graph":` + testGraph + `,"speakAfterS":1.5,"nodes":3,"edges":3}`))
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	return &reqs
}

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func graphOf(t *testing.T) map[string]any {
	t.Helper()
	var g map[string]any
	if err := json.Unmarshal([]byte(testGraph), &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestConductorSetSendsABareGraphAsTheGraph(t *testing.T) {
	reqs := conductorServer(t, 0, "")
	file := writeFile(t, "agenda.json", testGraph)
	out, errOut, code := run(t, "", "avatars", "conductor", "set", conductorAvatar, "--graph", file)
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	r := (*reqs)[0]
	if r.method != http.MethodPut || r.path != "/v1/avatars/"+conductorAvatar+"/conductor" {
		t.Fatalf("sent %s %s", r.method, r.path)
	}
	// The graph goes as written (null fields included); no speakAfterS unless asked.
	if want := map[string]any{"graph": graphOf(t)}; !reflect.DeepEqual(r.body, want) {
		t.Fatalf("body %v", r.body)
	}
	for _, want := range []string{"3 nodes, 3 edges, start node greet, speak after Causeur's default",
		"greet -> help (when), greet", "help -> bye (speak)", "bye end", "next session"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestConductorSetTakesAWholeBodyFromStdinAndSpeakAfterWins(t *testing.T) {
	reqs := conductorServer(t, 0, "")
	stdin := `{"graph": ` + testGraph + `, "speakAfterS": 2}`
	if _, errOut, code := run(t, stdin, "avatars", "conductor", "set", conductorAvatar, "--graph", "-"); code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if want := map[string]any{"graph": graphOf(t), "speakAfterS": 2.0}; !reflect.DeepEqual((*reqs)[0].body, want) {
		t.Fatalf("whole body: %v", (*reqs)[0].body)
	}
	out, errOut, code := run(t, stdin, "avatars", "conductor", "set", conductorAvatar, "--graph", "-", "--speak-after", "0.5", "--json")
	if code != 0 {
		t.Fatalf("%d %s", code, errOut)
	}
	if got := (*reqs)[1].body["speakAfterS"]; got != 0.5 {
		t.Fatalf("--speak-after replaces the body's: %v", got)
	}
	if m := jsonOf(t, out); m["avatarId"] != conductorAvatar || m["speakAfterS"] != 0.5 {
		t.Fatalf("--json: %v", m)
	}
}

func TestConductorSetUsageErrorsSendNothing(t *testing.T) {
	reqs := conductorServer(t, 0, "")
	file := writeFile(t, "agenda.json", testGraph)
	for _, tc := range []struct {
		stdin string
		args  []string
		want  string
	}{
		{"", []string{"set", conductorAvatar}, "--graph is required"},
		{"", []string{"set"}, "missing AVATAR_ID (find it with `atmee avatars list`)"},
		{"", []string{"set", conductorAvatar, "--graph", file, "--speak-after", "0.05"}, "--speak-after takes 0.1 to 10"},
		{"", []string{"set", conductorAvatar, "--graph", file, "--speak-after", "11"}, "--speak-after takes 0.1 to 10"},
		{"", []string{"set", conductorAvatar, "--graph", file, "--speak-after", "NaN"}, "--speak-after takes 0.1 to 10"},
		{"[1]", []string{"set", conductorAvatar, "--graph", "-"}, "must hold a JSON object"},
		{"{nope", []string{"set", conductorAvatar, "--graph", "-"}, "must hold a JSON object"},
		{"", []string{"set", conductorAvatar, "--graph", filepath.Join(t.TempDir(), "missing.json")}, "could not read"},
		{`{"start":"a","nodes":[],"reasoning":"` + strings.Repeat("z", 513*1024) + `"}`, []string{"set", conductorAvatar, "--graph", "-"}, "512 KiB"},
		{"", []string{"get"}, "missing AVATAR_ID"},
		{"", []string{"delete"}, "missing AVATAR_ID"},
	} {
		_, errOut, code := run(t, tc.stdin, append([]string{"avatars", "conductor"}, tc.args...)...)
		if code != 2 || !strings.Contains(errOut, tc.want) {
			t.Errorf("%v: %d %q", tc.args, code, errOut)
		}
	}
	if len(*reqs) != 0 {
		t.Fatalf("sent %d requests", len(*reqs))
	}
}

func TestConductorRefusalsSayWhatToDo(t *testing.T) {
	file := writeFile(t, "agenda.json", testGraph)
	for _, tc := range []struct {
		name, body string
		status     int
		args       []string
		want       []string
	}{
		{"v1", `{"error":"not_v2_avatar","message":"the graph conductor runs only on v2 avatars (engine v2); this avatar is v1"}`, 409,
			[]string{"set", conductorAvatar, "--graph", file}, []string{conductorAvatar + " is a v1 avatar", "only on v2 avatars", "not_v2_avatar"}},
		{"field path", `{"error":"invalid_conductor","message":"/graph/nodes/0/edges/1/weight: must be above 0"}`, 400,
			[]string{"set", conductorAvatar, "--graph", file}, []string{"refused the graph at /graph/nodes/0/edges/1/weight (/nodes/0/edges/1/weight in " + file + "): must be above 0 (invalid_conductor)"}},
		{"whole graph", `{"error":"invalid_conductor","message":"/graph: over 150000 characters"}`, 400,
			[]string{"set", conductorAvatar, "--graph", file}, []string{"at /graph (the whole graph in " + file + "): over 150000"}},
		{"422", `{"code":422,"message":"request body has an error: doesn't match schema: property \"mode\" is unsupported\nSchema: {}"}`, 422,
			[]string{"set", conductorAvatar, "--graph", file}, []string{"refused the request (422)", `"mode" is unsupported`}},
		{"not set", `{"error":"conductor_not_set","message":"no graph conductor is set on this avatar"}`, 404,
			[]string{"get", conductorAvatar}, []string{"no conductor is set on " + conductorAvatar, "atmee avatars conductor set " + conductorAvatar + " --graph FILE"}},
		{"no avatar", `{"error":"avatar_not_found","message":"avatar not found"}`, 404,
			[]string{"delete", conductorAvatar, "--yes"}, []string{"avatar " + conductorAvatar + " not found", "atmee avatars list"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conductorServer(t, tc.status, tc.body)
			_, errOut, code := run(t, "", append([]string{"avatars", "conductor"}, tc.args...)...)
			if code != 1 {
				t.Fatalf("code %d %s", code, errOut)
			}
			for _, w := range tc.want {
				if !strings.Contains(errOut, w) {
					t.Errorf("lacks %q: %s", w, errOut)
				}
			}
		})
	}
	// A whole-body file's path is the API's own: no second path.
	conductorServer(t, 400, `{"error":"invalid_conductor","message":"/graph/nodes/0/id: taken"}`)
	body := writeFile(t, "body.json", `{"graph": `+testGraph+`}`)
	if _, errOut, _ := run(t, "", "avatars", "conductor", "set", conductorAvatar, "--graph", body); strings.Contains(errOut, " in "+body) || !strings.Contains(errOut, "at /graph/nodes/0/id: taken") {
		t.Fatalf("whole body: %s", errOut)
	}
}

func TestConductorGetSavesABodySetTakesBack(t *testing.T) {
	reqs := conductorServer(t, 0, "")
	file := filepath.Join(t.TempDir(), "saved.json")
	out, errOut, code := run(t, "", "avatars", "conductor", "get", conductorAvatar, "--out", file)
	if code != 0 || !strings.Contains(out, "3 nodes, 3 edges, start node greet, speak after 1.5 s") || !strings.Contains(errOut, "Saved to "+file) {
		t.Fatalf("%d\n%s\n%s", code, out, errOut)
	}
	raw, _ := os.ReadFile(file)
	var saved map[string]any
	if err := json.Unmarshal(raw, &saved); err != nil || !reflect.DeepEqual(saved, map[string]any{"graph": graphOf(t), "speakAfterS": 1.5}) {
		t.Fatalf("saved %s", raw)
	}
	if _, errOut, code := run(t, "", "avatars", "conductor", "set", conductorAvatar, "--graph", file); code != 0 {
		t.Fatalf("set from the saved file: %d %s", code, errOut)
	}
	if got := (*reqs)[1].body; !reflect.DeepEqual(got, saved) {
		t.Fatalf("round trip sent %v", got)
	}
	out, _, _ = run(t, "", "avatars", "conductor", "get", conductorAvatar, "--json")
	if m := jsonOf(t, out); m["nodes"] != 3.0 || m["graph"].(map[string]any)["nodes"].([]any)[2].(map[string]any)["emotion"] != nil {
		t.Fatalf("--json: %v", m)
	}
	if !strings.Contains(out, `"emotion": null`) {
		t.Fatalf("--json keeps the graph's null fields: %s", out)
	}
}

func TestConductorDeleteAsksFirst(t *testing.T) {
	reqs := conductorServer(t, 0, "")
	if _, errOut, code := run(t, "n\n", "avatars", "conductor", "delete", conductorAvatar); code != 2 || !strings.Contains(errOut, "aborted") || len(*reqs) != 0 {
		t.Fatalf("no: %d %s %d", code, errOut, len(*reqs))
	}
	out, errOut, code := run(t, "y\n", "avatars", "conductor", "delete", conductorAvatar)
	if code != 0 || !strings.Contains(out, "Conductor removed") || (*reqs)[0].method != http.MethodDelete {
		t.Fatalf("yes: %d %s %s", code, out, errOut)
	}
	if _, errOut, code := run(t, "", "avatars", "conductor", "delete", conductorAvatar, "--yes"); code != 0 || len(*reqs) != 2 {
		t.Fatalf("--yes: %d %s", code, errOut)
	}
}

func TestAvatarGetShowsTheConductor(t *testing.T) {
	base := `{"id":"` + conductorAvatar + `","name":"M","kind":"conversational","status":"ready","readyToChat":true,"visibility":"private","createdAt":"2026-10-16T00:00:00Z","engine":"v2"`
	for conductor, want := range map[string]string{
		`,"conductor":{"configured":true,"nodes":4}}`: "Conductor: 4 nodes (atmee avatars conductor get " + conductorAvatar + ")",
		`,"conductor":{"configured":false}}`:          "Conductor: none",
	} {
		avatarServer(t, base+conductor)
		if out, errOut, code := run(t, "", "avatars", "get", conductorAvatar); code != 0 || !strings.Contains(out, want) {
			t.Errorf("%d %q %s", code, out, errOut)
		}
	}
	avatarServer(t, base+`}`)
	if out, _, _ := run(t, "", "avatars", "get", conductorAvatar); strings.Contains(out, "Conductor") {
		t.Errorf("no conductor field, no line: %s", out)
	}
}
