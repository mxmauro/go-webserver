// See the LICENSE file for license details.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/middleware"
)

// -----------------------------------------------------------------------------

type statusOutput struct {
	Status string `json:"status"`
}

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

	srv.GET("/status", func(req *webserver.RequestContext) error {
		req.WriteJSON(statusOutput{
			Status: "ok",
		})
		req.Success()
		return nil
	})

	if err = srv.Start(); err != nil {
		fmt.Printf("unable to start web server [%v]\n", err)
		return
	}
	defer srv.Stop()

	fmt.Println("Server running on http://127.0.0.1:3000/status")
	fmt.Println("Press CTRL+C to stop.")
	openBrowser("http://127.0.0.1:3000/status")

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
