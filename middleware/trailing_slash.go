// See the LICENSE file for license details.

package middleware

import (
	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/util"
)

// -----------------------------------------------------------------------------

// TrailingSlashOptions defines a middleware that adds or removes trailing slashes in paths.
type TrailingSlashOptions struct {
	// Remove tells the middleware to remove trailing slashes if present.
	// If this setting is false, then the trailing slash is added if absent.
	Remove bool

	// RedirectCode, if not zero, will make the middleware to return a redirect response.
	RedirectCode uint
}

// -----------------------------------------------------------------------------

// NewTrailingSlash creates new middleware to handle trailing slashes in request's paths
func NewTrailingSlash(opts TrailingSlashOptions) webserver.HandlerFunc {
	// Setup middleware function
	return func(req *webserver.RequestContext) error {
		uri := req.URI()

		// Check if the path contains (or not) the trailing slash
		modify := 0
		path := uri.Path()
		pathLen := len(path)
		if opts.Remove {
			if pathLen > 1 && (path[pathLen-1] == 47 || path[pathLen-1] == 92) {
				modify = -1
			}
		} else {
			if pathLen == 0 || (path[pathLen-1] != 47 && path[pathLen-1] != 92) {
				modify = 1
			}
		}
		if modify != 0 {
			newPath, err := util.SanitizeUrlPath(util.UnsafeByteSlice2String(path), modify)
			if err != nil {
				req.BadRequest(err.Error())
				return nil
			}
			uri.SetPath(newPath)

			// Redirect
			if opts.RedirectCode != 0 {
				req.Redirect(util.UnsafeByteSlice2String(uri.FullURI()), int(opts.RedirectCode))
				return nil
			}
		}

		// Go to next middleware
		return req.Next()
	}
}
