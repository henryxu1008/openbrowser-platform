package launchcode

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestLaunchServerStopBeforeServeClosesOwnedListenerAndKeepsGeneration(t *testing.T) {
	s := NewLaunchServer(nil, nil, nil, 0)
	type dispatch struct {
		server *http.Server
		ln     net.Listener
	}
	entered := make(chan dispatch, 1)
	release := make(chan struct{})
	resume := sync.OnceFunc(func() { close(release) })
	t.Cleanup(resume)
	done := make(chan error, 1)
	// Hold Serve before it can register the listener with http.Server. This
	// deterministically creates the formerly timing-dependent Start/Stop gap.
	if err := s.startWithServe(func(server *http.Server, ln net.Listener) error {
		entered <- dispatch{server, ln}
		<-release
		err := server.Serve(ln)
		done <- err
		return err
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	var old dispatch
	select {
	case old = <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Serve dispatch did not arrive")
	}
	oldPort := s.Port()
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal("second Stop was not idempotent")
	}
	if err := s.Start(); err != nil {
		t.Fatalf("old listener remained bound before Serve: %v", err)
	}
	if s.Port() != oldPort || s.server == old.server {
		t.Fatal("restart lost its fixed port or reused the old server")
	}
	// Release exactly once while keeping the cleanup safe on earlier failures.
	resume()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("old generation did not remain shut down: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("old generation did not finish")
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Port())), 5*time.Second)
	if err != nil {
		t.Fatalf("old generation disturbed the replacement: %v", err)
	}
	_ = conn.Close()
}
