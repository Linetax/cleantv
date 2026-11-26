package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestWindowProtocol(t *testing.T) {
	s := newServer(App{Name: "test"})
	ts := httptest.NewServer(s.routes())
	defer ts.Close()
	s.addr = strings.TrimPrefix(ts.URL, "http://")

	get := func(path string) (int, string) {
		r, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		return r.StatusCode, string(b)
	}
	post := func(path, body string) int {
		r, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}

	if code, _ := get("/api/events?since=0"); code != http.StatusForbidden {
		t.Errorf("events without the token: %d, want 403", code)
	}
	if code, body := get("/"); code != 200 || !strings.Contains(body, "<html") {
		t.Errorf("page: %d", code)
	}

	got := make(chan string)
	go func() { got <- s.ask(event{Kind: "yesno", Text: "Continuer ?", Def: "oui"}) }()

	var res struct{ Events []event }
	_, body := get("/api/events?since=0&t=" + s.token)
	if err := json.Unmarshal([]byte(body), &res); err != nil || len(res.Events) != 1 || res.Events[0].T != "prompt" {
		t.Fatalf("events = %s (%v)", body, err)
	}
	n := res.Events[0].N
	if code := post("/api/answer?t=wrong", `{"n":1,"value":"non"}`); code != http.StatusForbidden {
		t.Errorf("answer with a wrong token: %d", code)
	}
	if code := post("/api/answer?t="+s.token, `{"n":99,"value":"non"}`); code != http.StatusConflict {
		t.Errorf("answer to another question: %d", code)
	}
	if code := post("/api/answer?t="+s.token, `{"n":`+strconv.Itoa(n)+`,"value":"non"}`); code != 200 {
		t.Errorf("answer: %d", code)
	}
	if v := <-got; v != "non" {
		t.Errorf("ask returned %q", v)
	}
	if code := post("/api/answer?t="+s.token, `{"n":`+strconv.Itoa(n)+`,"value":"oui"}`); code != http.StatusConflict {
		t.Errorf("second answer to the same question: %d", code)
	}

	// Another host name (DNS rebinding) is refused even with the token.
	req, _ := http.NewRequest("GET", ts.URL+"/api/events?t="+s.token, nil)
	req.Host = "evil.example:80"
	if r, err := http.DefaultClient.Do(req); err != nil || r.StatusCode != http.StatusForbidden {
		t.Errorf("foreign host: %v %v", r, err)
	}
}
