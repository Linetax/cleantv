package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeTV answers like sdbd on a Tizen TV: sysinfo, sync push and pull, and a vd_appinstall
// shell. files holds what the TV has; every completed push is also sent on pushed.
func fakeTV(t *testing.T, version, shellOut string, files map[string][]byte) (ip string, pushed chan []byte) {
	ln, err := net.Listen("tcp", "127.0.0.1:26101")
	if err != nil {
		t.Skip("port 26101 busy: ", err)
	}
	t.Cleanup(func() { ln.Close() })
	pushed = make(chan []byte, 4)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		services := map[uint32]string{}
		var sync bytes.Buffer
		send := func(p packet) { _ = writePacket(c, p) }
		syncMsg := func(id string, data []byte) []byte {
			b := make([]byte, 8, 8+len(data))
			copy(b, id)
			binary.LittleEndian.PutUint32(b[4:], uint32(len(data)))
			return append(b, data...)
		}
		for {
			p, err := readPacket(c)
			if err != nil {
				return
			}
			switch p.cmd {
			case cmdCNXN:
				send(packet{cmdCNXN, 0x01000000, 4096, []byte("device::\x00")})
			case cmdOPEN:
				svc := strings.TrimRight(string(p.data), "\x00")
				remote := p.arg0 + 100
				services[remote] = svc
				send(packet{cmdOKAY, remote, p.arg0, nil})
				switch {
				case svc == "sysinfo:":
					info := make([]byte, 64*5)
					copy(info[192:], version)
					send(packet{cmdWRTE, remote, p.arg0, info})
				case strings.HasPrefix(svc, "shell:0 vd_appinstall xvvl3S1bvH /home/owner/share/tmp/sdk_tools/package.wgt"):
					send(packet{cmdWRTE, remote, p.arg0, []byte(shellOut)})
					send(packet{cmdCLSE, remote, p.arg0, nil})
				case strings.HasPrefix(svc, "shell:"):
					t.Errorf("unexpected shell %q", svc)
				}
			case cmdWRTE:
				send(packet{cmdOKAY, p.arg1, p.arg0, nil})
				if services[p.arg1] != "sync:" {
					continue
				}
				sync.Write(p.data)
				for {
					kind, path, data, n := parseSync(sync.Bytes())
					if n == 0 {
						break
					}
					sync.Next(n)
					switch kind {
					case "SEND":
						files[path] = data
						pushed <- data
						send(packet{cmdWRTE, p.arg1, p.arg0, []byte("OKAY\x00\x00\x00\x00")})
					case "RECV":
						if f, ok := files[path]; ok {
							send(packet{cmdWRTE, p.arg1, p.arg0, append(syncMsg("DATA", f), syncMsg("DONE", nil)...)})
						} else {
							send(packet{cmdWRTE, p.arg1, p.arg0, syncMsg("FAIL", []byte("No such file or directory"))})
						}
					}
				}
			}
		}
	}()
	return "127.0.0.1", pushed
}

// parseSync reads one complete sync request: SEND (path, DATA..., DONE), RECV or QUIT. It
// returns how many bytes it used, 0 while the request is incomplete.
func parseSync(b []byte) (kind, path string, file []byte, used int) {
	if len(b) < 8 {
		return "", "", nil, 0
	}
	kind, n := string(b[:4]), int(binary.LittleEndian.Uint32(b[4:8]))
	if kind == "QUIT" {
		return kind, "", nil, 8
	}
	if len(b) < 8+n {
		return "", "", nil, 0
	}
	path, used = string(b[8:8+n]), 8+n
	if kind == "RECV" {
		return kind, path, nil, used
	}
	path, _, _ = strings.Cut(path, ",") // SEND "path,mode"
	for len(b) >= used+8 {
		id, n := string(b[used:used+4]), int(binary.LittleEndian.Uint32(b[used+4:used+8]))
		if id == "DONE" {
			return kind, path, file, used + 8
		}
		if len(b) < used+8+n {
			break
		}
		file = append(file, b[used+8:used+8+n]...)
		used += 8 + n
	}
	return "", "", nil, 0
}

func TestInstallOnFakeTV(t *testing.T) {
	ip, pushed := fakeTV(t, "6.0", "app_id[xvvl3S1bvH.TizenBrewStandalone] install completed\nspend time for wascmd is [900]ms\n", map[string][]byte{})
	s, err := dialSDB(ip)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.deadline(10 * time.Second)
	if v, err := s.platformVersion(); err != nil || tizenMajor(v) != 6 {
		t.Fatalf("version %q %v", v, err)
	}
	wgt := bytes.Repeat([]byte("tizenbrew"), 5000) // several sync chunks and packets
	if err := s.push(remoteWgt, wgt, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := <-pushed; !bytes.Equal(got, wgt) {
		t.Fatalf("pushed %d bytes, want %d", len(got), len(wgt))
	}
	out, err := s.shell("0 vd_appinstall "+tizenbrewPkg+" "+remoteWgt, nil)
	if ok, _ := installResult(out); err != nil || !ok {
		t.Fatalf("install: %q %v", out, err)
	}
}

func TestAddModule(t *testing.T) {
	for name, tc := range map[string]struct {
		before string // "" = no settings file yet
		want   []string
	}{
		"fresh TizenBrew": {"", []string{"npm/@foxreis/tizentube", cinefluxModule}},
		"keeps other modules": {`{"modules":["npm/@foxreis/tizentube","npm/other"],"autoLaunchModule":"npm/other"}`,
			[]string{"npm/@foxreis/tizentube", "npm/other", cinefluxModule}},
		"already there": {`{"modules":["` + cinefluxModule + `"]}`, []string{cinefluxModule}},
	} {
		t.Run(name, func(t *testing.T) {
			files := map[string][]byte{}
			if tc.before != "" {
				files[tizenbrewConfig] = []byte(tc.before)
			}
			ip, _ := fakeTV(t, "6.0", "", files)
			s, err := dialSDB(ip)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			s.deadline(10 * time.Second)
			if err := addModule(s, cinefluxModule); err != nil {
				t.Fatal(err)
			}
			got, err := s.pull(tizenbrewConfig)
			if err != nil {
				t.Fatal(err)
			}
			var cfg struct {
				Modules          []string
				AutoLaunchModule string
			}
			if err := json.Unmarshal(got, &cfg); err != nil || strings.Join(cfg.Modules, " ") != strings.Join(tc.want, " ") {
				t.Fatalf("settings = %s (%v)", got, err)
			}
			if name == "keeps other modules" && cfg.AutoLaunchModule != "npm/other" {
				t.Errorf("lost autoLaunchModule: %s", got)
			}
		})
	}
}

func TestInstallResult(t *testing.T) {
	for out, want := range map[string]bool{
		"app_id[x] install completed\nspend time for wascmd is [1]ms": true,
		"app_id[x] install failed[118, -16], reason: ...":             false,
		"": false,
	} {
		if ok, _ := installResult(out); ok != want {
			t.Errorf("%q: got %v", out, ok)
		}
	}
	if tizenMajor("5.0") != 5 || tizenMajor("") != 0 {
		t.Error("tizenMajor")
	}
}
