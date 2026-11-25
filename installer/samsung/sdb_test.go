package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeTV answers like sdbd on a Tizen TV: sysinfo, sync push and a vd_appinstall shell.
func fakeTV(t *testing.T, version, shellOut string) (ip string, pushed chan []byte) {
	ln, err := net.Listen("tcp", "127.0.0.1:26101")
	if err != nil {
		t.Skip("port 26101 busy: ", err)
	}
	t.Cleanup(func() { ln.Close() })
	pushed = make(chan []byte, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		services := map[uint32]string{}
		var sync bytes.Buffer
		send := func(p packet) { _ = writePacket(c, p) }
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
				if data, done := parseSync(sync.Bytes()); done {
					pushed <- data
					sync.Reset()
					send(packet{cmdWRTE, p.arg1, p.arg0, []byte("OKAY\x00\x00\x00\x00")})
				}
			}
		}
	}()
	return "127.0.0.1", pushed
}

// parseSync returns the file once SEND, DATA... and DONE have all arrived.
func parseSync(b []byte) ([]byte, bool) {
	var file []byte
	for len(b) >= 8 {
		id, n := string(b[:4]), int(binary.LittleEndian.Uint32(b[4:8]))
		if id == "DONE" {
			return file, true
		}
		if len(b) < 8+n {
			return nil, false
		}
		if id == "DATA" {
			file = append(file, b[8:8+n]...)
		}
		b = b[8+n:]
	}
	return nil, false
}

func TestInstallOnFakeTV(t *testing.T) {
	ip, pushed := fakeTV(t, "6.0", "app_id[xvvl3S1bvH.TizenBrewStandalone] install completed\nspend time for wascmd is [900]ms\n")
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
	if err := s.push(remoteWgt, wgt); err != nil {
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
