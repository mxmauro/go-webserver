package middleware

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"strings"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/util"
)

// -----------------------------------------------------------------------------

var (
	ErrNoKeyProvided = errors.New("no key provided")
	ErrNotAuthorized = errors.New("not authorized")
)

// -----------------------------------------------------------------------------

// AuthErrorHandler defines a function to call when authorization fails.
type AuthErrorHandler func(req *webserver.RequestContext, err error) error

// AuthValidatorFunc defines a function that verifies whether the given key is valid.
type AuthValidatorFunc func(*webserver.RequestContext, []byte) (bool, error)

// AuthOptions defines an authorization check.
type AuthOptions struct {
	// ErrorHandler defines a handler to execute if authorization fails. If not defined, will return 401 if err is nil
	// else 500.
	ErrorHandler AuthErrorHandler

	// DisableAuthorizationBearer disables inspection of the Authorization Bearer header.
	// NOTE: This check has priority over the rest.
	DisableAuthorizationBearer bool

	// If HeaderName is defined, the value of that header will be used as the key if present.
	// NOTE: HeaderName is evaluated if Authorization Bearer is not present/checked.
	HeaderName string

	// If QueryName is defined, the value of that query parameter will be used as the key if present.
	// NOTE: QueryName is evaluated after the above.
	QueryName string

	// If CookieName is defined, the value of that cookie will be used as the key if present.
	// NOTE: CookieName is evaluated after the above.
	CookieName string

	// ValidateHandler is a function to validate a key.
	ValidateHandler AuthValidatorFunc
}

// -----------------------------------------------------------------------------

// NewAuth wraps a middleware that verifies authentication.
func NewAuth(opts AuthOptions) webserver.HandlerFunc {
	var headerName []byte
	var cookieName []byte
	var queryName []byte

	if opts.ValidateHandler == nil {
		opts.ValidateHandler = func(_ *webserver.RequestContext, _ []byte) (bool, error) {
			return true, nil
		}
	}
	if opts.ErrorHandler == nil {
		opts.ErrorHandler = func(req *webserver.RequestContext, err error) error {
			req.Unauthorized(err.Error())
			return nil
		}
	}

	if len(opts.HeaderName) > 0 {
		headerName = util.UnsafeString2ByteSlice(http.CanonicalHeaderKey(opts.HeaderName))
	}
	if len(opts.CookieName) > 0 {
		cookieName = util.UnsafeString2ByteSlice(opts.CookieName)
	}
	if len(opts.QueryName) > 0 {
		queryName = util.UnsafeString2ByteSlice(opts.QueryName)
	}

	// Set up the middleware function.
	return func(req *webserver.RequestContext) error {
		var key []byte
		var err error

		// Locate the key.
		if !opts.DisableAuthorizationBearer {
			key = req.RequestHeaders().PeekBytes(util.HeaderAuthorization)
			if len(key) > 7 && key[6] == ' ' &&
				(key[0]&(^uint8(0x20))) == 'B' &&
				(key[1]&(^uint8(0x20))) == 'E' &&
				(key[2]&(^uint8(0x20))) == 'A' &&
				(key[3]&(^uint8(0x20))) == 'R' &&
				(key[4]&(^uint8(0x20))) == 'E' &&
				(key[5]&(^uint8(0x20))) == 'R' {
				key = bytes.TrimSpace(key[7:])
			} else {
				key = nil
			}
		}

		if len(key) == 0 && headerName != nil {
			key = req.RequestHeaders().PeekBytes(headerName)
			if key != nil {
				key = bytes.TrimSpace(key)
			}
		}

		if len(key) == 0 && queryName != nil {
			key = req.QueryArgs().PeekBytes(queryName)
			if key != nil {
				newKey, err2 := url.QueryUnescape(util.UnsafeByteSlice2String(key))
				if err2 == nil {
					key = util.UnsafeString2ByteSlice(strings.TrimSpace(newKey))
				} else {
					key = nil
				}
			}
		}

		if len(key) == 0 && cookieName != nil {
			key = req.RequestHeaders().CookieBytes(cookieName)
			if key != nil {
				key = bytes.TrimSpace(key)
			}
		}

		// Validate it if a key is found.
		if len(key) == 0 {
			err = ErrNoKeyProvided
		} else {
			var success bool

			success, err = opts.ValidateHandler(req, key)
			if err != nil {
				return err
			}
			if success {
				// Run the next middleware.
				return req.Next()
			}
			err = ErrNotAuthorized
		}

		// Done.
		return opts.ErrorHandler(req, err)
	}
}
