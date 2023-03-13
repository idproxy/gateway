package gateway2

import (
	"fmt"
	"net/http"
	"path"
	"sync"
)

const (
	wildcard = "wildcard"
)

type Tree interface {
	Print()
	AddRoute(httpMethod, absolutePath string, handlers HandlersChain)
	GetSupportedmethods() []string
	GetReqValue(reqCtx *requestContext) valueContext
}

type methodTree struct {
	m                sync.RWMutex
	tree             map[string]*node
	supportedMethods map[string]struct{}
}

type nodeKind uint32

const (
	normal nodeKind = iota
	root
	param
	catchAll
	query
)

func (r nodeKind) String() string {
	switch r {
	case normal:
		return "normal"
	case root:
		return "root"
	case param:
		return "param"
	case catchAll:
		return "catchAll"
	case query:
		return "query"
	default:
		return "unknown"
	}
}

type node struct {
	pathSegment
	m        sync.RWMutex
	children map[string]*node
	handlers HandlersChain
}

type pathSegment struct {
	value string
	kind  nodeKind
}

func NewTree() Tree {
	r := &methodTree{
		supportedMethods: map[string]struct{}{
			http.MethodGet:     {},
			http.MethodPost:    {},
			http.MethodPut:     {},
			http.MethodPatch:   {},
			http.MethodHead:    {},
			http.MethodOptions: {},
			http.MethodDelete:  {},
			http.MethodConnect: {},
			http.MethodTrace:   {},
		},
		tree: map[string]*node{},
	}
	for method := range r.supportedMethods {
		r.tree[method] = &node{
			pathSegment: pathSegment{value: "/", kind: root},
			children:    map[string]*node{},
		}
	}
	return r
}

func (r *methodTree) Print() {
	r.m.RLock()
	defer r.m.RUnlock()
	for method, n := range r.tree {
		if method == "GET" {
			fmt.Println(method)
			//fmt.Printf("pathSegment tree: %s, handlers: %d\n", n.pathSegment.value, len(n.handlers))
			n.Print(0)
		}
	}
}

func (r *node) Print(i int) {
	r.m.RLock()
	defer r.m.RUnlock()
	fmt.Printf("%*s pathSegment: %s, kind: %s, handlers: %d\n", i, "", r.pathSegment.value, r.kind.String(), len(r.handlers))
	//fmt.Printf("children: %d\n", len(r.children))
	for _, n := range r.children {
		//fmt.Printf("pathSegment node: %s, handlers: %d\n", ps, len(n.handlers))
		n.Print(i + 1)
	}
}

func (r *methodTree) GetSupportedmethods() []string {
	supportedMethods := make([]string, 0, len(r.supportedMethods))
	for method := range r.supportedMethods {
		supportedMethods = append(supportedMethods, method)
	}
	return supportedMethods
}

func (r *methodTree) AddRoute(httpMethod, absolutePath string, handlers HandlersChain) {
	r.m.Lock()
	defer r.m.Unlock()
	assert1(absolutePath[0] == '/', "path must begin with '/'")
	assert1(httpMethod != "", "HTTP method can not be empty")
	assert1(len(handlers) > 0, "there must be at least one handler")
	rn, ok := r.tree[httpMethod]
	assert1(ok, fmt.Sprintf("method: %s not matching supported methods: %v", httpMethod, r.GetSupportedmethods()))

	pathSegments, valid := getPathSegments(absolutePath)
	if !valid {
		assert1(false, fmt.Sprintf("multiple wildcards found in pathSegment: %s", absolutePath))
	}
	fmt.Printf("pathSegments: %v\n", pathSegments)
	// this is a path with only "/"
	if len(pathSegments) == 1 {
		rn.handlers = handlers
		return
	}
	rn.addroute(1, pathSegments, handlers)
}

func (r *node) addroute(idx int, pathSegments []pathSegment, handlers HandlersChain) {
	ps := pathSegments[idx]
	psValue := ps.value
	if ps.kind == param || ps.kind == catchAll {
		psValue = wildcard
	}
	n, ok := r.children[psValue]
	if !ok {
		n = &node{
			pathSegment: ps,
			children:    map[string]*node{},
		}
		r.children[psValue] = n
	}
	if ok && psValue == wildcard {
		assert1(ok, fmt.Sprintf("got wildcard: %s, but another wildcard was already in place: %s", ps.value, n.pathSegment.value))
	}

	//n.addRoute(idx, pathSegments, handlers)

	fmt.Printf("addRoute: pathSegments: %v, idx: %d ps length: %d\n", pathSegments, idx, len(pathSegments)-1)
	if idx == len(pathSegments)-1 {
		fmt.Printf("addRoute: addHandlers %d\n", len(handlers))
		// for the last path segment add the handlers and return
		n.handlers = handlers
		return
	}
	n.addroute(idx+1, pathSegments, handlers)
}

/*
func (r *node) addRoute(idx int, pathSegments []pathSegment, handlers HandlersChain) {
	r.m.Lock()
	defer r.m.Unlock()

	fmt.Printf("addRoute: pathSegments: %v, idx: %d ps length: %d\n", pathSegments, idx, len(pathSegments)-1)
	if idx == len(pathSegments)-1 {
		fmt.Printf("addRoute: addHandlers %d\n", len(handlers))
		// for the last path segment add the handlers and return
		r.handlers = handlers
		return
	}
	// initialize the children
	r.addroute(idx+1, pathSegments, handlers)
}
*/

