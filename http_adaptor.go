package go_webserver

import (
	"net/http"

	"github.com/valyala/fasthttp/fasthttpadaptor"
)

// -----------------------------------------------------------------------------

// NewHandlerFromHttpHandler returns a HandlerFunc based on the provided http.Handler.
func NewHandlerFromHttpHandler(handler http.Handler) HandlerFunc {
	h := fasthttpadaptor.NewFastHTTPHandler(handler)
	return func(req *RequestContext) error {
		req.CallFastHttpHandler(h)
		return nil
	}
}

// NewHandlerFromHttpHandlerFunc returns a HandlerFunc based on the provided http.HandlerFunc.
func NewHandlerFromHttpHandlerFunc(f http.HandlerFunc) HandlerFunc {
	return NewHandlerFromHttpHandler(f)
}
