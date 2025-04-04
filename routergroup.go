package lux

import (
	"net/http"
	"path"
)

var (
	// anyMethods for RouterGroup Any method
	anyMethods = []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodConnect,
		http.MethodTrace,
	}
)

type RouterGroup struct {
	Handlers HandlerChain
	BasePath string
	engine   *Engine
	root     bool
}

type IRoutes interface {
	Use(...HandlerFunc) IRoutes
	Any(string, ...HandlerFunc) IRoutes
	Get(string, ...HandlerFunc) IRoutes
	Post(string, ...HandlerFunc) IRoutes
	Delete(string, ...HandlerFunc) IRoutes
	Patch(string, ...HandlerFunc) IRoutes
	Put(string, ...HandlerFunc) IRoutes
	OPTIONS(string, ...HandlerFunc) IRoutes
	HEAD(string, ...HandlerFunc) IRoutes
	Match([]string, string, ...HandlerFunc) IRoutes
}
type IRouter interface {
	IRoutes
	Group(string, ...HandlerFunc) *RouterGroup
}

func (r *RouterGroup) Use(handlerFunc ...HandlerFunc) IRoutes {
	r.Handlers = append(r.Handlers, handlerFunc...)
	return r.returnObj()
}

func (r *RouterGroup) Any(relPath string, handlers ...HandlerFunc) IRoutes {
	for _, method := range anyMethods {
		r.handle(method, relPath, handlers)
	}
	return r.returnObj()
}

func (r *RouterGroup) Get(relativePath string, handlers ...HandlerFunc) IRoutes {
	return r.handle(http.MethodGet, relativePath, handlers)
}

func (r *RouterGroup) Post(relativePath string, handlers ...HandlerFunc) IRoutes {
	return r.handle(http.MethodPost, relativePath, handlers)

}

func (r *RouterGroup) Delete(relativePath string, handlers ...HandlerFunc) IRoutes {
	return r.handle(http.MethodDelete, relativePath, handlers)
}

func (r *RouterGroup) Patch(relativePath string, handlers ...HandlerFunc) IRoutes {
	return r.handle(http.MethodPatch, relativePath, handlers)
}

func (r *RouterGroup) Put(relativePath string, handlers ...HandlerFunc) IRoutes {
	return r.handle(http.MethodPut, relativePath, handlers)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, handlers).
func (group *RouterGroup) OPTIONS(relativePath string, handlers ...HandlerFunc) IRoutes {
	return group.handle(http.MethodOptions, relativePath, handlers)
}

// HEAD is a shortcut for router.Handle("HEAD", path, handlers).
func (group *RouterGroup) HEAD(relativePath string, handlers ...HandlerFunc) IRoutes {
	return group.handle(http.MethodHead, relativePath, handlers)
}

// Match registers a route that matches the specified methods that you declared.
func (group *RouterGroup) Match(methods []string, relativePath string, handlers ...HandlerFunc) IRoutes {
	for _, method := range methods {
		group.handle(method, relativePath, handlers)
	}

	return group.returnObj()
}

func (r *RouterGroup) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		Handlers: r.combineHandlers(handlers),
		BasePath: r.calculateAbseloutPath(relativePath),
		engine:   r.engine,
	}
}
func (r *RouterGroup) returnObj() IRoutes {
	if r.root {
		return r.engine
	}
	return r
}

func (r *RouterGroup) combineHandlers(handlers []HandlerFunc) HandlerChain {
	finalSize := len(r.Handlers) + len(handlers)
	mergeHandelers := make(HandlerChain, finalSize)
	copy(mergeHandelers, r.Handlers)
	copy(mergeHandelers[len(r.Handlers):], handlers)
	return mergeHandelers
}

func (r *RouterGroup) calculateAbseloutPath(path string) string {
	return joinsPath(r.BasePath, path)
}

func (r *RouterGroup) handle(httpMethod string, relPath string, handlers []HandlerFunc) IRoutes {
	abseloutPaht := r.calculateAbseloutPath(relPath)
	handlers = r.combineHandlers(handlers)
	r.engine.addRoute(httpMethod, abseloutPaht, handlers)
	return r.returnObj()
}

func joinsPath(absolutePath string, relativePath string) string {
	if absolutePath == "" {
		return relativePath
	}
	return path.Join(absolutePath, relativePath)
}

var _ IRouter = (*RouterGroup)(nil)
