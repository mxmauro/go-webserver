// See the LICENSE file for license details.

package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/mxmauro/go-webserver/v2/metrics"
)

// -----------------------------------------------------------------------------

type healthOutput struct {
	Status string `json:"status"`
}

// -----------------------------------------------------------------------------

func main() {
	mc, err := metrics.CreateController(metrics.Options{
		Address:             "127.0.0.1",
		Port:                3000,
		HealthCallback:      healthCallback,
		EnableDebugProfiles: true,
	})
	if err != nil {
		fmt.Printf("unable to create metrics controller [%v]\n", err)
		return
	}
	defer mc.Stop()

	err = mc.NewCounterWithCallback(
		"random_counter", "A random counter",
		func() float64 {
			return rand.Float64()
		},
	)
	if err != nil {
		fmt.Printf("unable to create metric handler [%v]\n", err)
		return
	}

	if err = mc.Start(); err != nil {
		fmt.Printf("unable to start metrics controller [%v]\n", err)
		return
	}

	fmt.Println("Metrics controller running on http://127.0.0.1:3000")
	fmt.Println("Health:  http://127.0.0.1:3000/health")
	fmt.Println("Metrics: http://127.0.0.1:3000/metrics")
	fmt.Println("Press CTRL+C to stop.")
	openBrowser("http://127.0.0.1:3000/metrics")

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("Exiting...")
}

// -----------------------------------------------------------------------------

func healthCallback() string {
	state := healthOutput{
		Status: "ok",
	}

	j, _ := json.Marshal(state)
	return string(j)
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
