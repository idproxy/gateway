package gateway

type HandlerFunc func(gctx *Context)

// HandlersChain defines a HandlerFunc slice
type HandlersChain []HandlerFunc

// Last returns the last handler in the chain
func (r HandlersChain) Last() HandlerFunc {
	if l := len(r); l > 0 {
		return r[l-1]
	}
	return nil
}
