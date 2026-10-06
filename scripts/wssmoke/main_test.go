package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

// fakeProxy is a WebSocket server shaped like the proxy: it requires the client key, answers pings
// with pongs and echoes the closing handshake. The fields switch on the faults a hop can introduce.
type fakeProxy struct {
	open        bool // accept sockets without the key
	badAccept   bool // answer 101 with a wrong Sec-WebSocket-Accept
	closeOnPing bool // close instead of answering a ping
	chatter     bool // send data frames and a ping of its own before each pong
}

const goodKey = "client-key"

func (f fakeProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !f.open && r.Header.Get("Authorization") != "Bearer "+goodKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)

		return
	}

	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "not a websocket", http.StatusBadRequest)

		return
	}

	sum := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + acceptGUID))
	accept := base64.StdEncoding.EncodeToString(sum[:])

	if f.badAccept {
		accept = "wrong"
	}

	nc, rw, err := w.(http.Hijacker).Hijack()

	if err != nil {
		return
	}

	defer nc.Close()

	rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + accept + "\r\n\r\n")
	rw.Flush()

	for {
		op, payload, err := readMasked(rw.Reader)

		if err != nil {
			return
		}

		switch op {
		case opPing:
			if f.closeOnPing {
				nc.Write(serverFrame(opClose, binary.BigEndian.AppendUint16(nil, 1011), []byte("going away")))

				return
			}

			if f.chatter {
				nc.Write(serverFrame(opText, bytes.Repeat([]byte("a"), 200)))
				nc.Write(serverFrame(opText, bytes.Repeat([]byte("b"), 70000)))
				nc.Write(serverFrame(opPing, []byte("server")))
			}

			nc.Write(serverFrame(opPong, payload))
		case opPong:
			if string(payload) != "server" {
				return
			}
		case opClose:
			nc.Write(serverFrame(opClose, payload))

			return
		}
	}
}

// readMasked reads one client frame, which must be masked.
func readMasked(r *bufio.Reader) (byte, []byte, error) {
	var head [2]byte

	if _, err := io.ReadFull(r, head[:]); err != nil {
		return 0, nil, err
	}

	if head[1]&0x80 == 0 {
		return 0, nil, io.ErrUnexpectedEOF
	}

	n := uint64(head[1] & 0x7f)

	switch n {
	case 126:
		var ext [2]byte

		io.ReadFull(r, ext[:])
		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte

		io.ReadFull(r, ext[:])
		n = binary.BigEndian.Uint64(ext[:])
	}

	var mask [4]byte

	io.ReadFull(r, mask[:])
	payload := make([]byte, n)

	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}

	for i := range payload {
		payload[i] ^= mask[i%4]
	}

	return head[0] & 0x0f, payload, nil
}

func serverFrame(op byte, parts ...[]byte) []byte {
	payload := slices.Concat(parts...)
	frame := []byte{0x80 | op}

	switch n := len(payload); {
	case n < 126:
		frame = append(frame, byte(n))
	case n <= 0xffff:
		frame = append(frame, 126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(n))
	default:
		frame = append(frame, 127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(n))
	}

	return append(frame, payload...)
}

func newCheck(key string) check {
	return check{path: "/v1/responses", key: key, timeout: 2 * time.Second}
}

func runAgainst(t *testing.T, c check, srv *httptest.Server) (string, bool) {
	t.Helper()

	var out strings.Builder
	ok := c.run(&out, []string{srv.URL})

	return out.String(), ok
}

