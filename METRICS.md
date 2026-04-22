# Health and Metrics Controller

The `metrics` package provides a small controller for operational endpoints. It supports:

* A health endpoint.
* A Prometheus metrics endpoint.
* Optional debug profile endpoints.
* Optional access-token protection.
* Integration with an existing `go-webserver` server, or an internally managed one.

It is intended for observability and runtime diagnostics rather than application routing.

## Installation

```bash
go get github.com/mxmauro/go-webserver/v2/metrics
```

## Endpoints

When you do not override the paths, the controller exposes:

* `GET /health`
* `GET /metrics`
* `GET /debug/pprof` and related profile routes, if enabled

Path overrides are available through `HealthApiPath`, `MetricsApiPath`, and `DebugProfilesApiPath`.

All custom paths are sanitized before registration.

## Controller Modes

### Internal Server

If you provide `Address` and `Port` without `Server`, the controller creates and owns its own `go-webserver.Server`.
In this mode:

* `mc.Start()` starts the internal server.
* `mc.Stop()` stops it.
* Cache disabling and CORS middleware are added automatically for the operational endpoints.

### Existing Server

If you provide `Options.Server`, the controller registers its handlers on that server instead of creating a new one.

In this mode:

* You are responsible for starting and stopping the shared server.
* `mc.Start()` returns an error because the controller does not own the server lifecycle.
* `mc.Stop()` still tears down internal controller state.
* Cache and CORS behavior for the shared server is your responsibility.

## Options

`metrics.Options` supports the following main fields:

| Option                       | Description                                                           |
|------------------------------|-----------------------------------------------------------------------|
| `Server`                     | Existing `go-webserver.Server` instance to reuse.                     |
| `Name`                       | Name for the internally created server. Defaults to `metrics-server`. |
| `Address`, `Port`            | Bind target for the internal server.                                  |
| `TLSConfig`                  | Optional TLS configuration for HTTPS.                                 |
| `ListenErrorHandler`         | Listener error callback.                                              |
| `AccessToken`                | Shared token required for protected endpoints.                        |
| `RequestAccessTokenInHealth` | Require the token for `/health` too.                                  |
| `HealthCallback`             | Required callback that returns the health payload as a string.        |
| `EnableDebugProfiles`        | Expose debug profile routes.                                          |
| `Middlewares`                | Additional middleware applied to metrics endpoints.                   |
| `HealthApiPath`              | Custom path for the health endpoint.                                  |
| `MetricsApiPath`             | Custom path for the metrics endpoint.                                 |
| `DebugProfilesApiPath`       | Custom path for the debug profiles endpoint.                          |

## Health Endpoint

`HealthCallback` is required. It returns a string that is sent as the response body.

Common patterns include:

* Returning JSON.
* Returning plain text.
* Returning a summary derived from current application state.

The package does not enforce a schema for the health payload. If you want JSON, return JSON from the callback.

## Prometheus Metrics

The controller maintains its own Prometheus registry and exposes it through the metrics handler. You can also access
the registry directly with:

```go
registry := mc.Registry()
```

This is useful if you want to register custom collectors directly through the Prometheus APIs.

The helper methods provided by the package are:

* `NewCounterWithCallback`
* `NewCounterVecWithCallback`
* `NewGaugeWithCallback`
* `NewGaugeVecWithCallback`

### Metric Examples

Single-value metric:

```go
err = mc.NewGaugeWithCallback(
	"queue_depth",
	"Current queue depth",
	func() float64 {
		return float64(queueDepth())
	},
)
```

Labelled metric:

```go
err = mc.NewGaugeVecWithCallback(
	"http_inflight_requests",
	"Current inflight requests by route",
	[]string{"route"},
	metrics.VectorMetric{
		{
			Values: []string{"/api/users"},
			Handler: func() float64 {
				return float64(usersInflight())
			},
		},
		{
			Values: []string{"/api/orders"},
			Handler: func() float64 {
				return float64(ordersInflight())
			},
		},
	},
)
```

## Access Control

If `AccessToken` is set, the controller protects:

* `/metrics`.
* Debug profile routes.
* `/health`, only if `RequestAccessTokenInHealth` is `true`.

Authentication is implemented with the shared middleware stack and a constant-time token comparison.

The token is checked by the auth middleware using the standard supported lookup locations from `middleware.NewAuth`.

## Middleware Behavior

The controller always adds internal liveness and rundown-protection middleware.

When the controller owns the server, it also adds:

* `middleware.DisableClientCache()`
* `middleware.NewCORS(...)` with `GET`-only semantics for the operational endpoints

Any middleware passed through `Options.Middlewares` is prepended before the internal controller middleware stack.

## Debug Profiles

If `EnableDebugProfiles` is `true`, the controller registers Go runtime profile routes by calling `ServeDebugProfiles`
on the underlying server.

By default, the profile base path is:

```text
/debug/pprof
```

You can override it with `DebugProfilesApiPath`.

## Practical Deployment Notes

* Keep `/metrics` private unless you explicitly want public metrics.
* Use `AccessToken` or network-level restrictions for operational endpoints.
* Use `TLSConfig` if the internal server is directly exposed over HTTPS.
* Reuse an existing app server only if you want operational endpoints on the same listener.
* Prefer an internal dedicated listener if you want clear separation between application traffic and operational
  traffic.

## Quick Start / Example

Runnable example:

* [examples/metrics/main.go](./examples/metrics/main.go)

Run it from the repository root with:

```bash
go run ./examples/metrics
```

Example using an internal server:

```go
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"

	"github.com/mxmauro/go-webserver/v2/metrics"
)

type healthOutput struct {
	Status string `json:"status"`
}

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

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

func healthCallback() string {
	state := healthOutput{
		Status: "ok",
	}

	j, _ := json.Marshal(state)
	return string(j)
}
```

## Related Documentation

* Main server package: [README.md](./README.md)
* Prometheus client library: https://github.com/prometheus/client_golang

## License

See the [`LICENSE`](./LICENSE) file for details.
