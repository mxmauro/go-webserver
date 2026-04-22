// See the LICENSE file for license details.

package go_webserver

import (
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

func (srv *Server) createMainHandler() fasthttp.RequestHandler {
	// Wrapper.
	return func(ctx *fasthttp.RequestCtx) {
		// Create a new request context.
		req, freeReq := srv.requestCtxPool.newRequestContext(ctx, srv)
		defer freeReq()

		err := req.Next()
		if err != nil {
			srv.requestErrorHandler(req, err)
		}
	}
}

func (srv *Server) createEndpointHandler(h HandlerFunc, middlewares ...HandlerFunc) fasthttp.RequestHandler {
	// Wrapper.
	return func(ctx *fasthttp.RequestCtx) {
		req, ok := ctx.UserValue(reqContextLinkKey).(*RequestContext)
		if ok {
			req.setHandlerParams(h, middlewares)
		}
	}
}
