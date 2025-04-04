package lux

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type Engine struct {
	RouterGroup
	pool               sync.Pool
	trees              methodTrees
	MaxMultipartMemory int64
	maxParams          uint16
	maxSections        uint16
}

func NewEngine() *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{
			Handlers: nil,
			BasePath: "/",
			root:     true,
		},
		trees: make(methodTrees, 0, 9),
	}
	engine.pool.New = func() any {
		return engine.allocateContext(engine.maxParams)
	}
	engine.RouterGroup.engine = engine
	return engine
}

func (engine *Engine) allocateContext(maxParams uint16) *Context {
	v := make(Params, 0, maxParams)
	skippedNodes := make([]skippedNode, 0, engine.maxSections)
	return &Context{engine: engine, params: &v, skippedNodes: &skippedNodes}
}

func (e *Engine) addRoute(method string, path string, handlers []HandlerFunc) {
	root := e.trees.get(method)
	if root == nil {
		root = new(Node)
		root.Path = "/"
		e.trees = append(e.trees, NodeTree{
			Method: method,
			Root:   root,
		})
	}
	root.addRoute(path, handlers)
}

func (e *Engine) Routes() (routes RoutesInfo) {
	for _, tree := range e.trees {
		routes = iterate("", tree.Method, routes, tree.Root)
	}
	return routes
}

func iterate(path, method string, routes RoutesInfo, root *Node) RoutesInfo {
	path += root.Path
	if len(root.Handlers) > 0 {
		handlerFunc := root.Handlers.Last()
		routes = append(routes, RouteInfo{
			Method:      method,
			Path:        path,
			Handler:     nameOfFunction(handlerFunc),
			HandlerFunc: handlerFunc,
		})
	}
	for _, child := range root.Children {
		routes = iterate(path, method, routes, child)
	}
	return routes
}

func (e *Engine) Run(add string) (err error) {
	l, err := net.Listen("tcp", add)
	if err != nil {
		fmt.Println("Faild to bind address", add)
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Filed to bind port 4221")
			os.Exit(1)
		}
		go e.handleConn(conn)
	}
}

// Use in your handleConn function
func (e *Engine) handleConn(conn net.Conn) {
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	reader := bufio.NewReader(conn)

	req, err := http.ReadRequest(reader)
	if err != nil {
		if err != io.EOF {
			fmt.Println("error read Request ", err)
		}
		return
	}

	// Create a response writer using the connection
	writer := NewResponseWriter(conn, req)

	ctx := e.pool.Get().(*Context)
	ctx.writermem.reset(writer, conn)
	ctx.Request = req
	ctx.reset()
	e.handleHttpRequest(ctx)
	e.pool.Put(ctx)
}
func (e *Engine) handleHttpRequest(c *Context) {
	httpMehod := c.Request.Method
	rPath := c.Request.URL.Path
	t := e.trees

	//find root of tree
	for i, tl := 0, len(t); i < tl; i++ {
		if t[i].Method != httpMehod {
			continue
		}
		//root:=t[i].Root
		handler, params := t[i].Find(rPath)
		if handler != nil {
			c.handlers = handler
			c.Params = params
			c.Next()
			return
		}
	}

	c.Abort()
}
