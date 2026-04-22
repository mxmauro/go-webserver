// See the LICENSE file for license details.

package go_webserver_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	webserver "github.com/mxmauro/go-webserver/v2"
	"github.com/mxmauro/go-webserver/v2/internal/testcommon"
)

// -----------------------------------------------------------------------------

func TestWebServerStartStopRestart(t *testing.T) {
	srv, err := webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3000,
	})
	if err != nil {
		t.Fatalf("unable to create web server [%v]", err)
	}

	srv.GET("/api/version", func(req *webserver.RequestContext) error {
		req.Success()
		return nil
	})

	for idx := 0; idx < 2; idx++ {
		err = srv.Start()
		if err != nil {
			t.Fatalf("unable to start web server on cycle %d [%v]", idx, err)
		}

		_, _, err = testcommon.QueryApiVersion(false, nil, nil, []int{200})
		if err != nil {
			t.Fatalf("unable to query api on cycle %d [%v]", idx, err)
		}

		srv.Stop()
	}
}

func TestWebServerStopWhenNotRunning(t *testing.T) {
	srv, err := webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3101,
	})
	if err != nil {
		t.Fatalf("unable to create web server [%v]", err)
	}

	srv.Stop()

	err = srv.Start()
	if err != nil {
		t.Fatalf("unable to start web server after stop while not running [%v]", err)
	}

	srv.Stop()
	srv.Stop()
}

func TestWebServerStartWhenRunningFails(t *testing.T) {
	srv, err := webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3102,
	})
	if err != nil {
		t.Fatalf("unable to create web server [%v]", err)
	}

	err = srv.Start()
	if err != nil {
		t.Fatalf("unable to start web server [%v]", err)
	}
	defer srv.Stop()

	err = srv.Start()
	if err == nil {
		t.Fatalf("expected start while running to fail")
	}
}

func TestWebServerListenerCreationFailureKeepsStateSane(t *testing.T) {
	srv1, err := webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3103,
	})
	if err != nil {
		t.Fatalf("unable to create first web server [%v]", err)
	}
	err = srv1.Start()
	if err != nil {
		t.Fatalf("unable to start first web server [%v]", err)
	}
	defer srv1.Stop()

	srv2, err := webserver.Create(webserver.Options{
		Address: "127.0.0.1",
		Port:    3103,
	})
	if err != nil {
		t.Fatalf("unable to create second web server [%v]", err)
	}

	err = srv2.Start()
	if err == nil {
		t.Fatalf("expected second server start to fail while port is in use")
	}
	if strings.Contains(err.Error(), "server is not stopped") {
		t.Fatalf("unexpected internal state error after listener creation failure [%v]", err)
	}

	err = srv2.Start()
	if err == nil {
		t.Fatalf("expected repeated second server start to fail while port is still in use")
	}
	if strings.Contains(err.Error(), "server is not stopped") {
		t.Fatalf("unexpected internal state error on repeated start after listener creation failure [%v]", err)
	}
}

func TestWebServerConcurrentStopDoesNotDeadlock(t *testing.T) {
	releaseCh := make(chan struct{})
	enteredCh := make(chan struct{}, 1)

	srv := testcommon.RunWebServer(t, func(srv *webserver.Server) error {
		srv.GET("/block", func(req *webserver.RequestContext) error {
			enteredCh <- struct{}{}
			<-releaseCh
			req.Success()
			return nil
		})

		return nil
	})
	defer srv.Stop()

	errCh := make(chan error, 1)
	go func() {
		_, _, _, err := testcommon.Query("GET", "/block", nil, nil, nil, []int{200})
		errCh <- err
	}()

	select {
	case <-enteredCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("timeout waiting for blocked handler to start")
	}

	stopDoneCh := make(chan struct{}, 2)
	var wg sync.WaitGroup

	for idx := 0; idx < 2; idx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			srv.Stop()
			stopDoneCh <- struct{}{}
		}()
	}

	close(releaseCh)

	wg.Wait()
	close(stopDoneCh)

	stopCalls := 0
	for range stopDoneCh {
		stopCalls++
	}
	if stopCalls != 2 {
		t.Fatalf("unexpected number of completed stop calls [got:%v / expected:%v]", stopCalls, 2)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("unable to complete blocked request [%v]", err)
	}
}
