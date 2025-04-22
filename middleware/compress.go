package middleware

import (
	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

type CompressionLevel int

const (
	CompressionLevelDefault    CompressionLevel = 0
	CompressionLevelFavorSpeed CompressionLevel = 1
	CompressionLevelFavorSize  CompressionLevel = 2
)

// -----------------------------------------------------------------------------

// NewCompression creates a middleware that compresses the output
func NewCompression(level CompressionLevel) webserver.HandlerFunc {
	var compressor fasthttp.RequestHandler

	dummyHandler := func(_ *fasthttp.RequestCtx) {}

	// Setup compression algorithm
	switch level {
	case CompressionLevelFavorSpeed:
		compressor = fasthttp.CompressHandlerBrotliLevel(
			dummyHandler, fasthttp.CompressBrotliBestSpeed, fasthttp.CompressBestSpeed,
		)

	case CompressionLevelFavorSize:
		compressor = fasthttp.CompressHandlerBrotliLevel(
			dummyHandler, fasthttp.CompressBrotliBestCompression, fasthttp.CompressBestCompression,
		)

	case CompressionLevelDefault:
		fallthrough
	default:
		compressor = fasthttp.CompressHandlerBrotliLevel(
			dummyHandler, fasthttp.CompressBrotliDefaultCompression, fasthttp.CompressDefaultCompression,
		)
	}

	// Setup middleware function
	return func(req *webserver.RequestContext) error {
		// Go to the next middleware first
		err := req.Next()
		if err != nil {
			return err
		}

		// Compress
		req.CallFastHttpHandler(compressor)

		// Done
		return nil
	}
}
