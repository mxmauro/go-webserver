//go:build !windows

package go_webserver

import (
	"net"

	"github.com/valyala/fasthttp/reuseport"
)

// -----------------------------------------------------------------------------

func createListener(network string, address string) (net.Listener, error) {
	return reuseport.Listen(network, address)
}
