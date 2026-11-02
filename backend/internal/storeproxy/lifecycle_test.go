package storeproxy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCancellationKeepsStoreGateUntilOwnedCleanupFinishes(t *testing.T) {
	var m Manager
	owner := Owner{"session-a", "shop", 1}
	l, e := m.Begin(context.Background(), owner, "op-a")
	if e != nil {
		t.Fatal(e)
	}
	done, e := m.Cancel(owner, "op-a")
	if e != nil {
		t.Fatal(e)
	}
	select {
	case <-l.Context.Done():
	default:
		t.Fatal("cancel not delivered")
	}
	select {
	case <-done:
		t.Fatal("premature cleanup acknowledgement")
	default:
	}
	for _, replacement := range []Owner{{"session-a", "shop", 2}, {"session-b", "shop", 1}} {
		if _, e = m.Begin(context.Background(), replacement, "op-b"); !errors.Is(e, ErrBusy) {
			t.Fatal("replacement escaped pending gate", e)
		}
	}
	l.Finish()
	select {
	case <-done:
	default:
		t.Fatal("cleanup not acknowledged")
	}
	if _, e = m.Begin(context.Background(), owner, "op-a"); !errors.Is(e, ErrReplay) {
		t.Fatal("replayed operation executed", e)
	}
	next, e := m.Begin(context.Background(), owner, "op-b")
	if e != nil {
		t.Fatal(e)
	}
	next.Finish()
}
func TestCancelCannotAdoptAnotherOwnerOrOperation(t *testing.T) {
	var m Manager
	a := Owner{"session", "shop", 1}
	l, _ := m.Begin(context.Background(), a, "one")
	for _, o := range []Owner{{"other", "shop", 1}, {"session", "other", 1}, {"session", "shop", 2}} {
		d, e := m.Cancel(o, "one")
		if e != nil {
			t.Fatal(e)
		}
		<-d
		if l.Context.Err() != nil {
			t.Fatal("foreign cancel adopted worker")
		}
	}
	_, _ = m.Cancel(a, "other")
	if l.Context.Err() != nil {
		t.Fatal("operation key ignored")
	}
	l.Finish()
}
func TestCancelBeforeDispatchPreventsLaterStart(t *testing.T) {
	var m Manager
	o := Owner{"s", "shop", 1}
	d, e := m.Cancel(o, "id")
	if e != nil {
		t.Fatal(e)
	}
	<-d
	if _, e = m.Begin(context.Background(), o, "id"); !errors.Is(e, ErrCanceled) {
		t.Fatal("cancel raced dispatch", e)
	}
}
func TestParentExpiryDoesNotReleaseGateBeforeSettlement(t *testing.T) {
	var m Manager
	ctx, cancel := context.WithCancel(context.Background())
	o := Owner{"s", "shop", 1}
	l, _ := m.Begin(ctx, o, "id")
	deadline, ok := l.Context.Deadline()
	if !ok || time.Until(deadline) > 12*time.Second {
		t.Fatal("missing absolute bound")
	}
	cancel()
	if _, e := m.Begin(context.Background(), o, "next"); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	l.Finish()
}
func TestSessionGateAlsoBoundsDifferentStores(t *testing.T) {
	var m Manager
	l, e := m.Begin(context.Background(), Owner{"session", "shop-a", 1}, "one")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Begin(context.Background(), Owner{"session", "shop-b", 1}, "two"); !errors.Is(e, ErrBusy) {
		t.Fatal("session ran concurrent Store probes", e)
	}
	l.Finish()
	next, e := m.Begin(context.Background(), Owner{"session", "shop-b", 1}, "three")
	if e != nil {
		t.Fatal(e)
	}
	next.Finish()
}
