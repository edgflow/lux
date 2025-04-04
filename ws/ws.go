package ws

import (
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
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

	// For handling fragmented messages
	fragmentBuffer []byte
	fragmentOpCode OpCode
}

// Server represents a WebSocket server
type Server struct {
	Addr      string
	Handler   func(*Conn)
	TLSConfig *tls.Config // Added TLS config
}

// NewServer creates a new WebSocket server
func NewServer(addr string, handler func(*Conn)) *Server {
	return &Server{
		Addr:    addr,
		Handler: handler,
	}
}

// NewTLSServer creates a new WebSocket server with TLS support
func NewTLSServer(addr string, handler func(*Conn), tlsConfig *tls.Config) *Server {
	return &Server{
		Addr:      addr,
		Handler:   handler,
		TLSConfig: tlsConfig,
	}
}

// ListenAndServe starts the WebSocket server
func (s *Server) ListenAndServe() error {
	var listener net.Listener
	var err error

	if s.TLSConfig != nil {
		// Create TLS listener if TLS config is provided
		listener, err = tls.Listen("tcp", s.Addr, s.TLSConfig)
	} else {
		// Create regular TCP listener
		listener, err = net.Listen("tcp", s.Addr)
	}

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

// ListenAndServeTLS starts the WebSocket server with TLS
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	listener, err := tls.Listen("tcp", s.Addr, tlsConfig)
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

// Dial connects to a WebSocket server
func Dial(url string) (*Conn, error) {
	// Parse the URL to determine if it's ws:// or wss://
	isSecure := strings.HasPrefix(url, "wss://")
	hostPort := strings.TrimPrefix(strings.TrimPrefix(url, "ws://"), "wss://")

	var conn net.Conn
	var err error

	if isSecure {
		// Connect with TLS for wss://
		conn, err = tls.Dial("tcp", hostPort, &tls.Config{})
	} else {
		// Connect without TLS for ws://
		conn, err = net.Dial("tcp", hostPort)
	}

	if err != nil {
		return nil, err
	}

	// Create the WebSocket handshake request
	key := generateRandomKey()
	request := fmt.Sprintf(
		"GET / HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n\r\n",
		hostPort, key)

	_, err = conn.Write([]byte(request))
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Read the handshake response
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Check if the response is valid
	response := string(buf[:n])
	if !strings.Contains(response, "HTTP/1.1 101") || !strings.Contains(response, "Upgrade: websocket") {
		conn.Close()
		return nil, fmt.Errorf("invalid handshake response")
	}

	return &Conn{conn: conn}, nil
}

// generateRandomKey generates a random key for the WebSocket handshake
func generateRandomKey() string {
	key := make([]byte, 16)
	// In a real implementation, use crypto/rand to generate random bytes
	for i := range key {
		key[i] = byte(i)
	}
	return base64.StdEncoding.EncodeToString(key)
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
	for {
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

			// Properly handle 8-byte length
			// First bit must be 0 (unsigned)
			if extLen[0]&0x80 != 0 {
				return nil, fmt.Errorf("invalid payload length: most significant bit must be 0")
			}

			// Calculate the 64-bit length
			payloadLen64 := uint64(0)
			for i := 0; i < 8; i++ {
				payloadLen64 = (payloadLen64 << 8) | uint64(extLen[i])
			}

			// Check if the length fits in an int
			if payloadLen64 > uint64(^uint(0)>>1) {
				return nil, fmt.Errorf("payload too large for this implementation")
			}

			payloadLen = int(payloadLen64)
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

		// Handle control frames (ping, pong, close)
		if opcode >= OpClose {
			// Control frames cannot be fragmented
			if !fin {
				return nil, fmt.Errorf("control frames cannot be fragmented")
			}

			// Return control frames immediately
			return &Message{OpCode: opcode, Payload: payload}, nil
		}

		// Handle fragmented messages
		if opcode == OpContinuation {
			// This is a continuation frame
			if c.fragmentBuffer == nil {
				return nil, fmt.Errorf("received continuation frame but no fragmented message is in progress")
			}

			// Append this fragment to the buffer
			c.fragmentBuffer = append(c.fragmentBuffer, payload...)

			if fin {
				// This is the final fragment, return the complete message
				msg := &Message{
					OpCode:  c.fragmentOpCode,
					Payload: c.fragmentBuffer,
				}

				// Clear the fragment buffer
				c.fragmentBuffer = nil

				return msg, nil
			}

			// Not the final fragment, continue reading
			continue
		} else if !fin {
			// This is the start of a fragmented message
			c.fragmentBuffer = payload
			c.fragmentOpCode = opcode

			// Continue reading the next fragment
			continue
		}

		// This is a complete, unfragmented message
		return &Message{OpCode: opcode, Payload: payload}, nil
	}
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

		// Properly encode the 8-byte length
		header = append(header,
			byte(payloadLen>>56),
			byte(payloadLen>>48),
			byte(payloadLen>>40),
			byte(payloadLen>>32),
			byte(payloadLen>>24),
			byte(payloadLen>>16),
			byte(payloadLen>>8),
			byte(payloadLen))
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

// WriteFragmentedMessage writes a large message as multiple fragments
func (c *Conn) WriteFragmentedMessage(opcode OpCode, payload []byte, fragmentSize int) error {
	if fragmentSize <= 0 {
		return fmt.Errorf("fragment size must be positive")
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.closeSent {
		return fmt.Errorf("connection closed")
	}

	totalLen := len(payload)
	if totalLen == 0 {
		// Empty message, just send a single frame
		return c.writeFrame(true, opcode, payload)
	}

	// Send the first fragment
	if err := c.writeFrame(false, opcode, payload[:fragmentSize]); err != nil {
		return err
	}

	// Send continuation fragments
	for offset := fragmentSize; offset < totalLen; offset += fragmentSize {
		end := offset + fragmentSize
		if end > totalLen {
			end = totalLen
		}

		// Last fragment?
		isFinal := (end == totalLen)

		if err := c.writeFrame(isFinal, OpContinuation, payload[offset:end]); err != nil {
			return err
		}
	}

	return nil
}

// writeFrame writes a single WebSocket frame (without locking)
func (c *Conn) writeFrame(fin bool, opcode OpCode, payload []byte) error {
	payloadLen := len(payload)

	// Create frame header
	var header []byte

	// First byte: FIN bit, RSV1-3 are 0, opcode
	finBit := byte(0)
	if fin {
		finBit = 0x80
	}
	header = append(header, finBit|byte(opcode))

	// Second byte: No mask bit (0), and payload length
	if payloadLen < 126 {
		header = append(header, byte(payloadLen))
	} else if payloadLen < 65536 {
		header = append(header, 126)
		header = append(header, byte(payloadLen>>8), byte(payloadLen))
	} else {
		header = append(header, 127)

		// Properly encode the 8-byte length
		header = append(header,
			byte(payloadLen>>56),
			byte(payloadLen>>48),
			byte(payloadLen>>40),
			byte(payloadLen>>32),
			byte(payloadLen>>24),
			byte(payloadLen>>16),
			byte(payloadLen>>8),
			byte(payloadLen))
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
	if opcode == OpClose && fin {
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

// WriteFragmentedText writes a large text message as multiple fragments
func (c *Conn) WriteFragmentedText(message string, fragmentSize int) error {
	return c.WriteFragmentedMessage(OpText, []byte(message), fragmentSize)
}

// WriteFragmentedBinary writes a large binary message as multiple fragments
func (c *Conn) WriteFragmentedBinary(data []byte, fragmentSize int) error {
	return c.WriteFragmentedMessage(OpBinary, data, fragmentSize)
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

// CloseWithCode closes the WebSocket connection with a status code and reason
func (c *Conn) CloseWithCode(statusCode uint16, reason string) error {
	// Create payload with status code and reason
	payload := make([]byte, 2+len(reason))
	payload[0] = byte(statusCode >> 8)
	payload[1] = byte(statusCode)
	copy(payload[2:], reason)

	// Send close frame if not already sent
	if !c.closeSent {
		err := c.WriteMessage(OpClose, payload)
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

// SetReadDeadline sets the read deadline for the underlying connection
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline for the underlying connection
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// SetDeadline sets both read and write deadlines for the underlying connection
func (c *Conn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

// LocalAddr returns the local network address
func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// RemoteAddr returns the remote network address
func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// IsTLS returns true if the connection is using TLS
func (c *Conn) IsTLS() bool {
	_, ok := c.conn.(*tls.Conn)
	return ok
}

// TLSConnectionState returns the TLS connection state if using TLS
func (c *Conn) TLSConnectionState() (*tls.ConnectionState, bool) {
	tlsConn, ok := c.conn.(*tls.Conn)
	if !ok {
		return nil, false
	}
	state := tlsConn.ConnectionState()
	return &state, true
}
