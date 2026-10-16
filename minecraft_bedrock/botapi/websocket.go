package main

// A minimal WebSocket client (standard library only) for Home Assistant's
// WebSocket API through the Supervisor (ws://supervisor/core/websocket),
// which add-ons with homeassistant_api may use with their SUPERVISOR_TOKEN.

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type haSocket struct {
	conn   net.Conn
	reader *bufio.Reader
	nextID int
}

func dialHomeAssistant() (*haSocket, error) {
	token := envOr("SUPERVISOR_TOKEN", "")
	if token == "" {
		return nil, errors.New("no Supervisor token")
	}
	target, err := url.Parse(envOr("SUPERVISOR_URL", "http://supervisor") + "/core/websocket")
	if err != nil {
		return nil, err
	}
	host := target.Host
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nAuthorization: Bearer %s\r\n\r\n",
		target.Path, target.Host, base64.StdEncoding.EncodeToString(key), token)
	if _, err := conn.Write([]byte(request)); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil || response.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("websocket upgrade refused: %v", err)
	}
	ws := &haSocket{conn: conn, reader: reader}
	var hello map[string]any
	if err := ws.read(&hello); err != nil || hello["type"] != "auth_required" {
		ws.Close()
		return nil, errors.New("unexpected greeting")
	}
	if err := ws.write(map[string]any{"type": "auth", "access_token": token}); err != nil {
		ws.Close()
		return nil, err
	}
	var auth map[string]any
	if err := ws.read(&auth); err != nil || auth["type"] != "auth_ok" {
		ws.Close()
		return nil, fmt.Errorf("authentication failed: %v", auth["message"])
	}
	return ws, nil
}

func (ws *haSocket) Close() { ws.conn.Close() }

func (ws *haSocket) write(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	header := []byte{0x81} // final text frame
	switch n := len(payload); {
	case n < 126:
		header = append(header, byte(n)|0x80)
	case n < 65536:
		header = append(header, 126|0x80, byte(n>>8), byte(n))
	default:
		header = append(header, 127|0x80)
		header = binary.BigEndian.AppendUint64(header, uint64(n))
	}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	header = append(header, mask...)
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	_, err = ws.conn.Write(append(header, masked...))
	return err
}

// read reads one whole message (joining fragments, answering pings)
func (ws *haSocket) read(into any) error {
	var message []byte
	for {
		head := make([]byte, 2)
		if _, err := io.ReadFull(ws.reader, head); err != nil {
			return err
		}
		opcode, length := head[0]&0x0f, uint64(head[1]&0x7f)
		switch length {
		case 126:
			ext := make([]byte, 2)
			if _, err := io.ReadFull(ws.reader, ext); err != nil {
				return err
			}
			length = uint64(binary.BigEndian.Uint16(ext))
		case 127:
			ext := make([]byte, 8)
			if _, err := io.ReadFull(ws.reader, ext); err != nil {
				return err
			}
			length = binary.BigEndian.Uint64(ext)
		}
		if length > 64<<20 {
			return errors.New("message too large")
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(ws.reader, payload); err != nil {
			return err
		}
		switch opcode {
		case 0x8:
			return errors.New("connection closed")
		case 0x9: // ping: answer with a masked pong
			mask := make([]byte, 4)
			_, _ = rand.Read(mask)
			frame := append([]byte{0x8a, byte(len(payload)) | 0x80}, mask...)
			for i, b := range payload {
				frame = append(frame, b^mask[i%4])
			}
			_, _ = ws.conn.Write(frame)
			continue
		case 0xa:
			continue
		}
		message = append(message, payload...)
		if head[0]&0x80 != 0 {
			return json.Unmarshal(message, into)
		}
	}
}

// call sends a command and returns its result, or the error Home Assistant gave
func (ws *haSocket) call(command map[string]any) (json.RawMessage, error) {
	ws.nextID++
	id := ws.nextID
	command["id"] = id
	_ = ws.conn.SetDeadline(time.Now().Add(30 * time.Second))
	if err := ws.write(command); err != nil {
		return nil, err
	}
	for {
		var reply struct {
			ID      int             `json:"id"`
			Type    string          `json:"type"`
			Success bool            `json:"success"`
			Result  json.RawMessage `json:"result"`
			Error   *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := ws.read(&reply); err != nil {
			return nil, err
		}
		if reply.ID != id || reply.Type != "result" {
			continue
		}
		if !reply.Success {
			if reply.Error != nil {
				return nil, fmt.Errorf("%s: %s", reply.Error.Code, reply.Error.Message)
			}
			return nil, errors.New("command failed")
		}
		return reply.Result, nil
	}
}
