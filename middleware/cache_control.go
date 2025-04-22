package middleware

import (
	"fmt"
	"strings"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/util"
)

// -----------------------------------------------------------------------------

// CacheControlOptions defines the behavior on how Cache-Control headers are sent.
type CacheControlOptions struct {
	Public                        bool
	Private                       bool
	NoCache                       bool
	NoStore                       bool
	NoTransform                   bool
	MustRevalidate                bool
	ProxyRevalidate               bool
	MaxAgeInSeconds               *uint32
	SharedMaxAgeInSeconds         *uint32
	StaleWhileRevalidateInSeconds *uint32
	StaleIfErrorInSeconds         *uint32
}

// -----------------------------------------------------------------------------

// DisableClientCache creates a default cache control middleware that disables the client's cache
func DisableClientCache() webserver.HandlerFunc {
	var zero uint32

	return NewCacheControl(CacheControlOptions{
		Private:               true,
		NoCache:               true,
		NoStore:               true,
		MustRevalidate:        true,
		ProxyRevalidate:       true,
		MaxAgeInSeconds:       &zero,
		SharedMaxAgeInSeconds: &zero,
	})
}

// NewCacheControl creates new client cache control middleware based on the specified options
func NewCacheControl(opts CacheControlOptions) webserver.HandlerFunc {
	var finalCacheValue []byte

	// Build header content
	sb := strings.Builder{}
	if opts.Public {
		_, _ = sb.WriteString(",public")
	}
	if opts.Private {
		_, _ = sb.WriteString(",private")
	}

	if opts.NoCache {
		_, _ = sb.WriteString(",no-cache")
	}
	if opts.NoStore {
		_, _ = sb.WriteString(",no-store")
	}
	if opts.NoTransform {
		_, _ = sb.WriteString(",no-transform")
	}

	if opts.MustRevalidate {
		_, _ = sb.WriteString(",must-revalidate")
	}
	if opts.ProxyRevalidate {
		_, _ = sb.WriteString(",proxy-revalidate")
	}

	if opts.MaxAgeInSeconds != nil {
		_, _ = sb.WriteString(fmt.Sprintf(",max-age=%v", *opts.MaxAgeInSeconds))
	}
	if opts.SharedMaxAgeInSeconds != nil {
		_, _ = sb.WriteString(fmt.Sprintf(",s-maxage=%v", *opts.SharedMaxAgeInSeconds))
	}
	if opts.StaleWhileRevalidateInSeconds != nil {
		_, _ = sb.WriteString(fmt.Sprintf(",stale-if-error=%v", *opts.StaleWhileRevalidateInSeconds))
	}
	if opts.StaleIfErrorInSeconds != nil {
		_, _ = sb.WriteString(fmt.Sprintf(",stale-while-revalidate=%v", *opts.StaleIfErrorInSeconds))
	}

	if sb.Len() > 0 {
		finalCacheValue = util.UnsafeString2ByteSlice(sb.String())[1:]
	} else {
		finalCacheValue = make([]byte, 0)
	}

	// Setup middleware function
	return func(req *webserver.RequestContext) error {
		// Set cache control header
		req.ResponseHeaders().SetBytesKV(util.HeaderCacheControl, finalCacheValue)

		// Go to next middleware
		return req.Next()
	}
}

func CacheControlMaxAge(seconds uint32) *uint32 {
	return &seconds
}
