// See the LICENSE file for license details.

package go_webserver

import (
	"context"
	"net"
	"sync/atomic"
	"time"
)

// -----------------------------------------------------------------------------

const (
	stateNotStarted = 1
	stateStarting   = 2
	stateRunning    = 3
	stateStopping   = 4
	stateStopped    = 5
)

const (
	shutdownTimeout = 5 * time.Second
)

// -----------------------------------------------------------------------------

func (srv *Server) serve(ln net.Listener) {
	ch := make(chan error, 1)

	go func(ln net.Listener) {
		ch <- srv.srv.Serve(ln)
	}(ln)

	// Set the new state.
	srv.setState(stateRunning)

	// Run in the background until shutdown or error.
	go srv.serveLoop(ch)
}

func (srv *Server) serveLoop(ch chan error) {
	defer close(srv.shutdownCompleteCh)

	select {
	case err := <-ch:
		srv.setState(stateStopping)

		// The web server can no longer accept connections.
		if srv.listenErrorHandler != nil && err != nil {
			srv.listenErrorHandler(srv, err)
		}

	// Handle the termination signal.
	case <-srv.startShutdownCh:
		srv.setState(stateStopping)

		// Attempt the graceful shutdown by closing the listener and completing all inflight requests.
		ctx, ctxCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		_ = srv.srv.ShutdownWithContext(ctx)
		ctxCancel()
	}

	srv.setState(stateStopped)
}

func (srv *Server) setState(newState int32) {
	atomic.StoreInt32(&srv.state, newState)
}
