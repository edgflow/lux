package websocket

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
)

const WebSocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// OpCode represents a WebSocket frame type
type OpCode byte

const (
	OpContinuation OpCode = 0x0
	OpText         OpCode = 0x1
	OpBinary       OpCode = 0x2
	OpClose        OpCode = 0x8
	OpPing         OpCode = 0x9
	OpPong         OpCode = 0xA
)

// Message represents a WebSocket message
type Message struct {
	OpCode  OpCode
	Payload []byte
}

// Conn represents a WebSocket connection
type Conn struct {
	conn      net.Conn
	writeMu   sync.Mutex
	closeSent bool
}

// Server represents a WebSocket server
type Server struct {
	Addr    string
	Handler func(*Conn)
}

// NewServer creates a new WebSocket server
func NewServer(addr string, handler func(*Conn)) *Server {
	return &Server{
		Addr:    addr,
		Handler: handler,
	}
}

// ListenAndServe starts the WebSocket server
func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		go s.handleConnection(conn)
	}
}

// handleConnection handles the WebSocket handshake and passes the connection to the handler
func (s *Server) handleConnection(conn net.Conn) {
	wsConn, err := Upgrade(conn)
	if err != nil {
		conn.Close()
		return
	}

	s.Handler(wsConn)
}

// Upgrade upgrades a TCP connection to a WebSocket connection
func Upgrade(conn net.Conn) (*Conn, error) {
	// Buffer to read the HTTP upgrade request
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	// Parse the HTTP headers
	request := string(buf[:n])
	headers := parseHeaders(request)

	// Check if it's a WebSocket upgrade request
	if headers["Upgrade"] != "websocket" {
		return nil, fmt.Errorf("not a WebSocket upgrade request")
	}

	// Get the WebSocket key and generate the accept key
	key := headers["Sec-WebSocket-Key"]
	acceptKey := generateAcceptKey(key)

	// Send the WebSocket handshake response
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	_, err = conn.Write([]byte(response))
	if err != nil {
		return nil, err
	}

	return &Conn{conn: conn}, nil
}

// parseHeaders parses HTTP headers
func parseHeaders(request string) map[string]string {
	headers := make(map[string]string)
	lines := strings.Split(request, "\r\n")

	for _, line := range lines {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			headers[parts[0]] = parts[1]
		}
	}

	return headers
}

// generateAcceptKey generates the Sec-WebSocket-Accept value
func generateAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + WebSocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// ReadMessage reads a message from the WebSocket connection
func (c *Conn) ReadMessage() (*Message, error) {
	// Read frame header
	header := make([]byte, 2)
	_, err := io.ReadFull(c.conn, header)
	if err != nil {
		return nil, err
	}

	// Parse basic frame information
	fin := (header[0] & 0x80) != 0
	opcode := OpCode(header[0] & 0x0F)
	masked := (header[1] & 0x80) != 0
	payloadLen := int(header[1] & 0x7F)

	// Handle extended payload length
	if payloadLen == 126 {
		extLen := make([]byte, 2)
		_, err := io.ReadFull(c.conn, extLen)
		if err != nil {
			return nil, err
		}
		payloadLen = int(extLen[0])<<8 | int(extLen[1])
	} else if payloadLen == 127 {
		extLen := make([]byte, 8)
		_, err := io.ReadFull(c.conn, extLen)
		if err != nil {
			return nil, err
		}
		// For simplicity, we're not handling 8-byte lengths fully
		payloadLen = int(extLen[6])<<8 | int(extLen[7])
	}

	// Read masking key if frame is masked
	var maskingKey []byte
	if masked {
		maskingKey = make([]byte, 4)
		_, err := io.ReadFull(c.conn, maskingKey)
		if err != nil {
			return nil, err
		}
	}

	// Read payload
	payload := make([]byte, payloadLen)
	_, err = io.ReadFull(c.conn, payload)
	if err != nil {
		return nil, err
	}

	// Unmask the payload if necessary
	if masked {
		for i := 0; i < payloadLen; i++ {
			payload[i] ^= maskingKey[i%4]
		}
	}

	// Handle continuation frames
	if !fin && opcode != OpContinuation {
		// Start of a fragmented message
		// For simplicity, we don't handle fragmented messages in this example
		return &Message{OpCode: opcode, Payload: payload}, nil
	}

	return &Message{OpCode: opcode, Payload: payload}, nil
}

// WriteMessage writes a message to the WebSocket connection
func (c *Conn) WriteMessage(opcode OpCode, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.closeSent {
		return fmt.Errorf("connection closed")
	}

	payloadLen := len(payload)

	// Create frame header
	var header []byte

	// First byte: FIN bit set (1), RSV1-3 are 0, opcode
	header = append(header, 0x80|byte(opcode))

	// Second byte: No mask bit (0), and payload length
	if payloadLen < 126 {
		header = append(header, byte(payloadLen))
	} else if payloadLen < 65536 {
		header = append(header, 126)
		header = append(header, byte(payloadLen>>8), byte(payloadLen))
	} else {
		header = append(header, 127)
		// For simplicity, we only use the last 2 bytes for length
		header = append(header, 0, 0, 0, 0, 0, 0)
		header = append(header, byte(payloadLen>>8), byte(payloadLen))
	}

	// Send header followed by payload
	_, err := c.conn.Write(header)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(payload)
	if err != nil {
		return err
	}

	// Mark connection as closed if this was a close frame
	if opcode == OpClose {
		c.closeSent = true
	}

	return nil
}

// WriteText writes a text message to the WebSocket connection
func (c *Conn) WriteText(message string) error {
	return c.WriteMessage(OpText, []byte(message))
}

// WriteBinary writes a binary message to the WebSocket connection
func (c *Conn) WriteBinary(data []byte) error {
	return c.WriteMessage(OpBinary, data)
}

// Close closes the WebSocket connection
func (c *Conn) Close() error {
	// Send close frame if not already sent
	if !c.closeSent {
		err := c.WriteMessage(OpClose, nil)
		if err != nil {
			c.conn.Close()
			return err
		}
	}
	return c.conn.Close()
}

// Ping sends a ping message
func (c *Conn) Ping(data []byte) error {
	return c.WriteMessage(OpPing, data)
}

// Pong sends a pong message
func (c *Conn) Pong(data []byte) error {
	return c.WriteMessage(OpPong, data)
}
