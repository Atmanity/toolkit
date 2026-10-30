// Package web runs the short-lived localhost candidate picker the CLI opens
// in a browser.
package web

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed pages/*.html
var pages embed.FS

// Open shows url in the user's browser and always prints it too: on a
// headless machine the opener can start and exit without showing anything.
// ATMEE_NO_BROWSER only prints. Tests replace it.
var Open = func(url string) {
	fmt.Fprintf(os.Stderr, "Open %s in your browser if it did not open.\n", url)
	_ = Browse(url)
}

// Browse starts the default browser on url; with ATMEE_NO_BROWSER set it
// does nothing.
func Browse(url string) error {
	if os.Getenv("ATMEE_NO_BROWSER") != "" {
		return nil
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // reap the opener
	return nil
}

// jsString makes v safe inside a single-quoted JavaScript string literal.
func jsString(v any) string {
	b, _ := json.Marshal(v)
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "</", `<\/`, " ", ` `, " ", ` `)
	return r.Replace(string(b))
}

func page(name string, repl ...string) []byte {
	b, err := pages.ReadFile("pages/" + name)
	if err != nil {
		panic(err) // embedded at build time
	}
	return []byte(strings.NewReplacer(repl...).Replace(string(b)))
}

// server is a localhost server that stops after its first decisive POST or
// the timeout.
type server struct {
	secret string
	ln     net.Listener
	srv    *http.Server
	done   chan struct{}
	once   sync.Once
}

func start(h func(s *server) http.Handler) (*server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &server{secret: randomSecret(), ln: ln, done: make(chan struct{})}
	s.srv = &http.Server{Handler: h(s), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// trusted reports that r came from a page this server rendered: it carries
// the server's secret, a JSON body, and this server's origin. The secret is
// embedded in the page this server serves, so it keeps out other web pages
// (which cannot read a cross-origin response, and whose Origin is refused)
// but not other local processes, which can fetch the page; a local process
// can already act as the user. The custom header also forces a CORS
// preflight for cross-origin browser posts, which this server never approves.
func (s *server) trusted(r *http.Request) bool {
	return r.Header.Get("X-Atmee-Secret") == s.secret &&
		strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") &&
		(r.Header.Get("Origin") == "" || r.Header.Get("Origin") == s.url())
}

func (s *server) url() string {
	return fmt.Sprintf("http://localhost:%d", s.ln.Addr().(*net.TCPAddr).Port)
}

func (s *server) finish() { s.once.Do(func() { close(s.done) }) }

// wait blocks until finish or timeout, then shuts the server down.
func (s *server) wait(timeout time.Duration) bool {
	t := time.NewTimer(timeout)
	defer t.Stop()
	ok := true
	select {
	case <-s.done:
	case <-t.C:
		ok = false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
	return ok
}

func readJSON(r *http.Request) map[string]any {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	out := map[string]any{}
	_ = json.Unmarshal(body, &out)
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// Pick serves a picker page with data and returns the posted choice, or nil
// when nothing was chosen before the timeout.
func Pick(pageName string, data any, timeout time.Duration) (map[string]any, error) {
	var choice map[string]any
	s, err := start(func(s *server) http.Handler {
		html := page(pageName, "__DATA__", jsString(data), "__SECRET__", s.secret)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				if !s.trusted(r) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				choice = readJSON(r)
				writeJSON(w, map[string]bool{"ok": true})
				s.finish()
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(html)
		})
	})
	if err != nil {
		return nil, err
	}
	Open(s.url())
	if !s.wait(timeout) {
		return nil, nil
	}
	return choice, nil
}

func randomSecret() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
