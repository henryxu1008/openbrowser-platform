package storeproxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type delayedIOConn struct {
	net.Conn
	readStarted, readCleanup chan struct{}
	writes                   atomic.Int32
	closed                   atomic.Bool
}

func (c *delayedIOConn) Read([]byte) (int, error) {
	close(c.readStarted)
	<-c.readCleanup
	return 0, net.ErrClosed
}
func (c *delayedIOConn) Write(p []byte) (int, error) { c.writes.Add(1); return len(p), nil }
func (c *delayedIOConn) Close() error                { c.closed.Store(true); return nil }

func TestOwnedIOSealsLateActivityAndJoinsDelayedRead(t *testing.T) {
	owner := newOwnedIO()
	raw := &delayedIOConn{readStarted: make(chan struct{}), readCleanup: make(chan struct{})}
	connection, e := owner.dial(context.Background(), "tcp", "fixture", func(context.Context, string, string) (net.Conn, error) { return raw, nil })
	if e != nil {
		t.Fatal(e)
	}
	readDone := make(chan struct{})
	go func() { defer close(readDone); _, _ = connection.Read(make([]byte, 1)) }()
	<-raw.readStarted
	joined := make(chan struct{})
	go func() { defer close(joined); owner.closeAndWait(&http.Transport{}) }()
	<-owner.ctx.Done()
	select {
	case <-joined:
		t.Error("cleanup acknowledged while socket read still active")
	case <-time.After(20 * time.Millisecond):
	}
	if _, e = connection.Write([]byte("late")); !errors.Is(e, net.ErrClosed) || raw.writes.Load() != 0 {
		t.Error("late transport I/O escaped seal")
	}
	if _, e = owner.dial(context.Background(), "tcp", "fixture", func(context.Context, string, string) (net.Conn, error) {
		t.Error("late dial escaped seal")
		return nil, nil
	}); !errors.Is(e, net.ErrClosed) {
		t.Error(e)
	}
	close(raw.readCleanup)
	<-readDone
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("read cleanup did not join")
	}
	if !raw.closed.Load() {
		t.Fatal("owned socket not closed")
	}
}

func TestOwnedIOClosesDialConnectionReturnedAfterCancellation(t *testing.T) {
	owner := newOwnedIO()
	started, canceled, cleanup := make(chan struct{}), make(chan struct{}), make(chan struct{})
	raw := &delayedIOConn{}
	dialDone := make(chan struct{})
	go func() {
		defer close(dialDone)
		connection, e := owner.dial(context.Background(), "tcp", "fixture", func(ctx context.Context, _, _ string) (net.Conn, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			<-cleanup
			return raw, nil
		})
		if connection != nil || !errors.Is(e, net.ErrClosed) {
			t.Error("late connection was handed to transport")
		}
	}()
	<-started
	joined := make(chan struct{})
	go func() { defer close(joined); owner.closeAndWait(&http.Transport{}) }()
	<-canceled
	select {
	case <-joined:
		t.Error("dial cleanup not joined")
	case <-time.After(20 * time.Millisecond):
	}
	close(cleanup)
	<-dialDone
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("late dial cleanup did not join")
	}
	if !raw.closed.Load() {
		t.Fatal("late connection left alive")
	}
}
