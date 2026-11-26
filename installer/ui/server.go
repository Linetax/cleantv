package ui

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web
var web embed.FS

// event is one thing the window shows: a line, a new step, a question, the end.
type event struct {
	N      int      `json:"n"`
	T      string   `json:"t"`              // line, step, prompt, done, error
	Text   string   `json:"text,omitempty"` // the line, title, question or error
	Kind   string   `json:"kind,omitempty"` // prompt: pause, text, yesno, choice
	Def    string   `json:"def,omitempty"`
	Opts   []string `json:"opts,omitempty"`
	Button string   `json:"button,omitempty"`
}

type server struct {
	app   App
	token string
	addr  string

	mu       sync.Mutex
	events   []event
	wake     chan struct{} // closed and replaced on every new event
	pending  int           // N of the question waiting for an answer, 0 when none
	answer   chan string
	lastSeen time.Time // last request from the window
	seen     bool
	byeAt    time.Time
	code     int
}

// A window that stops asking for news for this long is gone. A request waits up to
// hold for news, so an open window always comes back well within it.
const (
	hold    = 20 * time.Second
	gone    = 45 * time.Second
	closing = 3 * time.Second // after the page says goodbye, unless it reloads
)

func newServer(app App) *server {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return &server{app: app, token: hex.EncodeToString(b), wake: make(chan struct{})}
}

func (s *server) start() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("Impossible d'ouvrir la fenêtre de l'installateur : " + err.Error())
	}
	s.addr = ln.Addr().String()
	go http.Serve(ln, s.routes())
	go s.watch()
	url := "http://" + s.addr + "/?t=" + s.token
	fmt.Println("Interface : " + url)
	if os.Getenv("CLEANTV_NO_BROWSER") == "" && !openWindow(url, filepath.Join(appDir(), "fenetre")) {
		go alert("cleantv", "Ouvrez cette adresse dans votre navigateur pour continuer :\n\n"+url)
	}
}

func appDir() string {
	d, err := os.UserCacheDir()
	if err != nil {
		d = os.TempDir()
	}
	return filepath.Join(d, "cleantv")
}

func (s *server) routes() http.Handler {
	files, _ := fs.Sub(web, "web")
	static := http.FileServer(http.FS(files))
	mux := http.NewServeMux()
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/answer", s.handleAnswer)
	mux.HandleFunc("/api/bye", s.handleBye)
	mux.Handle("/", static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only this window: no other host name (DNS rebinding), no other page (token).
		if r.Host != s.addr {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") &&
			subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(s.token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func (s *server) touch() {
	s.mu.Lock()
	s.lastSeen, s.seen = time.Now(), true
	s.mu.Unlock()
}

// handleEvents returns the events after ?since=, waiting up to hold for one.
func (s *server) handleEvents(w http.ResponseWriter, r *http.Request) {
	s.touch() // only at the start: a request the closed window abandoned must not count
	since, _ := strconv.Atoi(r.URL.Query().Get("since"))
	timer := time.NewTimer(hold)
	defer timer.Stop()
	for {
		s.mu.Lock()
		if since < 0 || since > len(s.events) {
			since = 0
		}
		news := append([]event(nil), s.events[since:]...)
		wake := s.wake
		s.mu.Unlock()
		if len(news) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"app": s.app, "events": news})
			return
		}
		select {
		case <-wake:
		case <-timer.C:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"app": s.app, "events": []event{}})
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (s *server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	var a struct {
		N     int    `json:"n"`
		Value string `json:"value"`
	}
	if r.Method != http.MethodPost || json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&a) != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if a.N == 0 || a.N != s.pending {
		http.Error(w, "this question was already answered", http.StatusConflict)
		return
	}
	s.pending = 0
	s.answer <- a.Value
}

func (s *server) handleBye(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.byeAt = time.Now()
	s.mu.Unlock()
}

// watch ends the program once its window has been closed.
func (s *server) watch() {
	for range time.Tick(time.Second) {
		s.mu.Lock()
		left := s.seen && time.Since(s.lastSeen) > gone ||
			!s.byeAt.IsZero() && s.lastSeen.Before(s.byeAt) && time.Since(s.byeAt) > closing
		code := s.code
		s.mu.Unlock()
		if left {
			os.Exit(code)
		}
	}
}

func (s *server) emit(e event) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emitLocked(e)
}

func (s *server) emitLocked(e event) int {
	e.N = len(s.events) + 1
	s.events = append(s.events, e)
	close(s.wake)
	s.wake = make(chan struct{})
	return e.N
}

// ask shows a question and waits for the window's answer.
func (s *server) ask(e event) string {
	e.T = "prompt"
	ch := make(chan string, 1)
	s.mu.Lock()
	s.answer = ch
	s.pending = s.emitLocked(e)
	s.mu.Unlock()
	return <-ch
}

// end shows the last page and leaves the program running until the window closes.
func (s *server) end(e event, code int) {
	s.mu.Lock()
	s.code = code
	s.emitLocked(e)
	s.mu.Unlock()
	select {}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	alert("cleantv", msg)
	os.Exit(1)
}
