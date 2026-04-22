// See the LICENSE file for license details.

package go_webserver

import (
	"context"
)

// -----------------------------------------------------------------------------

// UserContext returns a context.Context previously set by the user. Defaults to context.Background.
func (req *RequestContext) UserContext() context.Context {
	if req.userCtx == nil {
		req.userCtx = context.Background()
	}
	return req.userCtx
}

// SetUserContext sets a user-provided context.
func (req *RequestContext) SetUserContext(ctx context.Context) {
	req.userCtx = ctx
}
