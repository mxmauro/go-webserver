// See the LICENSE file for license details.

package go_webserver

import (
	"sync"

	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

type RequestContextPool struct {
	pool sync.Pool
}

// -----------------------------------------------------------------------------

func newRequestContextPool() *RequestContextPool {
	return &RequestContextPool{
		pool: sync.Pool{
			New: func() interface{} {
				return new(RequestContext)
			},
		},
	}
}

func (rcp *RequestContextPool) newRequestContext(ctx *fasthttp.RequestCtx, srv *Server) (*RequestContext, func()) {
	req, _ := rcp.pool.Get().(*RequestContext)
	req.ctx = ctx
	req.tp = srv.trustedProxy
	req.srvRouterHandler = srv.router.Handler
	req.srvMiddlewares = srv.middlewares
	req.srvMiddlewaresLen = len(srv.middlewares)
	ctx.SetUserValue(reqContextLinkKey, req)

	return req, func() {
		ctx.RemoveUserValue(reqContextLinkKey)
		req.ctx = nil
		req.tp = nil
		req.userCtx = nil
		req.middlewareIndex = 0
		req.handler = nil
		req.srvRouterHandler = nil
		req.srvMiddlewares = nil
		req.srvMiddlewaresLen = 0
		req.middlewares = nil
		req.middlewaresLen = 0

		rcp.pool.Put(req)
	}
}
