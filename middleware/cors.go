// See the LICENSE file for license details.

package middleware

import (
	"bytes"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/util"
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

// CORSOptions defines the behavior on how CORS requests should be handled.
type CORSOptions struct {
	// List of origins that may access the resource. Defaults to "*" if a list is empty.
	AllowOrigins []string `json:"allow-origins,omitempty"`

	// List methods allowed when accessing the resource. If the list is defined but empty, the preflight `Allow`
	// request header value will be used.
	AllowMethods []string `json:"allow-methods,omitempty"`

	// List of request headers that can be used when making the actual request.
	AllowHeaders []string `json:"allow-headers,omitempty"`

	// This flag indicates whether the response to the request can be exposed when the credential flag is true.
	// Ignored if allowed origins is "*".
	// See: http://blog.portswigger.net/2016/10/exploiting-cors-misconfigurations-for.html
	AllowCredentials bool `json:"allow-credentials,omitempty"`

	// A set of headers that clients are allowed to access.
	ExposeHeaders []string `json:"expose-headers,omitempty"`

	// This field indicates how many seconds the results of a preflight request can be cached. Defaults to 0.
	MaxAge int `json:"max-age,omitempty"`

	// This flag indicates whether the Access-Control-Allow-Private-Network response header should be set to true and
	// allow requests from private networks. Defaults to false.
	AllowPrivateNetwork bool
}

type originPattern struct {
	scheme    string
	host      string
	subdomain bool
}

// -----------------------------------------------------------------------------

// DefaultCORS creates a default CORS middleware that allows requests from anywhere
func DefaultCORS() webserver.HandlerFunc {
	return NewCORS(CORSOptions{})
}

// NewCORS creates new CORS middleware based on the specified options
func NewCORS(opts CORSOptions) webserver.HandlerFunc {
	var allowedOrigins []originPattern
	var allowedMethods []byte
	var exposeHeaders []byte
	var allowedHeaders []byte
	var maxAge []byte

	// Parse options
	if len(opts.AllowOrigins) > 0 {
		allowedOrigins = make([]originPattern, 0)

		for _, origin := range opts.AllowOrigins {
			if len(origin) == 0 {
				continue
			}
			if origin == "*" {
				allowedOrigins = nil
				break
			}

			idx := strings.Index(origin, "://")
			if idx < 0 {
				panic("go-webserver[CORS]: invalid origin")
			}

			op := originPattern{
				scheme: strings.ToLower(origin[:idx]) + "://",
				host:   strings.ToLower(origin[idx+3:]),
			}
			if op.scheme != "http" && op.scheme != "https" {
				panic("go-webserver[CORS]: invalid origin scheme")
			}
			if len(op.host) >= 2 && strings.HasPrefix(op.host, "*.") {
				op.subdomain = true
				op.host = op.host[2:]
			}

			idx = strings.Index(op.host, "/")
			if idx >= 0 {
				if len(op.host) > idx+1 {
					panic("go-webserver[CORS]: invalid origin path, query param or fragment")
				}
				op.host = op.host[:idx]
			}

			if _, err := url.Parse("http://" + op.host); err != nil {
				panic("go-webserver[CORS]: invalid origin host")
			}

			allowedOrigins = append(allowedOrigins, op)
		}
	}

	if len(opts.AllowMethods) > 0 {
		allowedMethods = util.UnsafeString2ByteSlice(strings.Join(opts.AllowMethods, ","))
	} else {
		allowedMethods = util.UnsafeString2ByteSlice(
			http.MethodGet + "," + http.MethodPost + "," + http.MethodHead + "," + http.MethodPut + "," +
				http.MethodPatch + "," + http.MethodDelete,
		)
	}
	if len(opts.AllowHeaders) > 0 {
		allowedHeaders = util.UnsafeString2ByteSlice(strings.Join(opts.AllowHeaders, ","))
	}
	if len(opts.ExposeHeaders) > 0 {
		exposeHeaders = util.UnsafeString2ByteSlice(strings.Join(opts.ExposeHeaders, ","))
	}
	if opts.MaxAge > 0 {
		maxAge = util.UnsafeString2ByteSlice(strconv.Itoa(opts.MaxAge))
	} else if opts.MaxAge < 0 {
		maxAge = []byte{48} // Zero
	}

	// Setup middleware function
	return func(req *webserver.RequestContext) error {
		var allowOrigin string

		// Get request origin
		origin := util.UnsafeByteSlice2String(req.RequestHeaders().PeekBytes(util.HeaderOrigin))
		// See https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request for details on how to handle
		// preflight request.
		isMethodOptions := bytes.Equal(req.Method(), util.MethodOptions)

		// No origin provided? Then no CORS request
		if len(origin) == 0 {
			// Add Vary header if not all origins are allowed
			// See https://fetch.spec.whatwg.org/#cors-protocol-and-http-caches
			if allowedOrigins != nil {
				req.ResponseHeaders().AddBytesKV(util.HeaderVary, util.HeaderOrigin)
			}
			if isMethodOptions {
				req.NoContent(http.StatusNoContent)
				return nil
			}
			// Go to next middleware
			return req.Next()
		}

		// A preflight request must have the Access-Control-Request-Method header
		if isMethodOptions && len(req.RequestHeaders().PeekBytes(util.HeaderAccessControlRequestMethod)) == 0 {
			// Adding Vary request for http cache
			req.ResponseHeaders().SetBytesKV(util.HeaderVary, util.HeaderOrigin)
			// Go to next middleware
			return req.Next()
		}

		// Check for allowed origins
		if allowedOrigins == nil {
			allowOrigin = "*"
		} else {
			allowOrigin = ""
			for _, o := range allowedOrigins {
				if o.match(origin) {
					allowOrigin = origin
					break
				}
			}
		}

		if !isMethodOptions {
			// Add Vary header if not all origins are allowed
			// See https://fetch.spec.whatwg.org/#cors-protocol-and-http-caches
			if allowedOrigins != nil {
				req.ResponseHeaders().AddBytesKV(util.HeaderVary, util.HeaderOrigin)
			}

			setCommonHeaders(req.ResponseHeaders(), allowOrigin, maxAge, exposeHeaders, opts.AllowCredentials)

			// Go to next middleware
			return req.Next()
		}

		// If we reach here, we are dealing with a pre-flight request
		respHdrs := req.ResponseHeaders()

		respHdrs.AddBytesKV(util.HeaderVary, util.HeaderOrigin)
		respHdrs.AddBytesKV(util.HeaderVary, util.HeaderAccessControlRequestMethod)
		respHdrs.AddBytesKV(util.HeaderVary, util.HeaderAccessControlRequestHeaders)

		setCommonHeaders(req.ResponseHeaders(), allowOrigin, maxAge, exposeHeaders, opts.AllowCredentials)

		if opts.AllowPrivateNetwork && bytes.Equal(req.RequestHeaders().PeekBytes(util.HeaderAccessControlRequestPrivateNetwork), util.BytesTrue) {
			respHdrs.AddBytesKV(util.HeaderVary, util.HeaderAccessControlRequestPrivateNetwork)
			respHdrs.SetBytesKV(util.HeaderAccessControlAllowPrivateNetwork, util.BytesTrue)
		}

		respHdrs.SetBytesKV(util.HeaderAccessControlAllowMethods, allowedMethods)
		if len(allowedHeaders) > 0 {
			respHdrs.SetBytesKV(util.HeaderAccessControlAllowHeaders, allowedHeaders)
		} else {
			header := req.RequestHeaders().PeekBytes(util.HeaderAccessControlRequestHeaders)
			if len(header) > 0 {
				respHdrs.SetBytesKV(util.HeaderAccessControlAllowHeaders, header)
			}
		}

		// Done
		req.NoContent(http.StatusNoContent)
		return nil
	}
}

// -----------------------------------------------------------------------------

func (op *originPattern) match(origin string) bool {
	if !strings.HasPrefix(origin, op.scheme) {
		return false // Scheme mismatch
	}

	host := origin[len(op.scheme):]

	if !strings.HasSuffix(host, op.host) {
		return false // Host mismatch
	}

	subdomain := host[:len(host)-len(op.host)]
	if !op.subdomain {
		if len(subdomain) > 0 {
			return false // Unexpected subdomain
		}
	} else {
		if len(subdomain) < 2 || (!strings.HasSuffix(subdomain, ".")) {
			return false // Expected a subdomain
		}
	}
	// Does match!
	return true
}

func setCommonHeaders(respHdrs *fasthttp.ResponseHeader, allowOrigin string, maxAge []byte, exposeHeaders []byte, allowCredentials bool) {
	// Allow-Origin and Allow-Credentials
	if len(allowOrigin) > 0 {
		respHdrs.SetBytesK(util.HeaderAccessControlAllowOrigin, allowOrigin)
		if allowCredentials && allowOrigin != "*" {
			respHdrs.SetBytesKV(util.HeaderAccessControlAllowCredentials, util.BytesTrue)
		}
	}

	// MaxAge
	if maxAge != nil {
		respHdrs.SetBytesKV(util.HeaderAccessControlMaxAge, maxAge)
	}

	// Expose-Headers
	if exposeHeaders != nil {
		respHdrs.SetBytesKV(util.HeaderAccessControlExposeHeaders, exposeHeaders)
	}
}
