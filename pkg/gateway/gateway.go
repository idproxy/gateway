package gateway

import (
	"fmt"
	"net/http"
	"path"
	"regexp"
	"sync"

	"github.com/idproxy/gateway/internal/bytesconv"
	"github.com/idproxy/gateway/pkg/binding"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var (
	default404Body = []byte("404 page not found")
	//default405Body = []byte("405 method not allowed")
)

var regSafePrefix = regexp.MustCompile("[^a-zA-Z0-9/-]+")
var regRemoveRepeatedChar = regexp.MustCompile("/{2,}")

type Gateway struct {
	RouterGroup

	// RedirectTrailingSlash enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 307 for all other request methods.
	RedirectTrailingSlash bool

	// RedirectFixedPath if enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterwards the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 307 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool

	// UseRawPath if enabled, the url.RawPath will be used to find parameters.
	UseRawPath bool

	// UnescapePathValues if true, the path value will be unescaped.
	// If UseRawPath is false (by default), the UnescapePathValues effectively is true,
	// as url.Path gonna be used, which is already unescaped.
	UnescapePathValues bool

	// RemoveExtraSlash a parameter can be parsed from the URL even with extra slashes.
	// See the PR #1817 and issue #1644
	RemoveExtraSlash bool

	// UseH2C enable h2c support.
	useH2C bool

	allNoRoute  HandlersChain
	allNoMethod HandlersChain
	noRoute     HandlersChain
	noMethod    HandlersChain
	pool        sync.Pool
	trees       methodTrees
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
		RedirectTrailingSlash: true,
		RedirectFixedPath:     false,
		trees:                 make(methodTrees, 0, 9),
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
	for _, tree := range r.trees {
		fmt.Println(tree.method)
		if tree.root != nil {
			tree.root.PrintNode()
		}
	}
}

func (r *Gateway) Handler() http.Handler {
	if !r.useH2C {
		return r
	}
	h2s := &http2.Server{}
	return h2c.NewHandler(r, h2s)
}

func (r *Gateway) allocateContext(maxParams uint16) *Context {
	v := make(Params, 0, maxParams)
	skippedNodes := make([]skippedNode, 0, r.maxSections)
	return &Context{gateway: r, params: &v, skippedNodes: &skippedNodes}
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
func (r *Gateway) Run(addr ...string) (err error) {
	address := resolveAddress(addr)
	debugPrint("Listening and serving HTTP on %s\n", address)
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

	if r.RemoveExtraSlash {
		rPath = cleanPath(rPath)
	}
	fmt.Println(rPath)

	// Find root of the tree for the given HTTP method
	t := r.trees
	for i, tl := 0, len(t); i < tl; i++ {
		if t[i].method != httpMethod {
			continue
		}

		root := t[i].root
		// Find route in tree
		value := root.getValue(rPath, gctx.params, gctx.skippedNodes, unescape)
		if value.params != nil {
			gctx.Params = *value.params
		}
		fmt.Printf("value: %v\n", value)
		if value.params != nil {
			fmt.Printf("value params: %v\n", value.params)
		}
		if value.handlers != nil {
			gctx.handlers = value.handlers
			gctx.fullPath = value.fullPath
			gctx.Next()
			gctx.writermem.WriteHeaderNow()
			return
		}
		if httpMethod != http.MethodConnect && rPath != "/" {
			if value.tsr && r.RedirectTrailingSlash {
				redirectTrailingSlash(gctx)
				return
			}
			if r.RedirectFixedPath && redirectFixedPath(gctx, root, r.RedirectFixedPath) {
				return
			}
		}
		break

	}
	gctx.handlers = r.allNoRoute
	serveError(gctx, http.StatusNotFound, default404Body)
}

func (r *Gateway) rebuild404Handlers() {
	r.allNoRoute = r.combineHandlers(r.noRoute)
}

func (r *Gateway) rebuild405Handlers() {
	r.allNoMethod = r.combineHandlers(r.noMethod)
}

func (r *Gateway) addRoute(method, path string, handlers HandlersChain) {
	assert1(path[0] == '/', "path must begin with '/'")
	assert1(method != "", "HTTP method can not be empty")
	assert1(len(handlers) > 0, "there must be at least one handler")

	debugPrintRoute(method, path, handlers)
	fmt.Println(method, path)
	root := r.trees.get(method)
	fmt.Printf("root: %#v, method: %s, path: %s", root, method, path)
	if root == nil {
		root = new(node)
		root.fullPath = "/"
		r.trees = append(r.trees, methodTree{method: method, root: root})
	}
	root.addRoute(path, handlers)

	// Update maxParams
	if paramsCount := countParams(path); paramsCount > r.maxParams {
		r.maxParams = paramsCount
	}

	if sectionsCount := countSections(path); sectionsCount > r.maxSections {
		r.maxSections = sectionsCount
	}
}

var mimePlain = []string{binding.MIMEPlain}

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

func redirectTrailingSlash(c *Context) {
	req := c.Request
	p := req.URL.Path
	if prefix := path.Clean(c.Request.Header.Get("X-Forwarded-Prefix")); prefix != "." {
		prefix = regSafePrefix.ReplaceAllString(prefix, "")
		prefix = regRemoveRepeatedChar.ReplaceAllString(prefix, "/")

		p = prefix + "/" + req.URL.Path
	}
	req.URL.Path = p + "/"
	if length := len(p); length > 1 && p[length-1] == '/' {
		req.URL.Path = p[:length-1]
	}
	redirectRequest(c)
}

func redirectFixedPath(c *Context, root *node, trailingSlash bool) bool {
	req := c.Request
	rPath := req.URL.Path

	if fixedPath, ok := root.findCaseInsensitivePath(cleanPath(rPath), trailingSlash); ok {
		req.URL.Path = bytesconv.BytesToString(fixedPath)
		redirectRequest(c)
		return true
	}
	return false
}

func redirectRequest(c *Context) {
	req := c.Request
	rPath := req.URL.Path
	rURL := req.URL.String()

	code := http.StatusMovedPermanently // Permanent redirect, request with GET method
	if req.Method != http.MethodGet {
		code = http.StatusTemporaryRedirect
	}
	debugPrint("redirecting request %d: %s --> %s", code, rPath, rURL)
	http.Redirect(c.Writer, req, rURL, code)
	c.writermem.WriteHeaderNow()
}
