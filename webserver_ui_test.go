// See the LICENSE file for license details.

package go_webserver_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/internal/testcommon"
	"github.com/mxmauro/go-webserver/v2/middleware"
)

// -----------------------------------------------------------------------------

func TestWebServerUI(t *testing.T) {
	rootDirectory := filepath.Join(testcommon.GetWorkingDirectory(t), "examples", "ui", "public")

	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		srv.Use(middleware.DefaultCORS())
		srv.Use(middleware.DisableClientCache())

		return srv.ServeFiles("/", webserver.ServerFilesOptions{
			RootDirectory: rootDirectory,
		}, middleware.NewCompression(middleware.CompressionLevelDefault))
	})
	defer srv.Stop()

	_, headers, body, err := testcommon.Query("GET", "/index.html", nil, http.Header{
		"Origin": []string{"https://ui.example"},
	}, nil, []int{200})
	if err != nil {
		t.Fatalf("unable to query UI index [%v]", err)
	}

	if !strings.Contains(string(body), "<title>go-webserver UI example</title>") {
		t.Fatalf("unexpected UI index contents")
	}
	if !strings.Contains(string(body), `fetch("/api/version")`) {
		t.Fatalf("unexpected UI client contents")
	}

	contentType := headers.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("unexpected UI content type [got:%v]", contentType)
	}

	cacheControl := headers.Get("Cache-Control")
	if !strings.Contains(cacheControl, "no-store") {
		t.Fatalf("unexpected Cache-Control header [got:%v]", cacheControl)
	}

	allowOrigin := headers.Get("Access-Control-Allow-Origin")
	if allowOrigin != "*" {
		t.Fatalf("unexpected Access-Control-Allow-Origin header [got:%v / expected:%v]", allowOrigin, "*")
	}
}
