// See the LICENSE file for license details.

package testcommon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/util"
)

// -----------------------------------------------------------------------------

type TestWebServer struct {
	Server *webserver.Server
	t      *testing.T
}

type versionApiOutput struct {
	Version string `json:"version"`
}

// -----------------------------------------------------------------------------

func RunWebServer(t *testing.T, initCB func(srv *webserver.Server) error) *TestWebServer {
	var err error

	tws := &TestWebServer{
		t: t,
	}

	//Create server
	tws.Server, err = webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3000,
	})
	if err != nil {
		t.Fatalf("unable to create web server [%v]", err)
	}

	// Add some fake endpoints
	tws.Server.GET("/api/version", renderApiVersion)
	tws.Server.POST("/api/version", renderApiVersion)

	// Add also profile output
	tws.Server.ServeDebugProfiles("/debug/")

	// init callback
	err = initCB(tws.Server)
	if err != nil {
		t.Fatalf("%v", err)
	}

	// Start server
	err = tws.Server.Start()
	if err != nil {
		t.Fatalf("unable to start web server [%v]", err)
	}

	// Done
	return tws
}

func (tws *TestWebServer) Stop() {
	tws.Server.Stop()
}

func GetWorkingDirectory(t *testing.T) string {
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("unable to get current directory [%v]", err)
	}
	if !strings.HasSuffix(workDir, string(os.PathSeparator)) {
		workDir += string(os.PathSeparator)
	}
	return workDir
}

func QueryApiVersion(doPost bool, queryParams map[string]string, headers http.Header, expectedStatus []int) (int, http.Header, error) {
	var body io.Reader
	var resp *http.Response

	method := http.MethodGet
	if doPost {
		method = http.MethodPost
		body = bytes.NewReader([]byte(`{ "some-key": "some-value" }`))
	}

	rawUrl := "http://127.0.0.1:3000/api/version"
	if queryParams != nil {
		first := "?"
		for k, v := range queryParams {
			rawUrl += first + k + "=" + url.QueryEscape(v)
			first = "&"
		}
	}
	req, err := http.NewRequest(method, rawUrl, body)
	if err != nil {
		return 0, nil, err
	}
	if headers != nil {
		req.Header = headers
	}

	reqCtx, reqCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer reqCtxCancel()

	resp, err = http.DefaultClient.Do(req.WithContext(reqCtx))
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		return 0, nil, err
	}
	if resp == nil {
		return 0, nil, fmt.Errorf("unexpected status code while querying api [0]")
	}

	statusOk := false
	for _, status := range expectedStatus {
		if status == resp.StatusCode {
			statusOk = true
			break
		}
	}
	if !statusOk {
		return resp.StatusCode, nil, fmt.Errorf("unexpected status code while querying api [%v]", resp.StatusCode)
	}

	// Done
	return resp.StatusCode, resp.Header, nil
}

func OpenBrowser(path string) {
	rawUrl := "http://127.0.0.1:3000" + path
	switch runtime.GOOS {
	case "linux":
		_ = exec.Command("xdg-open", rawUrl).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawUrl).Start()
	case "darwin":
		_ = exec.Command("open", rawUrl).Start()
	}
}

// -----------------------------------------------------------------------------

func renderApiVersion(req *webserver.RequestContext) error {
	// If a POST request, check for the expected body
	if bytes.Equal(req.Method(), util.UnsafeString2ByteSlice(http.MethodPost)) {
		var decodedBody map[string]interface{}

		bodyCheckSucceeded := false
		body := req.PostBody()
		if body != nil {
			if json.Unmarshal(body, &decodedBody) == nil {
				value, ok := decodedBody["some-key"]
				if ok {

					switch v := value.(type) {
					case string:
						if v == "some-value" {
							bodyCheckSucceeded = true
						}
					}
				}
			}
		}
		if !bodyCheckSucceeded {
			req.BadRequest("some-value not found in body")
			return nil
		}
	}

	// Create output
	output := versionApiOutput{
		Version: "1.0.0",
	}
	req.WriteJSON(output)
	req.Success()
	return nil
}
