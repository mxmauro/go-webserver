// See the LICENSE file for license details.

package middleware_test

import (
	"net/http"
	"strings"
	"testing"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/internal/testcommon"
	"github.com/mxmauro/go-webserver/v2/middleware"
)

// -----------------------------------------------------------------------------

func TestMiddlewareCORSSkipsInvalidOrigins(t *testing.T) {
	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		srv.Use(middleware.NewCORS(middleware.CORSOptions{
			AllowOrigins: []string{
				"invalid-origin",
				"https://valid.example",
				"https://bad.example/path",
			},
		}))

		return nil
	})
	defer srv.Stop()

	_, headers, err := testcommon.QueryApiVersion(false, nil, http.Header{
		"Origin": []string{"https://valid.example"},
	}, []int{200})
	if err != nil {
		t.Fatalf("unable to query api with valid origin [%v]", err)
	}
	if allowOrigin := headers.Get("Access-Control-Allow-Origin"); allowOrigin != "https://valid.example" {
		t.Fatalf("unexpected Access-Control-Allow-Origin for valid origin [got:%v / expected:%v]", allowOrigin, "https://valid.example")
	}

	_, headers, err = testcommon.QueryApiVersion(false, nil, http.Header{
		"Origin": []string{"https://other.example"},
	}, []int{200})
	if err != nil {
		t.Fatalf("unable to query api with unmatched origin [%v]", err)
	}
	if allowOrigin := headers.Get("Access-Control-Allow-Origin"); len(allowOrigin) != 0 {
		t.Fatalf("unexpected Access-Control-Allow-Origin for unmatched origin [got:%v]", allowOrigin)
	}
}

func TestMiddlewareCORSPreflightAndCredentials(t *testing.T) {
	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		srv.Use(middleware.NewCORS(middleware.CORSOptions{
			AllowOrigins:        []string{"https://api.example.com"},
			AllowMethods:        []string{http.MethodPost},
			AllowHeaders:        []string{"X-Test"},
			AllowCredentials:    true,
			ExposeHeaders:       []string{"X-Expose"},
			MaxAge:              123,
			AllowPrivateNetwork: true,
		}))

		return nil
	})
	defer srv.Stop()

	_, headers, body, err := testcommon.Query(http.MethodOptions, "/api/version", nil, http.Header{
		"Origin":                                 []string{"https://api.example.com"},
		"Access-Control-Request-Method":          []string{http.MethodPost},
		"Access-Control-Request-Headers":         []string{"X-Test"},
		"Access-Control-Request-Private-Network": []string{"true"},
	}, nil, []int{204})
	if err != nil {
		t.Fatalf("unable to query preflight api [%v]", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty preflight body [got:%d bytes]", len(body))
	}

	assertHeaderValue(t, headers, "Access-Control-Allow-Origin", "https://api.example.com")
	assertHeaderValue(t, headers, "Access-Control-Allow-Credentials", "true")
	assertHeaderValue(t, headers, "Access-Control-Allow-Methods", http.MethodPost)
	assertHeaderValue(t, headers, "Access-Control-Allow-Headers", "X-Test")
	assertHeaderValue(t, headers, "Access-Control-Allow-Private-Network", "true")
	assertHeaderValue(t, headers, "Access-Control-Max-Age", "123")

	vary := headers.Values("Vary")
	assertHeaderContains(t, vary, "Origin")
	assertHeaderContains(t, vary, "Access-Control-Request-Method")
	assertHeaderContains(t, vary, "Access-Control-Request-Headers")
	assertHeaderContains(t, vary, "Access-Control-Request-Private-Network")
}

func TestMiddlewareCORSWildcardAndSubdomainOrigins(t *testing.T) {
	t.Run("WildcardOrigin", func(t *testing.T) {
		srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
			srv.Use(middleware.NewCORS(middleware.CORSOptions{
				AllowOrigins:     []string{"*"},
				AllowCredentials: true,
			}))

			return nil
		})
		defer srv.Stop()

		_, headers, err := testcommon.QueryApiVersion(false, nil, http.Header{
			"Origin": []string{"https://foo.example.com"},
		}, []int{200})
		if err != nil {
			t.Fatalf("unable to query api with wildcard origin [%v]", err)
		}

		assertHeaderValue(t, headers, "Access-Control-Allow-Origin", "*")
		if value := headers.Get("Access-Control-Allow-Credentials"); len(value) != 0 {
			t.Fatalf("unexpected Access-Control-Allow-Credentials for wildcard origin [got:%v]", value)
		}
	})

	t.Run("SubdomainPattern", func(t *testing.T) {
		srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
			srv.Use(middleware.NewCORS(middleware.CORSOptions{
				AllowOrigins: []string{"https://*.example.com"},
			}))

			return nil
		})
		defer srv.Stop()

		_, headers, err := testcommon.QueryApiVersion(false, nil, http.Header{
			"Origin": []string{"https://api.example.com"},
		}, []int{200})
		if err != nil {
			t.Fatalf("unable to query api with subdomain origin [%v]", err)
		}
		assertHeaderValue(t, headers, "Access-Control-Allow-Origin", "https://api.example.com")

		_, headers, err = testcommon.QueryApiVersion(false, nil, http.Header{
			"Origin": []string{"https://example.com"},
		}, []int{200})
		if err != nil {
			t.Fatalf("unable to query api with apex origin [%v]", err)
		}
		if allowOrigin := headers.Get("Access-Control-Allow-Origin"); len(allowOrigin) != 0 {
			t.Fatalf("unexpected Access-Control-Allow-Origin for apex origin [got:%v]", allowOrigin)
		}
	})
}

// -----------------------------------------------------------------------------

func assertHeaderValue(t *testing.T, headers http.Header, key string, expected string) {
	t.Helper()

	if value := headers.Get(key); value != expected {
		t.Fatalf("unexpected %v [got:%v / expected:%v]", key, value, expected)
	}
}

func assertHeaderContains(t *testing.T, values []string, expected string) {
	t.Helper()

	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			if strings.TrimSpace(item) == expected {
				return
			}
		}
	}

	t.Fatalf("expected header value %v in %v", expected, values)
}
