// See the LICENSE file for license details.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/middleware"
)

// -----------------------------------------------------------------------------

func main() {
	srv, err := webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3000,
	})
	if err != nil {
		fmt.Printf("unable to create web server [%v]\n", err)
		return
	}

	srv.Use(middleware.DefaultCORS())
	srv.Use(middleware.DisableClientCache())

	_, thisFile, _, _ := runtime.Caller(0)
	rootDirectory := filepath.Join(filepath.Dir(thisFile), "public")

	err = srv.ServeFiles("/", webserver.ServerFilesOptions{
		RootDirectory: rootDirectory,
	}, middleware.NewCompression(middleware.CompressionLevelDefault))
	if err != nil {
		fmt.Printf("unable to configure file server [%v]\n", err)
		return
	}

	srv.GET("/api/version", func(req *webserver.RequestContext) error {
		req.WriteJSON(map[string]string{
			"version": "1.0.0",
		})
		req.Success()
		return nil
	})

	srv.ServeDebugProfiles("/debug/")

	if err = srv.Start(); err != nil {
		fmt.Printf("unable to start web server [%v]\n", err)
		return
	}
	defer srv.Stop()

	fmt.Println("UI example running on http://127.0.0.1:3000/")
	fmt.Println("Press CTRL+C to stop.")
	openBrowser("http://127.0.0.1:3000/")

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("Exiting...")
}

// -----------------------------------------------------------------------------

func openBrowser(rawURL string) {
	switch runtime.GOOS {
	case "linux":
		_ = exec.Command("xdg-open", rawURL).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
	case "darwin":
		_ = exec.Command("open", rawURL).Start()
	}
}
