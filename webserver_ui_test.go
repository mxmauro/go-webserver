// See the LICENSE file for license details.

package go_webserver_test

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/internal/testcommon"
	"github.com/mxmauro/go-webserver/v2/middleware"
)

// -----------------------------------------------------------------------------

func TestWebServerUI(t *testing.T) {
	//Create server
	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		// Add some middlewares
		srv.Use(middleware.DefaultCORS())
		srv.Use(middleware.DisableClientCache())

		// Add public files to the server
		err := srv.ServeFiles("/", webserver.ServerFilesOptions{
			RootDirectory: testcommon.GetWorkingDirectory(t) + "testdata/public",
		}, middleware.NewCompression(middleware.CompressionLevelDefault))
		if err != nil {
			return err
		}

		// Done
		return nil
	})
	defer srv.Stop()

	// Open default browser
	testcommon.OpenBrowser("/")

	// Wait for CTRL+C
	fmt.Println("Server running. Press CTRL+C to stop.")

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-c:
	case <-time.After(5 * time.Minute):
	}
	fmt.Println("Shutting down...")
}
