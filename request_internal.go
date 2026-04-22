// See the LICENSE file for license details.

package go_webserver

import (
	"net"
	"net/netip"

	"github.com/valyala/fasthttp"
)

// -----------------------------------------------------------------------------

const (
	reqContextLinkKey = "\xFF\xFF**reqContextLinkKey"
)

// -----------------------------------------------------------------------------

func (req *RequestContext) sendError(statusCode int, msg string) {
	if !req.IsHead() {
		if len(msg) == 0 {
			msg = fasthttp.StatusMessage(statusCode)
		}
		req.Error(msg, statusCode)
	} else {
		req.Error("", statusCode)
	}
}

func (req *RequestContext) isProxyTrusted() bool {
	if req.tp == nil {
		return false
	}
	return req.tp.IsIpTrusted(req.ctx.RemoteIP())
}

func (req *RequestContext) setHandlerParams(h HandlerFunc, middlewares []HandlerFunc) {
	req.handler = h
	req.middlewares = middlewares
	req.middlewaresLen = len(middlewares)
}

// -----------------------------------------------------------------------------

func getFirstIpAddress(header []byte) net.IP {
	ofs := 0
	l := len(header)
	for ofs < l {
		// Skip leading spaces.
		for ofs < l && header[ofs] == ' ' {
			ofs += 1
		}

		// Get the address.
		startOfs := ofs
		for ofs < l && header[ofs] != ' ' && header[ofs] != ',' {
			ofs += 1
		}
		endOfs := ofs

		// Skip trailing spaces.
		nonSpaceCharFound := false
		for ofs < l && header[ofs] != ',' {
			if header[ofs] != ' ' {
				nonSpaceCharFound = true
			}
			ofs += 1
		}

		// Skip the comma, if any.
		if ofs < l {
			ofs += 1
		}

		// Process this address.
		if !nonSpaceCharFound && endOfs > startOfs {
			addr, err := netip.ParseAddr(string(header[startOfs:endOfs]))
			if err == nil {
				return addr.AsSlice()
			}

		}
	}

	// Cannot retrieve the first IP address.
	return nil
}
