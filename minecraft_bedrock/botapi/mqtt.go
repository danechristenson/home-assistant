package main

// A small MQTT 3.1.1 client (standard library only): connect with a last
// will, publish (QoS 0, optionally retained), subscribe, keep alive and
// reconnect. Enough for Home Assistant's MQTT discovery and commands.

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type mqttMessage struct {
	topic   string
	payload []byte
}

type mqttClient struct {
	address, username, password, clientID string
	willTopic, willPayload                string
	subscriptions                         []string
	onConnect                             func()
	onMessage                             func(mqttMessage)

	mu   sync.Mutex
	conn net.Conn
}

func mqttString(s string) []byte {
	out := make([]byte, 2, 2+len(s))
	binary.BigEndian.PutUint16(out, uint16(len(s)))
	return append(out, s...)
}

func mqttPacket(kind byte, body []byte) []byte {
	out := []byte{kind}
	length := len(body)
	for {
		digit := byte(length % 128)
		length /= 128
		if length > 0 {
			digit |= 0x80
		}
		out = append(out, digit)
		if length == 0 {
			break
		}
	}
	return append(out, body...)
}

func readPacket(r *bufio.Reader) (byte, []byte, error) {
	kind, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	length, multiplier := 0, 1
	for i := 0; ; i++ {
		digit, err := r.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		length += int(digit&0x7f) * multiplier
		if digit&0x80 == 0 {
			break
		}
		if multiplier *= 128; i >= 3 {
			return 0, nil, errors.New("bad packet length")
		}
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return kind, body, nil
}

func (c *mqttClient) send(packet []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return errors.New("not connected")
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := c.conn.Write(packet)
	return err
}

// Publish sends a QoS 0 message; it's dropped while disconnected (retained
// state is republished on every reconnect)
func (c *mqttClient) Publish(topic string, payload []byte, retain bool) {
	flags := byte(0x30)
	if retain {
		flags |= 0x01
	}
	_ = c.send(mqttPacket(flags, append(mqttString(topic), payload...)))
}

func (c *mqttClient) connectOnce() (*bufio.Reader, error) {
	conn, err := net.DialTimeout("tcp", c.address, 10*time.Second)
	if err != nil {
		return nil, err
	}
	flags := byte(0x02) // clean session
	body := append(mqttString("MQTT"), 4)
	payload := mqttString(c.clientID)
	if c.willTopic != "" {
		flags |= 0x04 | 0x20 // will, retained
		payload = append(payload, mqttString(c.willTopic)...)
		payload = append(payload, mqttString(c.willPayload)...)
	}
	if c.username != "" {
		flags |= 0x80
		payload = append(payload, mqttString(c.username)...)
		if c.password != "" {
			flags |= 0x40
			payload = append(payload, mqttString(c.password)...)
		}
	}
	body = append(body, flags, 0, 60) // keep alive 60 s
	body = append(body, payload...)
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if _, err := conn.Write(mqttPacket(0x10, body)); err != nil {
		conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(conn)
	kind, ack, err := readPacket(reader)
	if err != nil || kind != 0x20 || len(ack) != 2 {
		conn.Close()
		return nil, fmt.Errorf("no CONNACK: %v", err)
	}
	if ack[1] != 0 {
		conn.Close()
		return nil, fmt.Errorf("broker refused the connection (code %d)", ack[1])
	}
	_ = conn.SetDeadline(time.Time{})
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	if len(c.subscriptions) > 0 {
		body := []byte{0, 1} // packet id 1
		for _, topic := range c.subscriptions {
			body = append(body, mqttString(topic)...)
			body = append(body, 0) // QoS 0
		}
		if err := c.send(mqttPacket(0x82, body)); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return reader, nil
}

// Run connects, calls onConnect, delivers messages and reconnects forever
func (c *mqttClient) Run() {
	delay := 5 * time.Second
	for {
		reader, err := c.connectOnce()
		if err != nil {
			log.Printf("[WARN] MQTT: %v; retrying in %s", err, delay)
			time.Sleep(delay)
			if delay < 2*time.Minute {
				delay *= 2
			}
			continue
		}
		delay = 5 * time.Second
		log.Printf("[INFO] Connected to MQTT; Home Assistant entities are published")
		if c.onConnect != nil {
			c.onConnect()
		}
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					_ = c.send([]byte{0xc0, 0}) // PINGREQ
				}
			}
		}()
		for {
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()
			_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
			kind, body, err := readPacket(reader)
			if err != nil {
				log.Printf("[WARN] MQTT connection lost: %v", err)
				break
			}
			if kind&0xf0 == 0x30 && len(body) >= 2 { // PUBLISH (QoS 0 from our subscriptions)
				n := int(binary.BigEndian.Uint16(body))
				if 2+n <= len(body) && c.onMessage != nil {
					c.onMessage(mqttMessage{topic: string(body[2 : 2+n]), payload: body[2+n:]})
				}
			}
		}
		close(done)
		c.mu.Lock()
		c.conn.Close()
		c.conn = nil
		c.mu.Unlock()
		time.Sleep(delay)
	}
}
