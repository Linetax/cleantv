package main

import "testing"

func TestParseTarget(t *testing.T) {
	for in, want := range map[string]string{
		"192.168.0.206":       "192.168.0.206:5555",
		" 10.0.0.2:5556":      "",
		"10.0.0.2:5556":       "10.0.0.2:5556",
		"192.168.0":           "",
		"tv.local":            "",
		"192.168.0.206:99999": "",
	} {
		got, ok := parseTarget(in)
		if got != want || ok != (want != "") {
			t.Errorf("parseTarget(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestParseList(t *testing.T) {
	got := parseList("# head\ncom.a   # ads\n\n  secure k = v \ncom.b\n")
	want := []string{"com.a", "secure k v", "com.b"}
	if len(got) != len(want) {
		t.Fatalf("parseList = %q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGuardList(t *testing.T) {
	other := "com.x/com.x.Svc"
	if got := guardList(other+":"+oldGuard+":"+pkgHome+"/.Guard", guard); got != other+":"+guard {
		t.Errorf("install: %q", got)
	}
	if got := guardList(other+":"+guard, ""); got != other {
		t.Errorf("undo: %q", got)
	}
	if got := guardList("", ""); got != "" {
		t.Errorf("empty: %q", got)
	}
}

func TestFuturity(t *testing.T) {
	c := "a=1,min_futurity=5000,b=2"
	if futurityIn(c) != "5000" || futurityIn("a=1") != "" {
		t.Error("futurityIn")
	}
	if got := futurityWith(c, "0"); got != "a=1,b=2,min_futurity=0" {
		t.Errorf("put: %q", got)
	}
	if got := futurityWith("min_futurity=0", ""); got != "" {
		t.Errorf("delete: %q", got)
	}
}

// The embedded lists must keep the packages that draw the screen out of reach.
func TestListsSafe(t *testing.T) {
	sdk = 30
	never := map[string]bool{}
	for _, p := range readList("never-disable.txt") {
		never[p] = true
	}
	if !never["com.amazon.tv.launcher"] {
		t.Fatal("never-disable list lost the Amazon launcher")
	}
	if len(readList("packages.txt")) < 20 {
		t.Fatal("packages list looks empty")
	}
}
