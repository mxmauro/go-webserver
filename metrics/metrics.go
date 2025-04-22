// See the LICENSE file for license details.

package metrics

// -----------------------------------------------------------------------------

import (
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/mxmauro/go-rundownprotection"
	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/middleware"
	"github.com/mxmauro/go-webserver/v2/util"
	"github.com/prometheus/client_golang/prometheus"
)

// -----------------------------------------------------------------------------

// Controller holds details about a metrics monitor instance.
type Controller struct {
	rp                  *rundownprotection.RundownProtection
	server              *webserver.Server
	usingInternalServer bool
	registry            *prometheus.Registry
	healthCallback      HealthCallback
}

// Options specify metrics controller initialization options.
type Options struct {
	// If a server is provided, use this server instead of creating a new one.
	Server *webserver.Server

	// Server name to use when sending response headers. Defaults to 'metrics-server'.
	Name string

	// Address is the bind address to attach the internal web server.
	Address string

	// Port is the port number the internal web server will use.
	Port uint16

	// TLSConfig optionally provides a TLS configuration for use.
	TLSConfig *tls.Config

	// A callback to call if an error is encountered.
	ListenErrorHandler webserver.ListenErrorHandler

	// AccessToken is an optional access token required to access the status endpoints.
	AccessToken string

	// If RequestAccessTokenInHealth is enabled, access token checked also in the "/health" endpoint.
	RequestAccessTokenInHealth bool

	// HealthCallback is a function that returns an object which, in turn, will be converted to JSON format.
	HealthCallback HealthCallback

	// Expose debugging profiles /debug/pprof endpoint.
	EnableDebugProfiles bool

	// Middlewares additional set of middlewares for the endpoints.
	Middlewares []webserver.HandlerFunc

	// If HealthApiPath is defined, it will override the default "/health" path for health requests.
	HealthApiPath string
	// If MetricsApiPath is defined, it will override the default "/metrics" path for metrics requests.
	MetricsApiPath string
	// If DebugProfilesApiPath is defined, it will override the default "/debug/pprof" path for debug profile requests.
	DebugProfilesApiPath string
}

// HealthCallback indicates a function that returns a string that will be returned as the output.
type HealthCallback func() string

// -----------------------------------------------------------------------------

const (
	defaultServerName = "metrics-server"
)

// -----------------------------------------------------------------------------

// CreateController initializes and creates a new controller
func CreateController(opts Options) (*Controller, error) {
	var path string
	var err error

	if opts.HealthCallback == nil {
		return nil, errors.New("invalid health callback")
	}

	// Create a metrics object
	mws := Controller{
		rp:             rundownprotection.Create(),
		healthCallback: opts.HealthCallback,
	}

	// Create webserver
	if opts.Server != nil {
		mws.server = opts.Server
	} else {
		serverName := opts.Name
		if len(serverName) == 0 {
			serverName = defaultServerName
		}

		mws.usingInternalServer = true
		mws.server, err = webserver.Create(webserver.Options{
			Name:               serverName,
			Address:            opts.Address,
			Port:               opts.Port,
			ReadTimeout:        10 * time.Second, // 10 seconds for reading a metrics request
			WriteTimeout:       time.Minute,      // and 1 minute to write
			MaxRequestBodySize: 512,              // Currently, no POST endpoints but leave a small buffer for future requests.
			ListenErrorHandler: opts.ListenErrorHandler,
			TLSConfig:          opts.TLSConfig,
			MinReqFileDescs:    16,
		})
		if err != nil {
			mws.Stop()
			return nil, fmt.Errorf("unable to create metrics web server [err=%v]", err)
		}
	}

	// Create Prometheus handler
	err = mws.createPrometheusRegistry()
	if err != nil {
		mws.Stop()
		return nil, err
	}

	// Add middlewares
	middlewares := make([]webserver.HandlerFunc, 0)
	if len(opts.Middlewares) > 0 {
		middlewares = append(middlewares, opts.Middlewares...)
	}
	middlewares = append(middlewares, mws.createAliveMiddleware())
	if opts.Server == nil {
		// Only disable cache and support CORS if we own the webserver, else the caller must take care of this.
		middlewares = append(middlewares, middleware.DisableClientCache())
		middlewares = append(middlewares, middleware.NewCORS(middleware.CORSOptions{
			AllowMethods:        []string{"GET"},
			MaxAge:              -1,
			AllowPrivateNetwork: true,
		}))
	}

	// Create middlewares with authorization
	middlewaresWithAuth := make([]webserver.HandlerFunc, len(middlewares))
	copy(middlewaresWithAuth, middlewares)
	if len(opts.AccessToken) > 0 {
		token := []byte(opts.AccessToken)
		middlewaresWithAuth = append(middlewaresWithAuth, middleware.NewAuth(middleware.AuthOptions{
			ValidateHandler: func(context *webserver.RequestContext, requestToken []byte) (bool, error) {
				return subtle.ConstantTimeCompare(token, requestToken) != 0, nil
			},
		}))
	}

	// Add health handler to web server
	m := middlewares
	if opts.RequestAccessTokenInHealth {
		m = middlewaresWithAuth
	}

	// Add health handler to web server
	if len(opts.HealthApiPath) > 0 {
		path, err = util.SanitizeUrlPath(opts.HealthApiPath, -1)
		if err != nil {
			mws.Stop()
			return nil, fmt.Errorf("invalid HealthApiPath option [err=%v]", err)
		}
	} else {
		path = "/health"
	}
	mws.server.GET(path, mws.getHealthHandler(), m...)

	// Add metrics handler to web server
	if len(opts.MetricsApiPath) > 0 {
		path, err = util.SanitizeUrlPath(opts.MetricsApiPath, -1)
		if err != nil {
			mws.Stop()
			return nil, fmt.Errorf("invalid MetricsApiPath option [err=%v]", err)
		}
	} else {
		path = "/metrics"
	}
	mws.server.GET(path, mws.getMetricsHandler(), middlewaresWithAuth...)

	// Add debug profiles handler to web server
	if opts.EnableDebugProfiles {
		if len(opts.DebugProfilesApiPath) > 0 {
			path, err = util.SanitizeUrlPath(opts.DebugProfilesApiPath, -1)
			if err != nil {
				mws.Stop()
				return nil, fmt.Errorf("invalid DebugProfilesApiPath option [err=%v]", err)
			}
		} else {
			path = "/debug/pprof"
		}
		mws.server.ServeDebugProfiles(path, middlewaresWithAuth...)
	}

	// Done
	return &mws, nil
}

// Start starts the monitor's internal web server
func (mws *Controller) Start() error {
	if mws.server == nil {
		return errors.New("metrics monitor web server not initialized")
	}
	if !mws.usingInternalServer {
		return errors.New("cannot start an external web server")
	}
	return mws.server.Start()
}

// Stop destroys the monitor and stops the internal web server
func (mws *Controller) Stop() {
	// Initiate shutdown
	mws.rp.Wait()

	// Cleanup
	if mws.server != nil {
		// Stop the internal web server if running
		if mws.usingInternalServer {
			mws.server.Stop()
		}
		mws.server = nil
	}
	mws.registry = nil
	mws.healthCallback = nil
}

// Registry returns the prometheus registry object
func (mws *Controller) Registry() *prometheus.Registry {
	return mws.registry
}
