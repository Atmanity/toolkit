package web

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// browse replaces the browser: it fetches the page and then posts like it.
func browse(t *testing.T, post func(base string)) {
	t.Helper()
	prev := Open
	Open = func(url string) {
		go func() {
			resp, err := http.Get(url)
			if err != nil {
				t.Error(err)
				return
			}
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if !strings.Contains(string(b), "<html") {
				t.Errorf("page: %.80s", b)
			}
			post(strings.TrimSuffix(url, "/"))
		}()
	}
	t.Cleanup(func() { Open = prev })
}

func TestJSStringEscapesForASingleQuotedLiteral(t *testing.T) {
	got := jsString(map[string]string{"label": `O'Neil </script> \ x`})
	for i := 0; i < len(got); i++ {
		if got[i] == '\'' && (i == 0 || got[i-1] != '\\') {
			t.Fatalf("unescaped quote at %d: %s", i, got)
		}
	}
	if strings.Contains(got, "</script>") {
		t.Fatalf("closing tag survived: %s", got)
	}
}

// pageSecret reads the secret a served page sends with its posts.
func pageSecret(t *testing.T, url string) string {
	resp, err := http.Get(url)
	if err != nil {
		t.Error(err)
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	m := regexp.MustCompile(`'X-Atmee-Secret': ?'([0-9a-f]+)'`).FindStringSubmatch(string(b))
	if m == nil {
		t.Error("page carries no secret")
		return ""
	}
	return m[1]
}

func TestPickRefusesPostsWithoutTheSecret(t *testing.T) {
	codes := make(chan int, 1)
	browse(t, func(base string) {
		codes <- postWith(base, "", "", `{"more":true}`)
		postWith(base, "", pageSecret(t, base), `{"selected":"c-1"}`)
	})
	got, err := Pick("candidate_picker.html", map[string]any{"items": []any{}}, 5*time.Second)
	if err != nil || got["selected"] != "c-1" || <-codes != 403 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestPickReturnsThePostedChoice(t *testing.T) {
	browse(t, func(base string) {
		postWith(base, "", pageSecret(t, base), `{"selected":"c-2"}`)
	})
	got, err := Pick("candidate_picker.html", map[string]any{"items": []any{}}, 5*time.Second)
	if err != nil || got["selected"] != "c-2" {
		t.Fatalf("%v %v", got, err)
	}
}

func TestPickTimesOutWithNoChoice(t *testing.T) {
	prev := Open
	Open = func(string) {}
	t.Cleanup(func() { Open = prev })
	got, err := Pick("candidate_picker.html", map[string]any{"items": []any{}}, 50*time.Millisecond)
	if err != nil || got != nil {
		t.Fatalf("%v %v", got, err)
	}
}

func postWith(base, path, secret, body string) int {
	req, _ := http.NewRequest(http.MethodPost, base+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if secret != "" {
		req.Header.Set("X-Atmee-Secret", secret)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}
