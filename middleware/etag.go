// See the LICENSE file for license details.

package middleware

import (
	"bytes"
	"hash/crc32"
	"net/http"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/util"
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

var (
	weakETagPrefix = []byte("W/")

	etagCrc32Table = crc32.MakeTable(0xD5828281)
)

// -----------------------------------------------------------------------------

// NewETag creates a middleware that adds/checks etags
func NewETag(weak bool) webserver.HandlerFunc {
	// Setup middleware function
	return func(req *webserver.RequestContext) error {
		var buf [2 + 1 + 24 + 1 + 24 + 1 + 64]byte

		// Go to the next middleware first
		err := req.Next()
		if err != nil {
			return err
		}

		// Generate ETags only on successful responses
		if req.Response().StatusCode() != http.StatusOK {
			return nil
		}

		// Do not generate a new ETag if the header is already present
		if req.RequestHeaders().PeekBytes(util.HeaderETag) != nil {
			return nil
		}

		// Do not generate ETag if no response body is available
		body := req.Response().Body()
		if len(body) == 0 {
			return nil
		}

		// Calculate ETag
		bufOfs := 0
		if weak {
			// Add a weak tag
			buf[0] = 'W'
			buf[1] = '/'
			bufOfs = 2
		}
		buf[bufOfs] = '"'
		bufOfs += 1
		bufOfs += fastUint2Bytes(buf[bufOfs:], uint64(len(body)))
		buf[bufOfs] = '-'
		bufOfs += 1
		bufOfs += fastUint2Bytes(buf[bufOfs:], uint64(crc32.Checksum(body, etagCrc32Table)))
		buf[bufOfs] = '"'
		bufOfs += 1

		etag := buf[:bufOfs]

		// Get ETag header from request
		clientETag := req.RequestHeaders().PeekBytes(util.HeaderIfNoneMatch)
		if len(clientETag) > 0 {
			// Is client's ETag is weak?
			if bytes.HasPrefix(clientETag, weakETagPrefix) {
				// Is server's ETag is weak?
				if bytes.Equal(clientETag[2:], etag) || bytes.Equal(clientETag[2:], etag[2:]) {
					// Tag is the same
					req.NoContent(fasthttp.StatusNotModified)
				} else {
					// Tag is different, add it
					req.ResponseHeaders().SetBytesKV(util.HeaderETag, etag)
				}

				// Done
				return nil
			}

			if bytes.Contains(clientETag, etag) {
				// Tag is the same
				req.NoContent(fasthttp.StatusNotModified)

				// Done
				return nil
			}
		}

		// ETag not present or different
		req.ResponseHeaders().SetBytesKV(util.HeaderETag, etag)

		// Done
		return nil
	}
}
