package middleware_test

import (
	"bytes"
	"net/http"
	"testing"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/internal/testcommon"
	"github.com/mxmauro/go-webserver/v2/middleware"
)

// -----------------------------------------------------------------------------

func TestMiddlewareAuth(t *testing.T) {
	var statusCode int

	// Create server
	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		// Add some middlewares
		srv.Use(middleware.NewAuth(middleware.AuthOptions{
			HeaderName:      "X-Auth",
			QueryName:       "auth",
			CookieName:      "auth",
			ValidateHandler: testAuthCheck,
		}))

		// Done
		return nil
	})
	defer srv.Stop()

	// Try with an authorization bearer
	_, _, err := testcommon.QueryApiVersion(true, nil, http.Header{
		"Authorization": []string{"Bearer abc1234"},
	}, []int{200})
	if err != nil {
		t.Fatalf("unable to query api with authorization bearer [%v]", err)
	}

	// Try with a bad authorization bearer
	statusCode, _, err = testcommon.QueryApiVersion(true, nil, http.Header{
		"Authorization": []string{"Bearer abc1234!!!"},
	}, []int{401})
	if err != nil {
		if statusCode == 401 || statusCode == 0 {
			t.Fatalf("unable to query api with wrong authorization bearer [%v]", err)
		} else {
			t.Fatalf("unexpected status code while querying api with wrong authorization bearer [%d]", statusCode)
		}
	}

	// Try with a header
	_, _, err = testcommon.QueryApiVersion(true, nil, http.Header{
		"X-Auth": []string{"abc1234"},
	}, []int{200})
	if err != nil {
		t.Fatalf("unable to query api with header [%v]", err)
	}

	// Try with a bad header
	statusCode, _, err = testcommon.QueryApiVersion(true, nil, http.Header{
		"X-Auth": []string{"abc1234!!!"},
	}, []int{401})
	if err != nil {
		if statusCode == 0 {
			t.Fatalf("unable to query api with wrong header [%v]", err)
		} else {
			t.Fatalf("unexpected status code while querying api with wrong header [%d]", statusCode)
		}
	}

	// Try with a query parameter
	_, _, err = testcommon.QueryApiVersion(true, map[string]string{
		"auth": "abc1234",
	}, nil, []int{200})
	if err != nil {
		t.Fatalf("unable to query api with query parameter [%v]", err)
	}

	// Try bad query parameter
	statusCode, _, err = testcommon.QueryApiVersion(true, map[string]string{
		"auth": "abc1234!!!",
	}, nil, []int{401})
	if err != nil {
		if statusCode == 401 || statusCode == 0 {
			t.Fatalf("unable to query api with wrong query parameter [%v]", err)
		} else {
			t.Fatalf("unexpected status code while querying api with wrong query parameter [%d]", statusCode)
		}
	}

	// Try with a cookie
	_, _, err = testcommon.QueryApiVersion(true, nil, http.Header{
		"Cookie": []string{"auth=abc1234"},
	}, []int{200})
	if err != nil {
		t.Fatalf("unable to query api with cookie [%v]", err)
	}

	// Try with a bad cookie
	statusCode, _, err = testcommon.QueryApiVersion(true, nil, http.Header{
		"Cookie": []string{"auth=abc1234!!!"},
	}, []int{401})
	if err != nil {
		if statusCode == 401 || statusCode == 0 {
			t.Fatalf("unable to query api with wrong cookie [%v]", err)
		} else {
			t.Fatalf("unexpected status code while querying api with wrong cookie [%d]", statusCode)
		}
	}
}

// -----------------------------------------------------------------------------

func testAuthCheck(_ *webserver.RequestContext, key []byte) (bool, error) {
	if key == nil {
		return false, nil
	}
	return bytes.Equal(key, []byte("abc1234")), nil
}
