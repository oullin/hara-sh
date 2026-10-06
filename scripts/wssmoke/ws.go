package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// A minimal RFC 6455 client: enough to open a socket, exchange control frames and close it.
// The standard library has no WebSocket client, and the checks need nothing more.

// errRefused is a handshake answered without switching protocols, with that status.
type errRefused struct{ status int }

type conn struct {
	net.Conn
	r *bufio.Reader
}

const (
	opText  = 0x1
	opClose = 0x8
	opPing  = 0x9
	opPong  = 0xa

	// RFC 6455 section 1.3: the server proves it read the handshake by hashing the key with this.
	acceptGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

func (e errRefused) Error() string {
	return fmt.Sprintf("HTTP %d %s instead of 101", e.status, http.StatusText(e.status))
}

// dial opens a WebSocket at base+path (http/https or ws/wss). An empty key sends no Authorization.
func dial(base, path, key string, tlsConf *tls.Config, timeout time.Duration) (*conn, error) {
	u, err := url.Parse(base + path)

	if err != nil {
		return nil, err
	}

	secure := u.Scheme == "https" || u.Scheme == "wss"
	host := u.Host

	if u.Port() == "" {
		host = net.JoinHostPort(u.Hostname(), map[bool]string{false: "80", true: "443"}[secure])
	}

	d := &net.Dialer{Timeout: timeout}

	var nc net.Conn

	if secure {
		conf := tlsConf.Clone()

		if conf == nil {
			conf = &tls.Config{}
		}

		conf.ServerName = u.Hostname()
		// HTTP/2 has no Upgrade header; the socket must be negotiated over HTTP/1.1.
		conf.NextProtos = []string{"http/1.1"}
		nc, err = tls.DialWithDialer(d, "tcp", host, conf)
	} else {
		nc, err = d.Dial("tcp", host)
	}

	if err != nil {
		return nil, err
	}

	c := &conn{Conn: nc, r: bufio.NewReader(nc)}

	if err := c.handshake(u, key, timeout); err != nil {
		nc.Close()

		return nil, err
	}

	return c, nil
}

func (c *conn) handshake(u *url.URL, key string, timeout time.Duration) error {
	nonce := make([]byte, 16)

	rand.Read(nonce)

	challenge := base64.StdEncoding.EncodeToString(nonce)

	req := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: u.Path, RawQuery: u.RawQuery}, Host: u.Host, Header: http.Header{}}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", challenge)

	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	c.SetDeadline(time.Now().Add(timeout))

	defer c.SetDeadline(time.Time{})

	if err := req.Write(c); err != nil {
		return err
	}

	resp, err := http.ReadResponse(c.r, req)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		resp.Body.Close()

		return errRefused{status: resp.StatusCode}
	}

	sum := sha1.Sum([]byte(challenge + acceptGUID))

	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != base64.StdEncoding.EncodeToString(sum[:]) {
		return fmt.Errorf("101 with a wrong Sec-WebSocket-Accept %q (a hop rewrote the handshake)", got)
	}

	return nil
}

// write sends one masked, unfragmented frame, as every client frame must be.
func (c *conn) write(op byte, payload []byte) error {
	frame := []byte{0x80 | op}

	switch n := len(payload); {
	case n < 126:
		frame = append(frame, 0x80|byte(n))
	case n <= 0xffff:
		frame = append(frame, 0x80|126)
		frame = binary.BigEndian.AppendUint16(frame, uint16(n))
	default:
		frame = append(frame, 0x80|127)
		frame = binary.BigEndian.AppendUint64(frame, uint64(n))
	}

	mask := make([]byte, 4)

	rand.Read(mask)

	frame = append(frame, mask...)

	for i, b := range payload {
		frame = append(frame, b^mask[i%4])
	}

	_, err := c.Write(frame)

	return err
}

// read returns the next frame. Server frames are unmasked; fragments are returned as they come.
func (c *conn) read() (byte, []byte, error) {
	var head [2]byte

	if _, err := io.ReadFull(c.r, head[:]); err != nil {
		return 0, nil, err
	}

	n := uint64(head[1] & 0x7f)

	switch n {
	case 126:
		var ext [2]byte

		if _, err := io.ReadFull(c.r, ext[:]); err != nil {
			return 0, nil, err
		}

		n = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte

		if _, err := io.ReadFull(c.r, ext[:]); err != nil {
			return 0, nil, err
		}

		n = binary.BigEndian.Uint64(ext[:])
	}

	if head[1]&0x80 != 0 {
		return 0, nil, errors.New("the server sent a masked frame")
	}

	if n > 16<<20 {
		return 0, nil, fmt.Errorf("frame of %d bytes is larger than this check reads", n)
	}

	payload := make([]byte, n)

	if _, err := io.ReadFull(c.r, payload); err != nil {
		return 0, nil, err
	}

	return head[0] & 0x0f, payload, nil
}

// ping sends a ping and waits for the pong that echoes it, answering server pings and skipping
// data frames meanwhile. A pong proves the frame crossed every hop in both directions.
func (c *conn) ping(timeout time.Duration) error {
	payload := make([]byte, 8)

	rand.Read(payload)

	c.SetDeadline(time.Now().Add(timeout))

	defer c.SetDeadline(time.Time{})

	if err := c.write(opPing, payload); err != nil {
		return err
	}

	for {
		op, data, err := c.read()

		if err != nil {
			return fmt.Errorf("no pong: %w", err)
		}

		switch {
		case op == opPong && string(data) == string(payload):
			return nil
		case op == opPing:
			if err := c.write(opPong, data); err != nil {
				return err
			}
		case op == opClose:
			return fmt.Errorf("the server closed the socket instead of answering (%s)", closeReason(data))
		}
	}
}

// close runs the closing handshake: send 1000 and wait for the server's close frame.
func (c *conn) close(timeout time.Duration) error {
	defer c.Conn.Close()

	c.SetDeadline(time.Now().Add(timeout))

	if err := c.write(opClose, binary.BigEndian.AppendUint16(nil, 1000)); err != nil {
		return err
	}

	for {
		op, _, err := c.read()

		if err != nil {
			return fmt.Errorf("no close frame back: %w", err)
		}

		if op == opClose {
			return nil
		}
	}
}

func closeReason(data []byte) string {
	if len(data) < 2 {
		return "no status"
	}

	return fmt.Sprintf("code %d %q", binary.BigEndian.Uint16(data), data[2:])
}
