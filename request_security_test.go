// See the LICENSE file for license details.

package go_webserver

import (
	"net"
	"testing"

	"github.com/mxmauro/go-webserver/v2/trusted_proxy"
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

func TestRequestContextTrustedProxyHeaders(t *testing.T) {
	tests := []struct {
		name             string
		trustedProxies   []string
		remoteAddr       string
		trueClientIP     string
		forwardedFor     string
		forwardedHost    string
		forwardedProto   string
		expectedHost     string
		expectedRemoteIP string
		expectedScheme   string
	}{
		{
			name:             "NoTrustedProxyConfigIgnoresForwardedHeaders",
			remoteAddr:       "10.0.0.1",
			trueClientIP:     "198.51.100.10",
			forwardedFor:     "203.0.113.10",
			forwardedHost:    "proxy.example",
			forwardedProto:   "https",
			expectedHost:     "origin.example",
			expectedRemoteIP: "10.0.0.1",
			expectedScheme:   "http",
		},
		{
			name:             "ExactTrustedProxyUsesForwardedHeaders",
			trustedProxies:   []string{"10.0.0.1"},
			remoteAddr:       "10.0.0.1",
			forwardedFor:     "203.0.113.10",
			forwardedHost:    "proxy.example",
			forwardedProto:   "https",
			expectedHost:     "proxy.example",
			expectedRemoteIP: "203.0.113.10",
			expectedScheme:   "https",
		},
		{
			name:             "TrustedProxyCIDRUsesTrueClientIPFirst",
			trustedProxies:   []string{"10.0.0.0/24"},
			remoteAddr:       "10.0.0.25",
			trueClientIP:     "198.51.100.10",
			forwardedFor:     "203.0.113.10",
			forwardedHost:    "proxy.example",
			forwardedProto:   "https",
			expectedHost:     "proxy.example",
			expectedRemoteIP: "198.51.100.10",
			expectedScheme:   "https",
		},
		{
			name:             "UntrustedProxyCannotSpoofForwardedHeaders",
			trustedProxies:   []string{"10.0.1.1"},
			remoteAddr:       "10.0.0.1",
			trueClientIP:     "198.51.100.10",
			forwardedFor:     "203.0.113.10",
			forwardedHost:    "proxy.example",
			forwardedProto:   "https",
			expectedHost:     "origin.example",
			expectedRemoteIP: "10.0.0.1",
			expectedScheme:   "http",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ctx fasthttp.RequestCtx

			ctx.SetRemoteAddr(&net.TCPAddr{
				IP:   net.ParseIP(tc.remoteAddr),
				Port: 12345,
			})
			ctx.Request.SetHost("origin.example")
			if len(tc.trueClientIP) > 0 {
				ctx.Request.Header.Set("True-Client-IP", tc.trueClientIP)
			}
			if len(tc.forwardedFor) > 0 {
				ctx.Request.Header.Set("X-Forwarded-For", tc.forwardedFor)
			}
			if len(tc.forwardedHost) > 0 {
				ctx.Request.Header.Set("X-Forwarded-Host", tc.forwardedHost)
			}
			if len(tc.forwardedProto) > 0 {
				ctx.Request.Header.Set("X-Forwarded-Proto", tc.forwardedProto)
			}

			req := &RequestContext{
				ctx: &ctx,
			}
			if len(tc.trustedProxies) > 0 {
				req.tp = trusted_proxy.NewTrustedProxy(tc.trustedProxies)
			}

			if host := req.Host(); host != tc.expectedHost {
				t.Fatalf("unexpected host [got:%v / expected:%v]", host, tc.expectedHost)
			}
			if remoteIP := req.RemoteIP().String(); remoteIP != tc.expectedRemoteIP {
				t.Fatalf("unexpected remote IP [got:%v / expected:%v]", remoteIP, tc.expectedRemoteIP)
			}
			if scheme := req.Scheme(); scheme != tc.expectedScheme {
				t.Fatalf("unexpected scheme [got:%v / expected:%v]", scheme, tc.expectedScheme)
			}
		})
	}
}

func TestRequestContextNextRestoresMiddlewareIndexAfterPanic(t *testing.T) {
	var ctx fasthttp.RequestCtx

	req := &RequestContext{
		ctx: &ctx,
		srvMiddlewares: []HandlerFunc{
			func(req *RequestContext) error {
				return req.Next()
			},
		},
		srvMiddlewaresLen: 1,
		srvRouterHandler: func(_ *fasthttp.RequestCtx) {
			panic("boom")
		},
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic")
			}
		}()

		_ = req.Next()
	}()

	if req.middlewareIndex != 0 {
		t.Fatalf("middleware index was not restored after panic [got:%v / expected:%v]", req.middlewareIndex, 0)
	}
}