func getPathSegments(path string) (ps []pathSegment, valid bool) {
	fmt.Printf("getPathSegments -> path: %s\n", path)
	ps = make([]pathSegment, 0)
	//we always start with a "/" pathSegment
	ps = append(ps, pathSegment{
		value: string([]byte(path)[0]),
		kind:  root,
	})
	valid = true
	begin := 0
	kind := normal
	lastPathSegment := false
	// when there is a single "/" we dont need to find the pathSegments as it is "/"
	for idx, c := range []byte(path) {
		// ignore the first char as it is always "/"
		if idx > 0 {
			// if char is a / or we are done we append the pathSegment to the slice
			if c == '/' || idx == len(path)-1 {
				end := idx
				if idx == len(path)-1 { // if we are at the end we include the last char
					if c == '/' {
						// the last char is "/" so we need to add create a new segment
						lastPathSegment = true
					} else {
						// the last char is not a "/", so we need to add the last char to the
						// path segment value
						end = end + 1
					}
				}
				fmt.Printf("getPathSegments -> append begin %d, end %d\n", begin, end)
				ps = append(ps, pathSegment{
					value: string([]byte(path[begin+1 : end])),
					kind:  kind,
				})
				if lastPathSegment {
					ps = append(ps, pathSegment{
						value: string(c),
						kind:  kind,
					})
				}
				begin = idx
				kind = normal
			}
			switch c {
			case ':':
				// this means there is a : char in the pathSegment that is not the first one
				if kind != normal {
					valid = false
				}
				kind = param
			case '*':
				// this means there is a : char in the pathSegment that is not the first one
				if kind != normal {
					valid = false
				}
				kind = catchAll
			}
		}
	}
	return ps, valid
}

func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}

	finalPath := path.Join(absolutePath, relativePath)
	if lastChar(relativePath) == '/' && lastChar(finalPath) != '/' {
		return finalPath + "/"
	}
	return finalPath
}

func lastChar(str string) uint8 {
	if str == "" {
		panic("The length of the string can't be 0")
	}
	return str[len(str)-1]
}

/*
type skippedNode struct {
	path        string
	node        *node
	paramsCount int16
}
*/

type requestContext struct {
	httpMethod string
	path       string
	params     *Params
	//skippedNodes   *[]skippedNode
	unescape       bool
	pathSegments   []pathSegment
	pathSegmentIdx int
}

type valueContext struct {
	handlers HandlersChain
	params   *Params
	tsr      bool
	//fullPath string
}

func (r *methodTree) GetReqValue(reqCtx *requestContext) valueContext {
	r.m.RLock()
	defer r.m.RUnlock()
	fmt.Printf("getValue -> path: %s, params: %v\n", reqCtx.path, reqCtx.params)

	pathSegments, valid := getPathSegments(reqCtx.path)
	if !valid {
		return valueContext{}
	}
	fmt.Printf("getValue -> pathSegments: %v \n", pathSegments)
	n, ok := r.tree[reqCtx.httpMethod]
	if !ok {
		// httpMethod not found
		return valueContext{}
	}
	if len(pathSegments) == 1 {
		return valueContext{
			handlers: n.handlers,
		}
	}
	reqCtx.pathSegments = pathSegments
	reqCtx.pathSegmentIdx = 1
	return n.GetValue(reqCtx)
}

func (r *node) GetValue(reqCtx *requestContext) valueContext {
	r.m.RLock()
	defer r.m.RUnlock()
	node, ok := r.children[reqCtx.pathSegments[reqCtx.pathSegmentIdx].value]
	if !ok {
		fmt.Printf("getValue: children %v\n", r.children)
		fmt.Printf("getValue: not found -> %s\n", reqCtx.pathSegments[reqCtx.pathSegmentIdx].value)
		// if the node was not found we need to validate if there is a wildcard
		node, ok = r.children[wildcard]
		if !ok {
			// no wildcard found in this pathSegment
			return valueContext{}
		}
		// wildcard exists for the pathSegment
		// add the param KeyValue to the parameter list
		fmt.Printf("params: %v\n", reqCtx.params)
		i := len(*reqCtx.params)
		*reqCtx.params = (*reqCtx.params)[:i+1]
		(*reqCtx.params)[i] = Param{
			Key:   node.pathSegment.value[1:],
			Value: reqCtx.pathSegments[reqCtx.pathSegmentIdx].value,
		}
		fmt.Printf("params: %v\n", reqCtx.params)
	}

	fmt.Printf("getValue: pathSegments %v pathSegmentIdx: %d total ps length: %d\n", reqCtx.pathSegments, reqCtx.pathSegmentIdx, len(reqCtx.pathSegments)-1)
	if reqCtx.pathSegmentIdx == len(reqCtx.pathSegments)-1 {
		fmt.Printf("getValue: handlers: %v\n", len(r.handlers))
		return valueContext{
			handlers: node.handlers,
			params:   reqCtx.params,
		}
	}
	reqCtx.pathSegmentIdx++
	return node.GetValue(reqCtx)
}
