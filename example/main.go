package main

import (
	"fmt"
	"github.com/edgflow/lux"

	"net"
	"os"
)

// Ensures gofmt doesn't remove the "net" and "os" imports above (feel free to remove this!)
var _ = net.Listen
var _ = os.Exit

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	fmt.Println("Logs from your program will appear here!")

	// Uncomment this block to pass the first stage
	engine := lux.NewEngine()

	engine.Get("/index", func(c *lux.Context) {
		c.WriteResponse("hi")
	})

	engine.Run("0.0.0.0:4222")

}
