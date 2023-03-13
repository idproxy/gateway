package gateway2

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/idproxy/gateway/pkg/binding"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	default404Body = []byte("404 page not found")
	//default405Body = []byte("405 method not allowed")
)

var mimePlain = []string{binding.MIMEPlain}

type Gateway struct {
	RouterGroup

	// UseRawPath if enabled, the url.RawPath will be used to find parameters.
	UseRawPath bool

	// UnescapePathValues if true, the path value will be unescaped.
	// If UseRawPath is false (by default), the UnescapePathValues effectively is true,
	// as url.Path gonna be used, which is already unescaped.
	UnescapePathValues bool

	// UseH2C enable h2c support.
	useH2C bool

	allNoRoute  HandlersChain
	allNoMethod HandlersChain
	noRoute     HandlersChain
	noMethod    HandlersChain
	pool        sync.Pool
	tree        Tree
	maxParams   uint16
	maxSections uint16
}

func New() *Gateway {
	r := &Gateway{
		RouterGroup: RouterGroup{
			handlers: nil,
			basePath: "/",
			root:     true,
		},
		tree:      NewTree(),
		maxParams: 8,
	}
	r.RouterGroup.gateway = r
	r.pool.New = func() any {
		return r.allocateContext(r.maxParams)
	}
	return r
}

func Default() *Gateway {
	g := New()
	g.Use(Logger(), Recovery())
	return g
}

func (r *Gateway) PrintMethodTree() {
	r.tree.Print()
}

func (r *Gateway) allocateContext(maxParams uint16) *Context {
	v := make(Params, 0, maxParams)
	//skippedNodes := make([]skippedNode, 0, r.maxSections)
	return &Context{gateway: r, params: &v} //skippedNodes: &skippedNodes}
}

func (r *Gateway) Handler() http.Handler {
	if !r.useH2C {
		return r
	}
	h2s := &http2.Server{}
	return h2c.NewHandler(r, h2s)
}

func (r *Gateway) Use(middleware ...HandlerFunc) Routes {
	r.RouterGroup.Use(middleware...)
	r.rebuild404Handlers()
	r.rebuild405Handlers()
	return r
}

// Run attaches the router to a http.Server and starts listening and serving HTTP requests.
// It is a shortcut for http.ListenAndServe(addr, router)
// Note: this method will block the calling goroutine indefinitely unless an error happens.
func (r *Gateway) Run(address string) (err error) {
	err = http.ListenAndServe(address, r.Handler())
	return
}

// ServeHTTP conforms to the http.Handler interface.
func (r *Gateway) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	gctx := r.pool.Get().(*Context)
	gctx.writermem.reset(w)
	gctx.Request = req
	fmt.Printf("Params: %v\n", gctx.Params)
	gctx.reset()

	r.handleHTTPRequest(gctx)

	r.pool.Put(gctx)
}

func (r *Gateway) handleHTTPRequest(gctx *Context) {
	httpMethod := gctx.Request.Method
	rPath := gctx.Request.URL.Path
	unescape := false
	if r.UseRawPath && len(gctx.Request.URL.RawPath) > 0 {
		rPath = gctx.Request.URL.RawPath
		unescape = r.UnescapePathValues
	}

	valueCtx := r.tree.GetReqValue(&requestContext{
		httpMethod:   httpMethod,
		path:         rPath,
		params:       gctx.params,
		//skippedNodes: gctx.skippedNodes,
		unescape:     unescape,
	})

	if valueCtx.params != nil {
		gctx.Params = *valueCtx.params
		fmt.Printf("valueCtx: %v\n", *valueCtx.params)
	}
	if valueCtx.handlers != nil {
		gctx.handlers = valueCtx.handlers
		//gctx.fullPath = valueCtx.fullPath
		gctx.Next()
		gctx.writermem.WriteHeaderNow()
		return
	}

	/*
		if httpMethod != http.MethodConnect && rPath != "/" {
			if valueCtx.tsr && r.RedirectTrailingSlash {
				redirectTrailingSlash(gctx)
				return
			}
			if r.RedirectFixedPath && redirectFixedPath(gctx, root, r.RedirectFixedPath) {
				return
			}
		}
	*/

	gctx.handlers = r.allNoRoute
	serveError(gctx, http.StatusNotFound, default404Body)
}

func serveError(gctx *Context, code int, defaultMessage []byte) {
	gctx.writermem.status = code
	gctx.Next()
	if gctx.writermem.Written() {
		return
	}
	if gctx.writermem.Status() == code {
		gctx.writermem.Header()["Content-Type"] = mimePlain
		_, err := gctx.Writer.Write(defaultMessage)
		if err != nil {
			debugPrint("cannot write message to writer during serve error: %v", err)
		}
		return
	}
	gctx.writermem.WriteHeaderNow()
}

func (r *Gateway) rebuild404Handlers() {
	r.allNoRoute = r.combineHandlers(r.noRoute)
}

func (r *Gateway) rebuild405Handlers() {
	r.allNoMethod = r.combineHandlers(r.noMethod)
}
