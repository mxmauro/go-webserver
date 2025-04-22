package go_webserver

//go:generate go run internal/generators/request_inherited.go

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"

	"github.com/mxmauro/go-webserver/v2/trusted_proxy"
	"github.com/mxmauro/go-webserver/v2/util"
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

var (
	preservedHeadersOnError = [][]byte{
		util.HeaderAccessControlAllowCredentials,
		util.HeaderAccessControlAllowHeaders,
		util.HeaderAccessControlAllowMethods,
		util.HeaderAccessControlAllowOrigin,
	}

	errPathNotHandled    = errors.New("path not handled")
	errInvalidCallToNext = errors.New("invalid call to Next")
)

// -----------------------------------------------------------------------------

// RequestContext represents a request context. It is used to handle the request and to provide access to the request
// and response objects.
type RequestContext struct {
	ctx     *fasthttp.RequestCtx
	tp      *trusted_proxy.TrustedProxy
	userCtx context.Context

	middlewareIndex   int
	srvRouterHandler  fasthttp.RequestHandler
	srvMiddlewares    []HandlerFunc
	srvMiddlewaresLen int
	middlewares       []HandlerFunc
	middlewaresLen    int
	handler           HandlerFunc
}

// -----------------------------------------------------------------------------

func (req *RequestContext) Request() *fasthttp.Request {
	return &req.ctx.Request
}

func (req *RequestContext) Response() *fasthttp.Response {
	return &req.ctx.Response
}

func (req *RequestContext) RequestHeaders() *fasthttp.RequestHeader {
	return &req.ctx.Request.Header
}

func (req *RequestContext) ResponseHeaders() *fasthttp.ResponseHeader {
	return &req.ctx.Response.Header
}

func (req *RequestContext) RequestHeader(key string) string {
	return string(req.ctx.Request.Header.Peek(key))
}

func (req *RequestContext) SetResponseHeader(key string, value string) {
	req.ctx.Response.Header.Set(key, value)
}

func (req *RequestContext) AddResponseHeader(key string, value string) {
	req.ctx.Response.Header.Add(key, value)
}

func (req *RequestContext) Error(msg string, statusCode int) {
	// Reset response but preserve some headers
	preservedHeaders := make(map[int][]byte)
	req.ctx.Response.Header.VisitAll(func(key, value []byte) {
		for idx, header := range preservedHeadersOnError {
			if bytes.Equal(key, header) {
				preservedHeaders[idx] = value
			}
		}
	})
	req.ctx.Response.Reset() // Real reset
	for idx, value := range preservedHeaders {
		req.ctx.Response.Header.SetBytesKV(preservedHeadersOnError[idx], value)
	}

	// Set the new status
	req.ctx.SetStatusCode(statusCode)

	// Set body and Content-Type
	msgLen := len(msg)
	if msgLen > 0 {
		isJson := false
		for idx := 0; idx < msgLen; idx++ {
			if msg[idx] == '{' || msg[idx] == '[' {
				isJson = true
				break
			} else if msg[idx] != ' ' && msg[idx] != '\t' && msg[idx] != '\r' && msg[idx] != '\n' {
				break
			}
		}

		if isJson {
			req.ctx.Response.Header.SetBytesKV(util.HeaderContentType, util.ContentTypeApplicationJSON)
		} else {
			req.ctx.Response.Header.SetBytesKV(util.HeaderContentType, util.ContentTypeTextPlain)
		}
		req.ctx.SetBodyString(msg)
	}
}

func (req *RequestContext) Success() {
	req.ctx.SetStatusCode(fasthttp.StatusOK)
}

func (req *RequestContext) NotFound(msg string) {
	req.sendError(fasthttp.StatusNotFound, msg)
}

func (req *RequestContext) BadRequest(msg string) {
	req.sendError(fasthttp.StatusBadRequest, msg)
}

func (req *RequestContext) Unauthorized(msg string) {
	req.sendError(fasthttp.StatusUnauthorized, msg)
}

func (req *RequestContext) AccessDenied(msg string) {
	req.sendError(fasthttp.StatusForbidden, msg)
}

func (req *RequestContext) InternalServerError(msg string) {
	req.sendError(fasthttp.StatusInternalServerError, msg)
}

