package main

import (
	"fmt"
	websocket "github.com/edgflow/lux/ws"
	"log"
	"time"
)

func main() {
	// Example server
	go runServer()

	// Wait for server to start
	time.Sleep(time.Second)

	// Example client
	runClient()
}

func runServer() {
	// Create a new WebSocket server
	server := websocket.NewServer(":8080", handleWebSocket)

	fmt.Println("WebSocket server running on port 8080")

	// Start the server
	log.Fatal(server.ListenAndServe())

	// For TLS:
	// server := websocket.NewTLSServer(":8443", handleWebSocket, &tls.Config{})
	// log.Fatal(server.ListenAndServeTLS("cert.pem", "key.pem"))
}

func handleWebSocket(conn *websocket.Conn) {
	defer conn.Close()

	fmt.Println("Client connected")

	// Send a welcome message
	err := conn.WriteText("Welcome to the WebSocket server!")
	if err != nil {
		fmt.Println("Error sending welcome message:", err)
		return
	}

	// Echo server implementation with fragmented message support
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message:", err)
			return
		}

		// Handle different message types
		switch msg.OpCode {
		case websocket.OpText:
			fmt.Println("Received text message:", string(msg.Payload))

			// Echo the message back
			// For large messages, use fragmented sending
			if len(msg.Payload) > 1024 {
				err = conn.WriteFragmentedText(string(msg.Payload), 1024)
			} else {
				err = conn.WriteText(string(msg.Payload))
			}

			if err != nil {
				fmt.Println("Error writing message:", err)
				return
			}

		case websocket.OpBinary:
			fmt.Println("Received binary message, length:", len(msg.Payload))

			// Echo the message back
			// For large messages, use fragmented sending
			if len(msg.Payload) > 1024 {
				err = conn.WriteFragmentedBinary(msg.Payload, 1024)
			} else {
				err = conn.WriteBinary(msg.Payload)
			}

			if err != nil {
				fmt.Println("Error writing message:", err)
				return
			}

		case websocket.OpPing:
			fmt.Println("Received ping")

			// Respond with pong
			err = conn.Pong(msg.Payload)
			if err != nil {
				fmt.Println("Error sending pong:", err)
				return
			}

		case websocket.OpClose:
			fmt.Println("Received close frame")
			return

		default:
			fmt.Printf("Received message with opcode: %d\n", msg.OpCode)
		}
	}
}

func runClient() {
	// Connect to the WebSocket server
	conn, err := websocket.Dial("ws://localhost:8080")
	if err != nil {
		log.Fatal("Error connecting to WebSocket server:", err)
	}
	defer conn.Close()

	// For TLS:
	// conn, err := websocket.Dial("wss://localhost:8443")

	fmt.Println("Connected to WebSocket server")

	// Read the welcome message
	msg, err := conn.ReadMessage()
	if err != nil {
		log.Fatal("Error reading welcome message:", err)
	}

	if msg.OpCode == websocket.OpText {
		fmt.Println("Server says:", string(msg.Payload))
	}

	// Send a message
	err = conn.WriteText("Hello from the client!")
	if err != nil {
		log.Fatal("Error sending message:", err)
	}

	// Read the echo response
	msg, err = conn.ReadMessage()
	if err != nil {
		log.Fatal("Error reading response:", err)
	}

	if msg.OpCode == websocket.OpText {
		fmt.Println("Server echoed:", string(msg.Payload))
	}

	// Send a large message that will be fragmented
	largeMessage := "This is a large message that will be sent as multiple fragments. " +
		"The server will echo it back also as multiple fragments. " +
		"Fragmentation is useful for sending large messages without buffering the entire message in memory."

	// Repeat to make it larger
	for i := 0; i < 10; i++ {
		largeMessage += largeMessage
	}

	fmt.Println("Sending large message, length:", len(largeMessage))

	err = conn.WriteFragmentedText(largeMessage, 1024)
	if err != nil {
		log.Fatal("Error sending fragmented message:", err)
	}

	// Read the fragmented echo response
	msg, err = conn.ReadMessage()
	if err != nil {
		log.Fatal("Error reading fragmented response:", err)
	}

	if msg.OpCode == websocket.OpText {
		fmt.Println("Server echoed large message, length:", len(msg.Payload))

		// Verify it's the same
		if string(msg.Payload) == largeMessage {
			fmt.Println("Large message echoed correctly")
		} else {
			fmt.Println("Large message echo mismatch")
		}
	}

	// Close with a status code and reason
	fmt.Println("Closing connection")
	conn.CloseWithCode(1000, "Normal closure")
}