func TestAllStepsPass(t *testing.T) {
	srv := httptest.NewServer(fakeProxy{})

	defer srv.Close()

	c := newCheck(goodKey)
	c.idle = 10 * time.Millisecond
	out, ok := runAgainst(t, c, srv)

	if !ok {
		t.Fatalf("run failed:\n%s", out)
	}

	for _, want := range []string{srv.URL + "/v1/responses", "ok    handshake", "ok    ping/pong", "ok    idle 10ms", "ok    close", "ok    no key       401 as expected"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestServerTrafficBeforeThePong(t *testing.T) {
	srv := httptest.NewServer(fakeProxy{chatter: true})

	defer srv.Close()

	if out, ok := runAgainst(t, newCheck(goodKey), srv); !ok {
		t.Fatalf("extended-length frames and server pings must not fail the check:\n%s", out)
	}
}

func TestWrongKeyFailsTheHandshake(t *testing.T) {
	srv := httptest.NewServer(fakeProxy{})

	defer srv.Close()

	out, ok := runAgainst(t, newCheck("wrong"), srv)

	if ok || !strings.Contains(out, "FAIL  handshake    HTTP 401 Unauthorized instead of 101") {
		t.Fatalf("want a handshake failure:\n%s", out)
	}

	if !strings.Contains(out, "ok    no key") {
		t.Fatalf("the key check still runs after a failed handshake:\n%s", out)
	}
}

func TestOpenRouteFails(t *testing.T) {
	srv := httptest.NewServer(fakeProxy{open: true})

	defer srv.Close()

	out, ok := runAgainst(t, newCheck(goodKey), srv)

	if ok || !strings.Contains(out, "FAIL  no key       101 without a client key") {
		t.Fatalf("a socket opened without the key must fail:\n%s", out)
	}
}

func TestRewrittenHandshakeFails(t *testing.T) {
	srv := httptest.NewServer(fakeProxy{badAccept: true})

	defer srv.Close()

	out, ok := runAgainst(t, newCheck(goodKey), srv)

	if ok || !strings.Contains(out, `wrong Sec-WebSocket-Accept "wrong"`) {
		t.Fatalf("want an accept failure:\n%s", out)
	}
}

func TestCloseInsteadOfPongFails(t *testing.T) {
	srv := httptest.NewServer(fakeProxy{closeOnPing: true})

	defer srv.Close()

	out, ok := runAgainst(t, newCheck(goodKey), srv)

	if ok || !strings.Contains(out, `closed the socket instead of answering (code 1011 "going away")`) {
		t.Fatalf("want a ping failure:\n%s", out)
	}

	if strings.Contains(out, "close  ") {
		t.Fatalf("steps after a failure must not run:\n%s", out)
	}
}

func TestPlainHTTPRouteFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	defer srv.Close()

	out, ok := runAgainst(t, newCheck(goodKey), srv)

	if ok || !strings.Contains(out, "HTTP 200 OK instead of 101") {
		t.Fatalf("a hop that drops the upgrade must fail:\n%s", out)
	}
}

func TestUnreachableAddressFails(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()

	var out strings.Builder

	if newCheck(goodKey).run(&out, []string{"http://" + addr}) || !strings.Contains(out.String(), "connection refused") {
		t.Fatalf("want a dial failure:\n%s", out.String())
	}
}

func TestTLS(t *testing.T) {
	srv := httptest.NewTLSServer(fakeProxy{})

	defer srv.Close()

	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())

	c := newCheck(goodKey)
	c.tls = &tls.Config{RootCAs: pool}

	if out, ok := runAgainst(t, c, srv); !ok {
		t.Fatalf("run over TLS failed:\n%s", out)
	}

	c.tls = nil

	if out, ok := runAgainst(t, c, srv); ok || !strings.Contains(out, "certificate") {
		t.Fatalf("an untrusted certificate must fail:\n%s", out)
	}
}

func TestAddresses(t *testing.T) {
	got := addresses([]string{" https://hara.local/ ", "", "http://localhost:8317", "https://hara.local"})

	if !slices.Equal(got, []string{"https://hara.local", "http://localhost:8317"}) {
		t.Fatalf("addresses = %v", got)
	}

	if got := addresses(nil); !slices.Equal(got, []string{"https://hara.local"}) {
		t.Fatalf("default = %v", got)
	}
}
