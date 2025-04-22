// See the LICENSE file for license details.

package go_webserver

import (
	"crypto/tls"
	"errors"
	"io/fs"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fasthttp/router"
	"github.com/mxmauro/go-webserver/v2/trusted_proxy"
	"github.com/mxmauro/go-webserver/v2/util"
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

// ListenErrorHandler is a callback to call if an error is encountered in the network listener.
type ListenErrorHandler func(srv *Server, err error)

// RequestErrorHandler is a callback to call if an error is encountered while processing a request.
type RequestErrorHandler func(req *RequestContext, err error)

// HandlerFunc defines a function that handles a request.
type HandlerFunc func(req *RequestContext) error

// Server is the main server object
type Server struct {
	srv                    fasthttp.Server
	router                 *router.Router
	bindAddress            net.IP
	bindPort               uint16
	listenErrorHandler     ListenErrorHandler
	requestErrorHandler    RequestErrorHandler
	middlewares            []HandlerFunc
	state                  int32
	startShutdownSignal    chan struct{}
	shutdownCompleteSignal chan struct{}
	requestCtxPool         *RequestContextPool
	trustedProxy           *trusted_proxy.TrustedProxy
}

// Options specify the server creation options.
type Options struct {
	// Server name to use when sending response headers. Defaults to 'go-webserver'.
	Name string

	// Address is the bind address to attach the server listener.
	Address string

	// Port is the port number the server will listen to.
	Port uint16

	// ReadTimeout is the amount of time allowed to read the full request including body. The connection's read
	// deadline is reset when the connection opens, or for keep-alive connections after the first byte has been read.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes of the response. It is reset after the
	// request handler has returned.
	WriteTimeout time.Duration

	// The maximum number of concurrent connections the server may serve. Defaults to 256K connections.
	Concurrency int

	// Maximum request body size.
	MaxRequestBodySize int

	// Closes incoming connections after sending the first response to the client.
	DisableKeepalive bool

	// Enable request body streaming and call the handler sooner when the given body is larger than the current limit.
	StreamRequestBody bool

	// Disable Multipart Form data parsing and return the binary blob instead.
	DisablePreParseMultipartForm bool

	// A callback to call if an error is encountered.
	ListenErrorHandler ListenErrorHandler

	// A callback to handle errors in requests.
	RequestErrorHandler RequestErrorHandler

	// A custom handler for 404 errors
	NotFoundHandler HandlerFunc

	// A custom handler for 405 errors
	MethodNotAllowedHandler HandlerFunc

	// TLSConfig optionally provides a TLS configuration for use.
	TLSConfig *tls.Config

	// If MinReqFileDescs is greater than zero, specifies the minimum number of required file descriptors
	// to be available.
	//
	// NOTES:
	// 1. Only valid on *nix operating systems.
	// 2. Starting from Go v1.19, the soft limit is automatically raised to the maximum allowed on process startup.
	MinReqFileDescs uint64

	// Use TrustedProxies to prevent header spoofing when you are behind a proxy. When used, and the remote IP
	// is a trusted proxy, the RequestContext object will behalf in the following way:
	//   1. Scheme: The value from X-Forwarded-Proto, X-Forwarded-Protocol, X-Forwarded-Ssl or X-Url-Scheme header
	//                will be used.
	//   2. RemoteIP: The value on ProxyHeader header will be used.
	//   3. Host: The value from the X-Forwarded-Host header will be used.
	TrustedProxies []string
}

// ServerFilesOptions sets the parameters to use in a ServeFiles call
type ServerFilesOptions struct {
	// Base directory where public files are located
	RootDirectory string

	// If a path with no file is requested (like '/'), by default, the file server will attempt to locate
	// 'index.html' and 'index.htm' files and serve them if available.
	DisableDefaultIndexPages bool

	// Accept client byte range requests
	AcceptByteRange bool

	// Custom "file-not-found" handler. Defaults to the server NotFound handler.
	NotFoundHandler HandlerFunc

	// File-system to use. Defaults to the OS file-system.
	FS         fs.FS
	FSBasePath string
}

// -----------------------------------------------------------------------------

const (
	DefaultReadTimeout        = 10 * time.Second
	DefaultWriteTimeout       = 10 * time.Second
	DefaultMaxRequestBodySize = 4 * 1048576 // 4MB
)

// -----------------------------------------------------------------------------

const (
	defaultServerName = "go-webserver"

	serveFilesSuffix = "{filepath:*}"
)

// -----------------------------------------------------------------------------

// Create creates a new webserver
func Create(opts Options) (*Server, error) {
	// Check options
	if len(opts.Address) == 0 {
		return nil, errors.New("invalid server bind address")
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return nil, errors.New("invalid server port")
	}

	readTimeout := opts.ReadTimeout
	if readTimeout < time.Duration(0) {
		return nil, errors.New("invalid read timeout")
	} else if readTimeout == time.Duration(0) {
		readTimeout = DefaultReadTimeout
	}

	writeTimeout := opts.WriteTimeout
	if writeTimeout < time.Duration(0) {
		return nil, errors.New("invalid write timeout")
	} else if writeTimeout == time.Duration(0) {
		writeTimeout = DefaultWriteTimeout
	}

	maxRequestBodySize := opts.MaxRequestBodySize
	if maxRequestBodySize < 0 {
		return nil, errors.New("invalid max request body size")
	} else if maxRequestBodySize == 0 {
		maxRequestBodySize = DefaultMaxRequestBodySize
	}

	parsedBindAddress := net.ParseIP(opts.Address)
	if parsedBindAddress == nil {
		return nil, errors.New("invalid server bind address")
	}
	if p4 := parsedBindAddress.To4(); len(p4) == net.IPv4len {
		parsedBindAddress = p4
	}

	if opts.MinReqFileDescs > 0 && util.CheckMaxFileDescriptors(opts.MinReqFileDescs) == false {
		return nil, errors.New("the number of process' file descriptors doesn't fulfill the minimum requirements")
	}

	// Create a new server container
	srv := &Server{
		router:                 router.New(),
		bindAddress:            parsedBindAddress,
		bindPort:               opts.Port,
		listenErrorHandler:     opts.ListenErrorHandler,
		requestErrorHandler:    opts.RequestErrorHandler,
		middlewares:            make([]HandlerFunc, 0),
		state:                  stateNotStarted,
		startShutdownSignal:    make(chan struct{}, 1),
		shutdownCompleteSignal: make(chan struct{}, 1),
		requestCtxPool:         newRequestContextPool(),
	}
	if len(opts.TrustedProxies) > 0 {
		srv.trustedProxy = trusted_proxy.NewTrustedProxy(opts.TrustedProxies)
	}

	// Set the default request error handler if none was specified.
	if srv.requestErrorHandler == nil {
		srv.requestErrorHandler = func(req *RequestContext, err error) {
			req.InternalServerError(err.Error())
		}
	}

	// Override some router settings
	srv.router.RedirectTrailingSlash = true
	srv.router.RedirectFixedPath = true
	srv.router.HandleMethodNotAllowed = true
	srv.router.HandleOPTIONS = false

	// Set the endpoint not found handler
	if opts.NotFoundHandler != nil {
		srv.router.NotFound = srv.createEndpointHandler(opts.NotFoundHandler)
	} else {
		srv.router.NotFound = func(ctx *fasthttp.RequestCtx) {
			ctx.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
		}
	}

	// Set the method not allowed handler
	if opts.MethodNotAllowedHandler != nil {
		srv.router.MethodNotAllowed = srv.createEndpointHandler(opts.MethodNotAllowedHandler)
	} else {
		srv.router.MethodNotAllowed = func(ctx *fasthttp.RequestCtx) {
			ctx.Error(fasthttp.StatusMessage(fasthttp.StatusMethodNotAllowed), fasthttp.StatusMethodNotAllowed)
		}
	}

	// Check the server name
	serverName := opts.Name
	if len(serverName) == 0 {
		serverName = defaultServerName
	}

	// Create FastHTTP server
	srv.srv = fasthttp.Server{
		Name:               serverName,
		Handler:            srv.createMainHandler(),
		ReadTimeout:        readTimeout,
		WriteTimeout:       writeTimeout,
		Concurrency:        opts.Concurrency,
		DisableKeepalive:   opts.DisableKeepalive,
		MaxRequestBodySize: maxRequestBodySize,
		TLSConfig:          opts.TLSConfig,
		Logger:             newSilentLogger(),
		CloseOnShutdown:    true,
	}

	// Done
	return srv, nil
}

// Start initiates listening
func (srv *Server) Start() error {
	if !atomic.CompareAndSwapInt32(&srv.state, stateNotStarted, stateStarting) {
		return errors.New("server is not stopped")
	}

	// Create the listener
	var network string

	// "tcp" network is not supported by all platforms
	address := srv.bindAddress.String()
	if len(srv.bindAddress) == net.IPv4len {
		network = "tcp4"
	} else {
		network = "tcp6"
		address = "[" + address + "]"
	}

	// Create a graceful shutdown listener
	ln, err := createListener(network, address+":"+strconv.Itoa(int(srv.bindPort)))
	if err != nil {
		srv.setState(stateNotStarted)
		return err
	}

	// Wrap a listener into a TLS listener if a TLS configuration was specified
	if srv.srv.TLSConfig != nil {
		ln = tls.NewListener(ln, srv.srv.TLSConfig.Clone())
	}

	// Start accepting connections and run in the background until shutdown or error
	srv.serve(ln)

	// Done
	return nil
}

// Stop shuts down the web server
func (srv *Server) Stop() {
	if atomic.CompareAndSwapInt32(&srv.state, stateRunning, stateStopping) {
		srv.startShutdownSignal <- struct{}{}

		// Spin until the server is really stopped
		for atomic.LoadInt32(&srv.state) == stateStopping {
			runtime.Gosched()
		}
	}
}

// Use adds a middleware that will be executed as part of the request handler
func (srv *Server) Use(middleware HandlerFunc) {
	srv.middlewares = append(srv.middlewares, middleware)
}

// GET adds a GET handler for the specified route
func (srv *Server) GET(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle("GET", path, srv.createEndpointHandler(handler, middlewares...))
}

// HEAD adds a HEAD handler for the specified route
func (srv *Server) HEAD(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle("HEAD", path, srv.createEndpointHandler(handler, middlewares...))
}

// OPTIONS add an OPTIONS handler for the specified route
func (srv *Server) OPTIONS(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle("OPTIONS", path, srv.createEndpointHandler(handler, middlewares...))
}

// POST adds a POST handler for the specified route
func (srv *Server) POST(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle("POST", path, srv.createEndpointHandler(handler, middlewares...))
}

// PUT adds a PUT handler for the specified route
func (srv *Server) PUT(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle("PUT", path, srv.createEndpointHandler(handler, middlewares...))
}

// PATCH adds a PATCH handler for the specified route
func (srv *Server) PATCH(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle("PATCH", path, srv.createEndpointHandler(handler, middlewares...))
}

// DELETE adds a DELETE handler for the specified route
func (srv *Server) DELETE(path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle("DELETE", path, srv.createEndpointHandler(handler, middlewares...))
}

// CustomMethod adds a custom method handler for the specified route
func (srv *Server) CustomMethod(method string, path string, handler HandlerFunc, middlewares ...HandlerFunc) {
	srv.router.Handle(method, path, srv.createEndpointHandler(handler, middlewares...))
}

// ServeFiles adds a custom filesystem handler for the specified route
func (srv *Server) ServeFiles(path string, opts ServerFilesOptions, middlewares ...HandlerFunc) error {
	var err error

	// Check some options
	if opts.FS == nil {
		if !filepath.IsAbs(opts.RootDirectory) {
			return errors.New("absolute path must be provided")
		}
	}

	// Normalize the path
	path, err = util.SanitizeUrlPath(path, 1)
	if err != nil {
		return err
	}
	path += serveFilesSuffix

	indexNames := make([]string, 0)
	if !opts.DisableDefaultIndexPages {
		indexNames = append(indexNames, "index.html", "index.htm")
	}

	// Create filesystem
	fhfs := fasthttp.FS{
		FS:                 opts.FS,
		Root:               opts.RootDirectory,
		IndexNames:         indexNames,
		GenerateIndexPages: !opts.DisableDefaultIndexPages,
		AcceptByteRange:    opts.AcceptByteRange,
		PathNotFound:       srv.router.NotFound,
	}
	if opts.NotFoundHandler != nil {
		fhfs.PathNotFound = srv.createEndpointHandler(opts.NotFoundHandler)
	}

	// If the url path contains a subdirectory within the base path, we must remove them to avoid mapping it
	// into the filesystem. I.e.: If the base path is '/foo/bar' and root-dir is '/tmp/public' is request to
	// '/foo/bar/index.html' would become '/tmp/public/foo/bar/index.html' instead of '/tmp/public/index.html'.
	toStrip := strings.Count(path[:len(path)-(len(serveFilesSuffix)+1)], "/") // Exclude the last fragment
	if toStrip > 0 {
		fhfs.PathRewrite = fasthttp.NewPathSlashesStripper(toStrip)
	}

	if opts.FS != nil && len(opts.FSBasePath) > 0 {
		var basePath []byte

		if !strings.HasPrefix(opts.FSBasePath, "/") {
			basePath = append([]byte("/"), []byte(opts.FSBasePath)...)
		} else {
			basePath = []byte(opts.FSBasePath)
		}
		if len(basePath) > 0 && basePath[len(basePath)-1] == '/' {
			basePath = basePath[:len(basePath)-1]
		}

		// Create a new path rewrite function
		origPathRewrite := fhfs.PathRewrite
		fhfs.PathRewrite = func(ctx *fasthttp.RequestCtx) []byte {
			var newPath []byte

			if origPathRewrite != nil {
				newPath = origPathRewrite(ctx)
			} else {
				newPath = ctx.Path()
			}
			return append(basePath, newPath...)
		}
	}

	// Wrap file-system handler
	fsHandler := fhfs.NewRequestHandler()
	handler := func(req *RequestContext) error {
		req.CallFastHttpHandler(fsHandler)
		return nil
	}

	// And add to router
	srv.router.Handle("GET", path, srv.createEndpointHandler(handler, middlewares...))

	// Done
	return nil
}
