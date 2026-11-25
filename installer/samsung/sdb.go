package main

// A minimal client for sdb, Samsung's fork of the ADB wire protocol, spoken by a Samsung TV
// in developer mode on TCP 26101. Only what installing a widget needs: sysinfo, push, shell.
// Same packets TizenBrew Installer sends (adbhost: no AUTH, version 0x01000000, maxdata 4096).

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	cmdCNXN = 0x4e584e43
	cmdOPEN = 0x4e45504f
	cmdOKAY = 0x59414b4f
	cmdCLSE = 0x45534c43
	cmdWRTE = 0x45545257
)

type packet struct {
	cmd, arg0, arg1 uint32
	data            []byte
}

type sdb struct {
	conn    net.Conn
	localID uint32
}

func writePacket(w io.Writer, p packet) error {
	var sum uint32
	for _, b := range p.data {
		sum += uint32(b)
	}
	h := make([]byte, 24)
	binary.LittleEndian.PutUint32(h[0:], p.cmd)
	binary.LittleEndian.PutUint32(h[4:], p.arg0)
	binary.LittleEndian.PutUint32(h[8:], p.arg1)
	binary.LittleEndian.PutUint32(h[12:], uint32(len(p.data)))
	binary.LittleEndian.PutUint32(h[16:], sum)
	binary.LittleEndian.PutUint32(h[20:], p.cmd^0xffffffff)
	_, err := w.Write(append(h, p.data...))
	return err
}

func readPacket(r io.Reader) (packet, error) {
	h := make([]byte, 24)
	if _, err := io.ReadFull(r, h); err != nil {
		return packet{}, err
	}
	p := packet{
		cmd:  binary.LittleEndian.Uint32(h[0:]),
		arg0: binary.LittleEndian.Uint32(h[4:]),
		arg1: binary.LittleEndian.Uint32(h[8:]),
	}
	n := binary.LittleEndian.Uint32(h[12:])
	if n > 1<<20 {
		return packet{}, fmt.Errorf("paquet sdb trop grand (%d octets)", n)
	}
	p.data = make([]byte, n)
	_, err := io.ReadFull(r, p.data)
	return p, err
}

func dialSDB(ip string) (*sdb, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "26101"), 5*time.Second)
	if err != nil {
		return nil, err
	}
	s := &sdb{conn: conn}
	s.deadline(10 * time.Second)
	if err := writePacket(conn, packet{cmdCNXN, 0x01000000, 4096, []byte("host::\x00")}); err != nil {
		conn.Close()
		return nil, err
	}
	for {
		p, err := readPacket(conn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		if p.cmd == cmdCNXN {
			return s, nil
		}
	}
}

func (s *sdb) Close() error { return s.conn.Close() }

func (s *sdb) deadline(d time.Duration) { _ = s.conn.SetDeadline(time.Now().Add(d)) }

// stream is one OPENed service.
type stream struct {
	s             *sdb
	local, remote uint32
	closed        bool
}

func (s *sdb) open(service string) (*stream, error) {
	s.localID++
	st := &stream{s: s, local: s.localID}
	if err := writePacket(s.conn, packet{cmdOPEN, st.local, 0, append([]byte(service), 0)}); err != nil {
		return nil, err
	}
	for {
		p, err := readPacket(s.conn)
		if err != nil {
			return nil, err
		}
		switch {
		case p.cmd == cmdOKAY && p.arg1 == st.local:
			st.remote = p.arg0
			return st, nil
		case p.cmd == cmdCLSE && p.arg1 == st.local:
			return nil, fmt.Errorf("la TV refuse le service %q", strings.SplitN(service, " ", 2)[0])
		}
	}
}

// write sends data and waits for the TV to acknowledge each packet.
func (st *stream) write(data []byte) error {
	for len(data) > 0 {
		n := min(len(data), 4096)
		if err := writePacket(st.s.conn, packet{cmdWRTE, st.local, st.remote, data[:n]}); err != nil {
			return err
		}
		data = data[n:]
		for {
			p, err := readPacket(st.s.conn)
			if err != nil {
				return err
			}
			if p.arg1 != st.local {
				continue
			}
			if p.cmd == cmdCLSE {
				st.closed = true
				return fmt.Errorf("la TV a fermé la connexion")
			}
			if p.cmd == cmdOKAY {
				break
			}
		}
	}
	return nil
}

// read returns the next chunk the TV sends, or io.EOF once it closes the stream.
func (st *stream) read() ([]byte, error) {
	for !st.closed {
		p, err := readPacket(st.s.conn)
		if err != nil {
			return nil, err
		}
		if p.arg1 != st.local {
			continue
		}
		switch p.cmd {
		case cmdWRTE:
			if err := writePacket(st.s.conn, packet{cmdOKAY, st.local, st.remote, nil}); err != nil {
				return nil, err
			}
			return p.data, nil
		case cmdCLSE:
			st.closed = true
		}
	}
	return nil, io.EOF
}

func (st *stream) close() {
	if !st.closed {
		_ = writePacket(st.s.conn, packet{cmdCLSE, st.local, st.remote, nil})
		st.closed = true
	}
}

// platformVersion reads Tizen's version ("5.0", "6.0"...) from the sysinfo service: fixed
// 64-byte fields, the platform version is the fourth.
func (s *sdb) platformVersion() (string, error) {
	st, err := s.open("sysinfo:")
	if err != nil {
		return "", err
	}
	defer st.close()
	var buf []byte
	for len(buf) < 256 {
		d, err := st.read()
		if err != nil {
			break
		}
		buf = append(buf, d...)
	}
	if len(buf) < 256 {
		return "", fmt.Errorf("réponse sysinfo trop courte")
	}
	return strings.TrimSpace(string(bytes.TrimRight(buf[192:256], "\x00"))), nil
}

// push copies data to path on the TV (sync service: SEND, DATA..., DONE).
func (s *sdb) push(path string, data []byte) error {
	st, err := s.open("sync:")
	if err != nil {
		return err
	}
	defer st.close()
	msg := func(id string, n int) []byte {
		b := make([]byte, 8)
		copy(b, id)
		binary.LittleEndian.PutUint32(b[4:], uint32(n))
		return b
	}
	spec := path + ",33261" // 0755, as TizenBrew Installer sends it
	var req bytes.Buffer
	req.Write(msg("SEND", len(spec)))
	req.WriteString(spec)
	for i := 0; i < len(data); i += 1420 {
		chunk := data[i:min(i+1420, len(data))]
		req.Write(msg("DATA", len(chunk)))
		req.Write(chunk)
	}
	req.Write(msg("DONE", int(time.Now().Unix())))
	if err := st.write(req.Bytes()); err != nil {
		return err
	}
	reply, err := st.read()
	if err != nil {
		return fmt.Errorf("pas de réponse après l'envoi : %v", err)
	}
	if !bytes.HasPrefix(reply, []byte("OKAY")) {
		return fmt.Errorf("envoi refusé : %s", strings.TrimSpace(string(reply[min(8, len(reply)):])))
	}
	_ = st.write(msg("QUIT", 0))
	return nil
}

// shell runs a command and returns everything it printed, until it exits or stop matches.
func (s *sdb) shell(cmd string, stop func(string) bool) (string, error) {
	st, err := s.open("shell:" + cmd)
	if err != nil {
		return "", err
	}
	defer st.close()
	var out strings.Builder
	for {
		d, err := st.read()
		if err == io.EOF {
			return out.String(), nil
		}
		if err != nil {
			return out.String(), err
		}
		out.Write(d)
		if stop != nil && stop(out.String()) {
			return out.String(), nil
		}
	}
}
