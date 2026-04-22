// See the LICENSE file for license details.

package metrics_test

import (
	"testing"

	"github.com/mxmauro/go-webserver/v2/metrics"
)

// -----------------------------------------------------------------------------

func TestCreateControllerRequiresHealthCallback(t *testing.T) {
	controller, err := metrics.CreateController(metrics.Options{
		Address: "127.0.0.1",
		Port:    3000,
	})
	if err == nil {
		if controller != nil {
			controller.Stop()
		}
		t.Fatalf("expected controller creation without HealthCallback to fail")
	}
}

func TestCreateControllerRejectsInvalidHealthApiPath(t *testing.T) {
	controller, err := metrics.CreateController(metrics.Options{
		Address:        "127.0.0.1",
		Port:           3000,
		HealthCallback: func() string { return `{"status":"ok"}` },
		HealthApiPath:  "..",
	})
	if err == nil {
		if controller != nil {
			controller.Stop()
		}
		t.Fatalf("expected controller creation with invalid HealthApiPath to fail")
	}
}

func TestCreateControllerRejectsInvalidMetricsApiPath(t *testing.T) {
	controller, err := metrics.CreateController(metrics.Options{
		Address:        "127.0.0.1",
		Port:           3000,
		HealthCallback: func() string { return `{"status":"ok"}` },
		MetricsApiPath: "..",
	})
	if err == nil {
		if controller != nil {
			controller.Stop()
		}
		t.Fatalf("expected controller creation with invalid MetricsApiPath to fail")
	}
}

func TestCreateControllerRejectsInvalidDebugProfilesApiPath(t *testing.T) {
	controller, err := metrics.CreateController(metrics.Options{
		Address:              "127.0.0.1",
		Port:                 3000,
		HealthCallback:       func() string { return `{"status":"ok"}` },
		EnableDebugProfiles:  true,
		DebugProfilesApiPath: "..",
	})
	if err == nil {
		if controller != nil {
			controller.Stop()
		}
		t.Fatalf("expected controller creation with invalid DebugProfilesApiPath to fail")
	}
}