func (req *RequestContext) ServiceUnavailable(msg string) {
	req.sendError(fasthttp.StatusServiceUnavailable, msg)
}

func (req *RequestContext) TooManyRequests(msg string) {
	req.sendError(fasthttp.StatusTooManyRequests, msg)
}

func (req *RequestContext) WriteJSON(obj interface{}) {
	req.ctx.Response.Header.SetBytesKV(util.HeaderContentType, util.ContentTypeApplicationJSON)
	req.ctx.Response.SetStatusCode(fasthttp.StatusOK)

	err := json.NewEncoder(req.ctx).Encode(obj)
	if err != nil {
		req.InternalServerError("unable to encode json output")
	}
}

func (req *RequestContext) NoContent(statusCode int) {
	req.ctx.Response.SetStatusCode(statusCode)
	req.ctx.Response.ResetBody()
}

func (req *RequestContext) CallFastHttpHandler(handler fasthttp.RequestHandler) {
	handler(req.ctx)
}

func (req *RequestContext) Host() string {
	if req.isProxyTrusted() {
		host := req.ctx.Request.Header.PeekBytes(util.HeaderXForwardedHost)
		if len(host) > 0 {
			commaPos := bytes.IndexByte(host, ',')
			if commaPos >= 0 {
				return string(host[:commaPos])
			}
			return string(host)
		}
	}
	return string(req.ctx.Request.URI().Host())
}

func (req *RequestContext) RemoteIP() net.IP {
	if req.isProxyTrusted() {
		addresses := req.ctx.Request.Header.PeekBytes(util.HeaderTrueClientIP)
		if len(addresses) == 0 {
			addresses = req.ctx.Request.Header.PeekBytes(util.HeaderXForwardedFor)
		}
		if len(addresses) > 0 {
			ip := getFirstIpAddress(addresses)
			if ip != nil {
				return ip
			}
		}
	}
	return req.ctx.RemoteIP()
}

func (req *RequestContext) Scheme() string {
	if req.ctx.IsTLS() {
		return "https"
	}
	scheme := "http"
	if req.isProxyTrusted() {
		req.ctx.Request.Header.VisitAll(func(key, val []byte) {
			if len(key) >= 12 && key[0] == 'X' && key[1] == '-' {
				if bytes.Equal(key, util.HeaderXForwardedProto) || bytes.Equal(key, util.HeaderXForwardedProtocol) {
					v := string(val)
					commaPos := strings.Index(v, ",")
					if commaPos != -1 {
						scheme = v[:commaPos]
					} else {
						scheme = v
					}
				} else if bytes.Equal(key, util.HeaderXForwardedSsl) && bytes.Equal(val, util.UnsafeString2ByteSlice("on")) {
					scheme = "https"
				} else if bytes.Equal(key, util.HeaderXUrlScheme) {
					scheme = string(val)
				}
			}
		})
	}
	return scheme
}

func (req *RequestContext) Next() error {
	var err error

	req.middlewareIndex += 1

	if req.middlewareIndex <= req.srvMiddlewaresLen {
		err = req.srvMiddlewares[req.middlewareIndex-1](req)
	} else if req.middlewareIndex == req.srvMiddlewaresLen+1 {
		req.srvRouterHandler(req.ctx)
		// If the status code is different from OK, assume someone created a response, for e.g., a redirection.
		if req.ResponseHeaders().StatusCode() == 200 {
			if req.handler != nil {
				err = req.Next()
			} else {
				err = errPathNotHandled
			}
		}
	} else if req.middlewareIndex <= req.srvMiddlewaresLen+1+req.middlewaresLen {
		err = req.middlewares[req.middlewareIndex-req.srvMiddlewaresLen-2](req)
	} else if req.middlewareIndex == req.srvMiddlewaresLen+2+req.middlewaresLen {
		err = req.handler(req)
	} else {
		err = errInvalidCallToNext
	}

	req.middlewareIndex -= 1

	// DOne
	return err
}

func (req *RequestContext) UserValueAsString(key []byte) (string, bool) {
	value := req.ctx.UserValueBytes(key)
	if value != nil {
		switch v := value.(type) {
		case string:
			return v, true
		case []byte:
			return string(v), true
		}
	}
	return "", false
}
