package network

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type WSConn struct {
	conn    net.Conn
	reader  *bufio.Reader
	writeMu sync.Mutex
}

func upgradeWebSocket(w http.ResponseWriter, r *http.Request) (*WSConn, error) {
	if !headerContains(r.Header, "Connection", "upgrade") || !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("not a websocket upgrade request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("missing Sec-WebSocket-Key")
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("response writer does not support hijacking")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	hash := sha1.New()
	_, _ = hash.Write([]byte(key + websocketGUID))
	accept := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return &WSConn{conn: conn, reader: rw.Reader}, nil
}

func headerContains(header http.Header, key, needle string) bool {
	for _, value := range header.Values(key) {
		for _, part := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), needle) {
				return true
			}
		}
	}
	return false
}

func (c *WSConn) ReadText() ([]byte, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return nil, err
		}
		switch opcode {
		case 0x1:
			return payload, nil
		case 0x8:
			_ = c.WriteClose()
			return nil, io.EOF
		case 0x9:
			if err := c.writeFrame(0xA, payload); err != nil {
				return nil, err
			}
		case 0xA:
			continue
		default:
			return nil, fmt.Errorf("unsupported websocket opcode %d", opcode)
		}
	}
}

func (c *WSConn) WriteJSON(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.WriteText(payload)
}

func (c *WSConn) WriteText(payload []byte) error {
	return c.writeFrame(0x1, payload)
}

func (c *WSConn) WriteClose() error {
	return c.writeFrame(0x8, nil)
}

func (c *WSConn) Close() error {
	return c.conn.Close()
}

func (c *WSConn) readFrame() (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return 0, nil, err
	}

	fin := header[0]&0x80 != 0
	opcode := header[0] & 0x0F
	if !fin {
		return 0, nil, fmt.Errorf("fragmented websocket frames are not supported")
	}

	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)
	switch length {
	case 126:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(buf))
	case 127:
		buf := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, buf); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(buf)
	}

	if length > 1<<20 {
		return 0, nil, fmt.Errorf("websocket frame too large: %d", length)
	}

	mask := make([]byte, 4)
	if masked {
		if _, err := io.ReadFull(c.reader, mask); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return opcode, payload, nil
}

func (c *WSConn) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var frame bytes.Buffer
	frame.WriteByte(0x80 | opcode)
	length := len(payload)
	switch {
	case length < 126:
		frame.WriteByte(byte(length))
	case length <= 0xFFFF:
		frame.WriteByte(126)
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(length))
		frame.Write(buf[:])
	default:
		frame.WriteByte(127)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(length))
		frame.Write(buf[:])
	}
	frame.Write(payload)
	_, err := c.conn.Write(frame.Bytes())
	return err
}
