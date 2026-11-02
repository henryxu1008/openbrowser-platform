package storeproxy

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrBusy = errors.New("store_proxy_check_busy")
var ErrCanceled = errors.New("store_proxy_check_canceled")
var ErrReplay = errors.New("store_proxy_check_replay")
var ErrCapacity = errors.New("store_proxy_check_capacity")

// Owner is derived by native code, never accepted from renderer identity data.
// Session includes trusted issuer/subject/generation; Store gates across remounts.
type Owner struct {
	Session, Store string
	Revision       uint64
}
type operationKey struct {
	Owner Owner
	ID    string
}

// A Store retains its gate even if a replacement login remounts the renderer.
type gateKey struct{ Store string }
type history struct {
	at       time.Time
	canceled bool
}
type Lease struct {
	Context context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	finish  func()
	acquire func() error
	once    sync.Once
}

func (l *Lease) Done() <-chan struct{} { return l.done }
func (l *Lease) Finish()               { l.once.Do(l.finish) }
func (l *Lease) Acquire() error        { return l.acquire() }

// Zero value is ready. Cancellation never releases a gate; only the worker's
// Finish, after all owned cleanup, acknowledges settlement.
type Manager struct {
	mu       sync.Mutex
	active   map[gateKey]*Lease
	sessions map[string]*Lease
	keys     map[operationKey]*Lease
	history  map[operationKey]history
}

func (m *Manager) init() {
	if m.active == nil {
		m.active = map[gateKey]*Lease{}
		m.sessions = map[string]*Lease{}
		m.keys = map[operationKey]*Lease{}
		m.history = map[operationKey]history{}
	}
	for k, v := range m.history {
		if time.Since(v.at) > 5*time.Minute {
			delete(m.history, k)
		}
	}
}
func (m *Manager) Reserve(parent context.Context, owner Owner, id string) (*Lease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	key := operationKey{owner, id}
	gate := gateKey{owner.Store}
	if h, ok := m.history[key]; ok {
		if h.canceled {
			return nil, ErrCanceled
		}
		return nil, ErrReplay
	}
	if _, ok := m.keys[key]; ok {
		return nil, ErrBusy
	}
	if len(m.history)+len(m.keys) >= 512 {
		return nil, ErrCapacity
	}
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	l := &Lease{Context: ctx, cancel: cancel, done: make(chan struct{})}
	l.acquire = func() error {
		m.mu.Lock()
		defer m.mu.Unlock()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if current, ok := m.active[gate]; ok && current != l {
			return ErrBusy
		}
		if current, ok := m.sessions[owner.Session]; ok && current != l {
			return ErrBusy
		}
		m.active[gate] = l
		m.sessions[owner.Session] = l
		return nil
	}
	l.finish = func() {
		cancel()
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.active[gate] == l {
			delete(m.active, gate)
		}
		if m.sessions[owner.Session] == l {
			delete(m.sessions, owner.Session)
		}
		delete(m.keys, key)
		m.history[key] = history{at: time.Now()}
		close(l.done)
	}
	m.keys[key] = l
	return l, nil
}
func (m *Manager) Begin(parent context.Context, owner Owner, id string) (*Lease, error) {
	l, err := m.Reserve(parent, owner, id)
	if err != nil {
		return nil, err
	}
	if err = l.Acquire(); err != nil {
		l.Finish()
		return nil, err
	}
	return l, nil
}
func (m *Manager) Cancel(owner Owner, id string) (<-chan struct{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.init()
	key := operationKey{owner, id}
	if l, ok := m.keys[key]; ok {
		l.cancel()
		return l.done, nil
	}
	if _, ok := m.history[key]; !ok {
		if len(m.history)+len(m.keys) >= 512 {
			return nil, ErrCapacity
		}
		m.history[key] = history{at: time.Now(), canceled: true}
	}
	// A cancellation arriving before Begin is a tombstone, not a future grant.
	done := make(chan struct{})
	close(done)
	return done, nil
}
