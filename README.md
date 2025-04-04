# Lux

Lux is a lightweight HTTP web framework inspired by Gin, implemented in Go for testing and educational purposes. It provides a minimalist approach to handling HTTP requests while maintaining performance and usability.

## Overview

Lux is a simplified implementation of the core features found in the popular Gin framework. This project was created for testing and learning purposes to understand the internals of Go HTTP frameworks.

## Features

- Router with support for HTTP methods (GET, POST, PUT, DELETE, etc.)
- Middleware support with `Next()` function for passing control
- Context object for request/response handling
- Custom ResponseWriter implementation
- Raw TCP/IP connection handling
- Support for route parameters
- Simple API similar to Gin

## Installation

```bash
go get github.com/edgflow/lux
```

## Quick Start

```go
package main

import (
    "github.com/edgflow/lux"
    "net/http"
)

func main() {
    // Create a new Lux engine
    r := lux.New()
    
    // Define a middleware
    r.Use(lux.Logger())
    
    // Define routes
    r.GET("/ping", func(c *lux.Context) {
        c.JSON(http.StatusOK, lux.H{
            "message": "pong",
        })
    })
    
    r.GET("/user/:name", func(c *lux.Context) {
        name := c.Param("name")
        c.String(http.StatusOK, "Hello %s", name)
    })
    
    // Start server
    r.Run(":8080")
}
```

## Key Components

### Engine

The core of the framework that manages routing and middleware.

### Context

Encapsulates HTTP request and response handling, providing a clean API for handlers.

### ResponseWriter

Custom implementation of `http.ResponseWriter` that works directly with TCP connections.

## Implementation Details

### Raw TCP/IP Connection Handling

Lux includes a custom ResponseWriter implementation that can work directly with `net.Conn` objects:

```go
func (e *Engine) handleConn(conn net.Conn) {
    defer conn.Close()
    conn.SetReadDeadline(time.Now().Add(30 * time.Second))
    conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

    reader := bufio.NewReader(conn)
    req, err := http.ReadRequest(reader)
    if err != nil {
        // Handle error
        return
    }

    // Create a response writer using the connection
    writer := NewResponseWriter(conn, req)
    
    ctx := Context{
        Request:        req,
        responseWriter: writer,
        index:          -1,
    }

    e.handleHttpRequest(&ctx)
}
```

### Router Implementation

Simple but effective router for handling different HTTP methods:

```go
func (e *Engine) addRoute(method, path string, handlers ...HandlerFunc) {
    // Implementation details
}

func (e *Engine) GET(path string, handlers ...HandlerFunc) {
    e.addRoute("GET", path, handlers...)
}

func (e *Engine) POST(path string, handlers ...HandlerFunc) {
    e.addRoute("POST", path, handlers...)
}
```

## Differences from Gin

- Simplified implementation without all Gin features
- Focused on core functionality for learning purposes
- Custom ResponseWriter that works with raw connections
- Minimalist approach to middleware

## License

MIT License

## Disclaimer

This project is for educational and testing purposes only. It is not intended for production use.

## Contributing

Feel free to contribute by opening issues or submitting pull requests.