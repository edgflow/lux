package lux

import (
	"errors"
	"math"
	"net/http"
	"net/url"
	"sync"
)

const abortIndex int8 = math.MaxInt8 >> 1

type Context struct {
	writermem responseWriter
	Request   *http.Request
	Writer    ResponseWriter

	handlers     HandlerChain
	Params       Params
	index        int8
	fullPath     string
	engine       *Engine
	params       *Params
	skippedNodes *[]skippedNode
	mu           sync.RWMutex

	Keys       map[string]any
	queryCache url.Values
	formCache  url.Values
}

func (c *Context) reset() {
	c.Writer = &c.writermem
	c.Params = c.Params[:0]
	c.handlers = nil
	c.index = -1

	c.fullPath = ""
	c.Keys = nil
	c.queryCache = nil
	c.formCache = nil
	c.params = nil
}

func (c *Context) Next() {
	c.index++
	for c.index < int8(len(c.handlers)) {
		h := c.handlers[c.index]
		if h != nil {
			h(c)
		}
		c.index++
	}
}

func (c *Context) Param(key string) string {
	return c.Params.ByName(key)
}

// AddParam adds param to context and
// replaces path param key with given value for e2e testing purposes
// Example Route: "/user/:id"
// AddParam("id", 1)
// Result: "/user/1"
func (c *Context) AddParam(key, value string) {
	c.Params = append(c.Params, Param{Key: key, Value: value})
}

func (c *Context) WriteResponse(s string) {
	c.writermem.Write([]byte(s))
}

func (c *Context) WriteNotFound() {

}

// IsAborted returns true if the current context was aborted.
func (c *Context) IsAborted() bool {
	return c.index >= abortIndex
}

func (c *Context) Abort() {
	c.index = abortIndex
}

func (c *Context) Handler() HandlerFunc {
	return c.handlers.Last()
}

func (c *Context) FullPath() string { return c.fullPath }

func (c *Context) Set(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Keys == nil {
		c.Keys = make(map[string]any)
	}
	c.Keys[key] = value
}

func (c *Context) Get(key string) (value any, exists bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists = c.Keys[key]
	return value, exists
}

func (c *Context) GetString(key string) (s string) {
	return getTyped[string](c, key)
}

func getTyped[T any](c *Context, key string) (res T) {
	if val, ok := c.Get(key); ok && val != nil {
		res, _ = val.(T)
	}
	return
}

func (c *Context) Query(key string) (value string) {
	value, _ = c.GetQuery(key)
	return
}

func (c *Context) initQueryCache() {
	if c.queryCache == nil {
		if c.Request != nil && c.Request.URL != nil {
			c.queryCache = c.Request.URL.Query()
		} else {
			c.queryCache = url.Values{}
		}
	}
}

func (c *Context) GetQuery(key string) (string, interface{}) {
	if values, ok := c.GetQueryArray(key); ok {
		return values[0], ok
	}
	return "", false
}

func (c *Context) GetQueryArray(key string) (values []string, ok bool) {
	c.initQueryCache()
	values, ok = c.queryCache[key]
	return
}

// PostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns an empty string `("")`.
func (c *Context) PostForm(key string) (value string) {
	value, _ = c.GetPostForm(key)
	return
}

// DefaultPostForm returns the specified key from a POST urlencoded form or multipart form
// when it exists, otherwise it returns the specified defaultValue string.
// See: PostForm() and GetPostForm() for further information.
func (c *Context) DefaultPostForm(key, defaultValue string) string {
	if value, ok := c.GetPostForm(key); ok {
		return value
	}
	return defaultValue
}

// GetPostForm is like PostForm(key). It returns the specified key from a POST urlencoded
// form or multipart form when it exists `(value, true)` (even when the value is an empty string),
// otherwise it returns ("", false).
// For example, during a PATCH request to update the user's email:
//
//	    email=mail@example.com  -->  ("mail@example.com", true) := GetPostForm("email") // set email to "mail@example.com"
//		   email=                  -->  ("", true) := GetPostForm("email") // set email to ""
//	                            -->  ("", false) := GetPostForm("email") // do nothing with email
func (c *Context) GetPostForm(key string) (string, bool) {
	if values, ok := c.GetPostFormArray(key); ok {
		return values[0], ok
	}
	return "", false
}

// PostFormArray returns a slice of strings for a given form key.
// The length of the slice depends on the number of params with the given key.
func (c *Context) PostFormArray(key string) (values []string) {
	values, _ = c.GetPostFormArray(key)
	return
}

func (c *Context) initFormCache() {
	if c.formCache == nil {
		c.formCache = make(url.Values)
		req := c.Request
		if err := req.ParseMultipartForm(c.engine.MaxMultipartMemory); err != nil {
			if !errors.Is(err, http.ErrNotMultipart) {
				debugPrint("error on parse multipart form array: %v", err)
			}
		}
		c.formCache = req.PostForm
	}
}

// GetPostFormArray returns a slice of strings for a given form key, plus
// a boolean value whether at least one value exists for the given key.
func (c *Context) GetPostFormArray(key string) (values []string, ok bool) {
	c.initFormCache()
	values, ok = c.formCache[key]
	return
}
