package main

import (
	"edgflow/lux/ws"
	"fmt"
	"log"
)

func main() {

	s := ws.NewServer("localhost:2222", handleWebSocket)

	log.Fatal(s.ListenAndServe())
}
func handleWebSocket(conn *ws.Conn) {
	defer conn.Close()

	fmt.Println("Client connected")

	// Echo server implementation
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message:", err)
			return
		}

		// Handle different message types
		switch msg.OpCode {
		case ws.OpText:
			fmt.Println("Received text message:", string(msg.Payload))

			// Echo the message back
			err = conn.WriteText(string(msg.Payload))
			if err != nil {
				fmt.Println("Error writing message:", err)
				return
			}

		case ws.OpBinary:
			fmt.Println("Received binary message, length:", len(msg.Payload))

			// Echo the message back
			err = conn.WriteBinary(msg.Payload)
			if err != nil {
				fmt.Println("Error writing message:", err)
				return
			}

		case ws.OpPing:
			fmt.Println("Received ping")

			// Respond with pong
			err = conn.Pong(msg.Payload)
			if err != nil {
				fmt.Println("Error sending pong:", err)
				return
			}

		case ws.OpClose:
			fmt.Println("Received close frame")
			return

		default:
			fmt.Printf("Received message with opcode: %d\n", msg.OpCode)
		}
	}
}
