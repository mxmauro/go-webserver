# go-webserver

`go-webserver` is an HTTP server library for Go built on top of `fasthttp` and `fasthttp/router`. It provides:

* A small server wrapper with route registration helpers.
* Middleware support at both server and route level.
* A request context wrapper with JSON/error helpers.
* Static file serving.
* Trusted proxy support for forwarded headers.
* Optional debug profile endpoints.

This repository is a fork of the original [RandLabs.IO go-webserver library](https://github.com/randlabs/go-webserver)
and may contain modified behavior.

## Installation

```bash
go get github.com/mxmauro/go-webserver/v2
```

## Server

Create a server with `webserver.Create(opts)`, register global middleware with `Use`, then add routes with `GET`,
`HEAD`, `OPTIONS`, `POST`, `PUT`, `PATCH`, `DELETE`, and `CustomMethod`.

`Start()` begins listening and `Stop()` performs a graceful shutdown.

### Options

`webserver.Options` supports the following common settings:

| Option                         | Description                                                     |
|--------------------------------|-----------------------------------------------------------------|
| `Name`                         | Server name sent in response headers.                           |
| `Address`, `Port`              | Bind target.                                                    |
| `ReadTimeout`, `WriteTimeout`  | Connection-level timeouts.                                      |
| `Concurrency`                  | Maximum number of concurrent connections.                       |
| `MaxRequestBodySize`           | Maximum accepted request body size.                             |
| `DisableKeepalive`             | Close connections after the first response.                     |
| `StreamRequestBody`            | Enable body streaming for large requests.                       |
| `DisablePreParseMultipartForm` | Disable automatic multipart parsing.                            |
| `TLSConfig`                    | Enable HTTPS and set TLS configuration.                         |
| `ListenErrorHandler`           | Callback invoked if the listener fails.                         |
| `RequestErrorHandler`          | Callback invoked when a handler or middleware returns an error. |
| `NotFoundHandler`              | Custom 404 handler.                                             |
| `MethodNotAllowedHandler`      | Custom 405 handler.                                             |
| `TrustedProxies`               | Exact IPs or CIDR ranges allowed to supply forwarded headers.   |

#### Trusted Proxies

Forwarded headers are ignored by default. If your server runs behind a reverse proxy and you want to trust headers
such as `X-Forwarded-For`, `True-Client-IP`, `X-Forwarded-Host`, and `X-Forwarded-Proto`, configure
`TrustedProxies`.

Example:

```go
srv, err := webserver.Create(webserver.Options{
	Address: "127.0.0.1",
	Port:    3000,
	TrustedProxies: []string{
		"127.0.0.1",
		"10.0.0.0/24",
	},
})
```

## RequestContext

Handlers receive a `*webserver.RequestContext`, which wraps `fasthttp.RequestCtx` and adds convenience helpers such as:

* Client info: `Host`, `RemoteIP`, `Scheme` and others.
* Response writer helpers: `WriteJSON`, `Success`, `BadRequest`, `Unauthorized`, `AccessDenied`, `InternalServerError`,
  `NoContent` and others.
* Access to the underlying request and response through `Request()` and `Response()`.
* Inherited `fasthttp.RequestCtx` methods exposed on `RequestContext`.

#### Additional Notes

When the remote address matches one of the trusted proxies:

* `req.RemoteIP()` uses `True-Client-IP` first, then `X-Forwarded-For`;
* `req.Host()` uses `X-Forwarded-Host`;
* `req.Scheme()` uses the forwarded scheme headers.

If the remote address is not trusted, those headers are ignored.

## Middlewares

Middlewares use the same `HandlerFunc` type as normal handlers:

```go
type HandlerFunc func(req *webserver.RequestContext) error
```

Global middleware runs through `srv.Use(...)`. Route-specific middleware is passed after the route handler, for
example `srv.GET("/private", handler, authMiddleware, auditMiddleware)`.

### Included Middleware

The `middleware` package includes reusable middleware for common tasks:

| Middleware           | Description                                             |
|----------------------|---------------------------------------------------------|
| CORS                 | Add CORS headers and handle preflight requests.         |
| Authentication       | Protect routes with token-based authentication.         |
| Rate Limiting        | Limit request rates per generated key.                  |
| Compression          | Compress HTTP responses.                                |
| ETag                 | Generate and validate ETag headers.                     |
| Client Cache Control | Disable client-side caching with cache-control headers. |
| Safe Browsing        | Add common browser security headers.                    |
| Trailing Slash       | Redirect or normalize trailing slashes in paths.        |
| Panic Recovery       | Recover from panics raised during request handling.     |
| Conditional          | Execute middleware conditionally.                       |
| No-Op                | Pass requests through without modification.             |

See the package source for option details and defaults.

## Error Handling

If a handler returns an error and you did not provide a custom `RequestErrorHandler`, the server responds with HTTP 500
and the error message.

For explicit responses, prefer returning `nil` after writing the response yourself:

```go
srv.GET("/example", func(req *webserver.RequestContext) error {
	req.BadRequest("missing parameter")
	return nil
})
```

## Static Files

Use `ServeFiles` to expose files from either the OS file system or any `fs.FS`.

### Serve from a local directory

```go
err := srv.ServeFiles("/", webserver.ServerFilesOptions{
	RootDirectory: "/var/www/public",
})
```

`RootDirectory` must be an absolute path when `FS` is not provided.

### Serve from an `fs.FS`

```go
publicFS := os.DirFS("./examples/ui")

err := srv.ServeFiles("/assets/", webserver.ServerFilesOptions{
	FS:         publicFS,
	FSBasePath: "public",
})
```

#### Useful Options

| Option                     | Description                                            |
|----------------------------|--------------------------------------------------------|
| `DisableDefaultIndexPages` | Disable automatic `index.html` and `index.htm` lookup. |
| `AcceptByteRange`          | Enable range requests.                                 |
| `NotFoundHandler`          | Override the default file-not-found behavior.          |
| `FSBasePath`               | Prepend a base path when serving from `fs.FS`.         |

## Debug Profiles

Expose Go runtime profiles with:

```go
srv.ServeDebugProfiles("/debug/pprof")
```

This adds endpoints under that base path, including CPU profiling and other standard runtime profiles.

## Health and Metrics

This repository also includes a separate metrics controller built on top of the same server package.
See [METRICS.md](./METRICS.md) for:

* `/health` endpoint support.
* Prometheus metrics exposure.
* Optional access-token protection.
* Optional debug profile exposure.

## Quick Start / Examples

Runnable examples live under [`examples/`](./examples):

- Basic API server: [examples/basic/main.go](./examples/basic/main.go)
- Static file and browser UI demo: [examples/ui/main.go](./examples/ui/main.go)
- Metrics controller example: [examples/metrics/main.go](./examples/metrics/main.go)

### How to Run

From the repository root:

```bash
go run ./examples/basic
go run ./examples/ui
go run ./examples/metrics
```

## Automated tests

```bash
go test ./...
go test -race ./...
```

## License

See the [`LICENSE`](./LICENSE) file for details.
