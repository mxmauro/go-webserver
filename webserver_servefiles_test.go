// See the LICENSE file for license details.

package go_webserver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/internal/testcommon"
)

// -----------------------------------------------------------------------------

func TestServeFilesRequiresAbsolutePathWithoutFS(t *testing.T) {
	srv, err := webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3000,
	})
	if err != nil {
		t.Fatalf("unable to create web server [%v]", err)
	}

	err = srv.ServeFiles("/", webserver.ServerFilesOptions{
		RootDirectory: filepath.Join("examples", "ui", "public"),
	})
	if err == nil {
		t.Fatalf("expected relative RootDirectory to fail")
	}
}

func TestServeFilesRootDirectoryServesContentE2E(t *testing.T) {
	rootDirectory := filepath.Join(testcommon.GetWorkingDirectory(t), "examples", "ui", "public")

	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		return srv.ServeFiles("/", webserver.ServerFilesOptions{
			RootDirectory: rootDirectory,
		})
	})
	defer srv.Stop()

	_, headers, body, err := testcommon.Query("GET", "/index.html", nil, nil, nil, []int{200})
	if err != nil {
		t.Fatalf("unable to query root file [%v]", err)
	}
	if !strings.Contains(string(body), "go-webserver UI example") {
		t.Fatalf("unexpected root file contents")
	}
	if !strings.Contains(string(body), `fetch("/api/version")`) {
		t.Fatalf("unexpected root file client contents")
	}
	if contentType := headers.Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("unexpected root file content type [got:%v]", contentType)
	}
	_, headers, body, err = testcommon.Query("GET", "/api/version", nil, nil, nil, []int{200})
	if err != nil {
		t.Fatalf("unable to query version api [%v]", err)
	}
	if !strings.Contains(string(body), `"version":"1.0.0"`) {
		t.Fatalf("unexpected version api contents")
	}
	if contentType := headers.Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("unexpected version api content type [got:%v]", contentType)
	}
}

func TestServeFilesFSBasePathServesContentE2E(t *testing.T) {
	testFS := os.DirFS(filepath.Join(testcommon.GetWorkingDirectory(t), "examples", "ui"))

	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		return srv.ServeFiles("/assets/", webserver.ServerFilesOptions{
			FS:         testFS,
			FSBasePath: "public",
		})
	})
	defer srv.Stop()

	_, headers, body, err := testcommon.Query("GET", "/assets/index.html", nil, nil, nil, []int{200})
	if err != nil {
		t.Fatalf("unable to query fs-backed index file [%v]", err)
	}
	if !strings.Contains(string(body), "go-webserver UI example") {
		t.Fatalf("unexpected fs-backed index file contents")
	}
	if !strings.Contains(string(body), `fetch("/api/version")`) {
		t.Fatalf("unexpected fs-backed client contents")
	}
	if contentType := headers.Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("unexpected fs-backed content type [got:%v]", contentType)
	}
}
