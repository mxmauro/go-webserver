// See the LICENSE file for license details.

package middleware

import (
	"bytes"
	"fmt"
	"strings"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/util"
)

// -----------------------------------------------------------------------------

// SafeBrowsingOptions defines how common response headers for safe browsing are added.
type SafeBrowsingOptions struct {
	// XXSSProtection sets the `X-XSS-Protection` header to stop pages from loading when they detect reflected
	// cross-site scripting (XSS) attacks.
	// Optional. Defaults to "1; mode=block".
	XXSSProtection string `json:"x-xss-protection,omitempty"`

	// XContentTypeNoSniff sets the `X-Content-Type-Options` header to indicate that the MIME types advertised
	// in the Content-Type headers should be followed and not be changed.
	// Optional. Defaults to "nosniff".
	XContentTypeNoSniff string `json:"x-content-type-options,omitempty"`

	// XFrameOptions can be used to indicate whether a browser should be allowed to render a page in a <frame>,
	// <iframe> or <object>.
	// Optional. Defaults to "sameorigin".
	// Possible values: "sameorigin", "deny", "allow-from uri"
	XFrameOptions string `json:"x-frame-options,omitempty"`

	// HSTS controls the `Strict-Transport-Security` header to inform browsers that the site should only be
	// accessed using HTTPS and that any future attempts to access it using HTTP should automatically be
	// converted to HTTPS.
	HSTS struct {
		// MaxAge establishes the time, in seconds, that the browser should remember that a site is only to be
		// accessed using HTTPS.
		// Optional. Defaults to 0.
		MaxAge uint `json:"max-age,omitempty"`

		// IncludeSubdomains is used to apply the HSTS settings to all the site's subdomains as well. Optional.
		IncludeSubdomains bool `json:"include-subdomains,omitempty"`

		// Preload will add the preload tag in the HSTS header. See https://hstspreload.org/ for details. Optional.
		Preload bool `json:"preload,omitempty"`
	} `json:"hsts,omitempty"`

	// ContentSecurityPolicy sets the `Content-Security-Policy` header to enhance security against XSS. Optional.
	ContentSecurityPolicy string `json:"content-security-policy,omitempty"`

	// ContentSecurityPolicyReportOnly would use the `Content-Security-Policy-Report-Only` header instead
	// of the `Content-Security-Policy` header. Used to report violations instead of blocking resources. Optional.
	ContentSecurityPolicyReportOnly bool `json:"csp-report-only,omitempty"`

	// ReferrerPolicy sets the `Referrer-Policy` header providing security against leaking potentially sensitive
	// request paths to third parties. Optional.
	ReferrerPolicy string `json:"referrer-policy,omitempty"`
}

// -----------------------------------------------------------------------------

// DefaultSafeBrowsing creates default SafeBrowsing middleware with commonly used options
func DefaultSafeBrowsing() webserver.HandlerFunc {
	return NewSafeBrowsing(SafeBrowsingOptions{
		XXSSProtection:      "1; mode=block",
		XContentTypeNoSniff: "nosniff",
		XFrameOptions:       "sameorigin",
	})
}

// NewSafeBrowsing creates new SafeBrowsing middleware based on the specified options
func NewSafeBrowsing(opts SafeBrowsingOptions) webserver.HandlerFunc {
	var xXssProtection []byte
	var xContentTypeNoSniff []byte
	var xFrameOptions []byte
	var contentSecurityPolicy []byte
	var referrerPolicy []byte

	if len(opts.XXSSProtection) > 0 {
		xXssProtection = util.UnsafeString2ByteSlice(opts.XXSSProtection)
	}
	if len(opts.XContentTypeNoSniff) > 0 {
		xContentTypeNoSniff = util.UnsafeString2ByteSlice(opts.XContentTypeNoSniff)
	}
	if len(opts.XFrameOptions) > 0 {
		xFrameOptions = util.UnsafeString2ByteSlice(opts.XFrameOptions)
	}
	if len(opts.ContentSecurityPolicy) > 0 {
		contentSecurityPolicy = util.UnsafeString2ByteSlice(opts.ContentSecurityPolicy)
	}
	if len(opts.ReferrerPolicy) > 0 {
		referrerPolicy = util.UnsafeString2ByteSlice(opts.ReferrerPolicy)
	}

	// Setup middleware function
	return func(req *webserver.RequestContext) error {
		respHdrs := req.ResponseHeaders()

		// Set X-XSS-Protection header
		if xXssProtection != nil {
			respHdrs.SetBytesKV(util.HeaderXXSSProtection, xXssProtection)
		}

		// Set X-Content-Type-Options header
		if xContentTypeNoSniff != nil {
			respHdrs.SetBytesKV(util.HeaderXContentTypeOptions, xContentTypeNoSniff)
		}

		// Set X-Frame-Options header
		if len(opts.XFrameOptions) > 0 {
			respHdrs.SetBytesKV(util.HeaderXFrameOptions, xFrameOptions)
		}

		// Set Strict-Transport-Security header
		if opts.HSTS.MaxAge > 0 &&
			(req.IsTLS() ||
				bytes.Equal(req.RequestHeaders().PeekBytes(util.HeaderXForwardedProto), util.BytesHttp)) {
			sb := strings.Builder{}
			_, _ = sb.WriteString(fmt.Sprintf("max-age=%d", opts.HSTS.MaxAge))
			if opts.HSTS.IncludeSubdomains {
				_, _ = sb.WriteString("; includeSubdomains")
			}
			if opts.HSTS.Preload {
				_, _ = sb.WriteString("; preload")
			}
			respHdrs.SetBytesK(util.HeaderStrictTransportSecurity, sb.String())
		}

		// Set the "Content-Security-Policy" or "Content-Security-Policy-Report-Only" header
		if contentSecurityPolicy != nil {
			if opts.ContentSecurityPolicyReportOnly {
				respHdrs.SetBytesKV(util.HeaderContentSecurityPolicyReportOnly, contentSecurityPolicy)
			} else {
				respHdrs.SetBytesKV(util.HeaderContentSecurityPolicy, contentSecurityPolicy)
			}
		}

		// Set Referrer-Policy header
		if referrerPolicy != nil {
			respHdrs.SetBytesKV(util.HeaderReferrerPolicy, referrerPolicy)
		}

		// Go to next middleware
		return req.Next()
	}
}
