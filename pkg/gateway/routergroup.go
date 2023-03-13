package gateway

import (
	"fmt"
	"net/http"
)

var (
	// regEnLetter matches english letters for http method name
	//regEnLetter = regexp.MustCompile("^[A-Z]+$")

	// anyMethods for RouterGroup Any method
	anyMethods = []string{
		http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodHead, http.MethodOptions, http.MethodDelete, http.MethodConnect,
		http.MethodTrace,
	}
)

// Router defines all router handle interface includes single and group router.
type Router interface {
	Routes
	Group(string, ...HandlerFunc) *RouterGroup
}

type Routes interface {
	Use(middleware ...HandlerFunc) Routes

	Any(string, ...HandlerFunc) Routes
	GET(string, ...HandlerFunc) Routes
	POST(string, ...HandlerFunc) Routes
	DELETE(string, ...HandlerFunc) Routes
	PATCH(string, ...HandlerFunc) Routes
	PUT(string, ...HandlerFunc) Routes
	OPTIONS(string, ...HandlerFunc) Routes
	HEAD(string, ...HandlerFunc) Routes
}

type RouterGroup struct {
	handlers HandlersChain
	basePath string
	gateway  *Gateway
	root     bool
}

func (r *RouterGroup) Use(middleware ...HandlerFunc) Routes {
	r.handlers = append(r.handlers, middleware...)
	return r.returnObj()
}

// Group creates a new router group. You should add all the routes that have common middlewares or the same path prefix.
// For example, all the routes that use a common middleware for authorization could be grouped.
func (r *RouterGroup) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		handlers: r.combineHandlers(handlers),
		basePath: r.calculateAbsolutePath(relativePath),
		gateway:  r.gateway,
	}
}

// BasePath returns the base path of router group.
// For example, if v := router.Group("/rest/n/v1/api"), v.BasePath() is "/rest/n/v1/api".
func (group *RouterGroup) BasePath() string {
	return group.basePath
}

// POST is a shortcut for router.Handle("POST", path, handlers).
func (r *RouterGroup) POST(relativePath string, handlers ...HandlerFunc) Routes {
	return r.handle(http.MethodPost, relativePath, handlers)
}

// GET is a shortcut for router.Handle("GET", path, handlers).
func (r *RouterGroup) GET(relativePath string, handlers ...HandlerFunc) Routes {
	return r.handle(http.MethodGet, relativePath, handlers)
}

// DELETE is a shortcut for router.Handle("DELETE", path, handlers).
func (r *RouterGroup) DELETE(relativePath string, handlers ...HandlerFunc) Routes {
	return r.handle(http.MethodDelete, relativePath, handlers)
}

// PATCH is a shortcut for router.Handle("PATCH", path, handlers).
func (r *RouterGroup) PATCH(relativePath string, handlers ...HandlerFunc) Routes {
	return r.handle(http.MethodPatch, relativePath, handlers)
}

// PUT is a shortcut for router.Handle("PUT", path, handlers).
func (r *RouterGroup) PUT(relativePath string, handlers ...HandlerFunc) Routes {
	return r.handle(http.MethodPut, relativePath, handlers)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, handlers).
func (r *RouterGroup) OPTIONS(relativePath string, handlers ...HandlerFunc) Routes {
	return r.handle(http.MethodOptions, relativePath, handlers)
}

// HEAD is a shortcut for router.Handle("HEAD", path, handlers).
func (r *RouterGroup) HEAD(relativePath string, handlers ...HandlerFunc) Routes {
	return r.handle(http.MethodHead, relativePath, handlers)
}

// Any registers a route that matches all the HTTP methods.
// GET, POST, PUT, PATCH, HEAD, OPTIONS, DELETE, CONNECT, TRACE.
func (r *RouterGroup) Any(relativePath string, handlers ...HandlerFunc) Routes {
	for _, method := range anyMethods {
		r.handle(method, relativePath, handlers)
	}
	return r.returnObj()
}

func (r *RouterGroup) handle(httpMethod, relativePath string, handlers HandlersChain) Routes {
	absolutePath := r.calculateAbsolutePath(relativePath)
	handlers = r.combineHandlers(handlers)
	fmt.Println(absolutePath)
	r.gateway.addRoute(httpMethod, absolutePath, handlers)
	return r.returnObj()
}

func (r *RouterGroup) combineHandlers(handlers HandlersChain) HandlersChain {
	finalSize := len(r.handlers) + len(handlers)
	assert1(finalSize < int(abortIndex), "too many handlers")
	mergedHandlers := make(HandlersChain, finalSize)
	copy(mergedHandlers, r.handlers)
	copy(mergedHandlers[len(r.handlers):], handlers)
	return mergedHandlers
}

func (r *RouterGroup) calculateAbsolutePath(relativePath string) string {
	fmt.Printf("basePath %s relativePath: %s\n", r.basePath, relativePath)
	return joinPaths(r.basePath, relativePath)
}

func (r *RouterGroup) returnObj() Routes {
	if r.root {
		return r.gateway
	}
	return r
}
