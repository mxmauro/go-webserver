package metrics_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/mxmauro/go-webserver/v2/internal/testcommon"
	"github.com/mxmauro/go-webserver/v2/metrics"
)

// -----------------------------------------------------------------------------

type State struct {
	System string `json:"system"`
}

// -----------------------------------------------------------------------------

func TestMetricsWebServer(t *testing.T) {
	// Create a new health and metrics controller with a web server
	mc, err := metrics.CreateController(metrics.Options{
		Address: "127.0.0.1",
		Port:    3000,
		HealthCallback: func() string {
			state := State{
				System: "all services running",
			}

			j, _ := json.Marshal(state)
			return string(j)
		},
		EnableDebugProfiles:  true,
		DebugProfilesApiPath: "/debug",
	})
	if err != nil {
		t.Fatalf("unable to create web server [%v]", err)
	}
	defer mc.Stop()

	// Create some custom counters
	err = mc.NewCounterWithCallback(
		"random_counter", "A random counter",
		func() float64 {
			return rand.Float64()
		},
	)
	if err != nil {
		t.Fatalf("unable to create metric handlers [NewCounterWithCallback] [%v]", err)
	}

	err = mc.NewCounterVecWithCallback(
		"random_counter_vec", "A random counter vector", []string{"set", "value"},
		metrics.VectorMetric{
			{
				Values: []string{"Set A", "Value 1"},
				Handler: func() float64 {
					return rand.Float64()
				},
			},
			{
				Values: []string{"Set A", "Value 2"},
				Handler: func() float64 {
					return rand.Float64()
				},
			},
			{
				Values: []string{"Set A", "Value 3"},
				Handler: func() float64 {
					return rand.Float64()
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("unable to create metric handlers [NewCounterVecWithCallback] [%v]", err)
	}

	err = mc.NewGaugeVecWithCallback(
		"random_gauge_vec", "A random gauge vector", []string{"set", "value"},
		metrics.VectorMetric{
			{
				Values: []string{"Set A", "Value 1"},
				Handler: func() float64 {
					return rand.Float64()
				},
			},
			{
				Values: []string{"Set A", "Value 2"},
				Handler: func() float64 {
					return rand.Float64()
				},
			},
			{
				Values: []string{"Set A", "Value 3"},
				Handler: func() float64 {
					return rand.Float64()
				},
			},
		},
	)
	if err != nil {
		t.Fatalf("unable to create metric handlers [NewGaugeVecWithCallback] [%v]", err)
	}

	// Start server
	err = mc.Start()
	if err != nil {
		t.Fatalf("unable to start web server [%v]", err)
	}

	// Open default browser
	testcommon.OpenBrowser("/metrics")

	// Wait for CTRL+C
	fmt.Println("Server running. Press CTRL+C to stop.")

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	select {
	case <-c:
	case <-time.After(1 * time.Minute):
	}
	fmt.Println("Shutting down...")
}
