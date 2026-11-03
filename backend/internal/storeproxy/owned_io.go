package storeproxy

import (
	"context"
	"net"
	"net/http"
	"sync"
)

// http.Transport may return before a canceled dial/CONNECT has unwound. Own the
// actual dial, connection and I/O capabilities; seal them before joining cleanup.
// Late transport bookkeeping cannot start another socket or use an old one.
type ownedIO struct {
	mu          sync.Mutex
	changed     *sync.Cond
	sealed      bool
	active      int
	connections map[*ownedConn]struct{}
	ctx         context.Context
	cancel      context.CancelFunc
}

func newOwnedIO() *ownedIO {
	ctx, cancel := context.WithCancel(context.Background())
	o := &ownedIO{ctx: ctx, cancel: cancel, connections: map[*ownedConn]struct{}{}}
	o.changed = sync.NewCond(&o.mu)
	return o
}
func (o *ownedIO) begin() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.sealed {
		return false
	}
	o.active++
	return true
}
func (o *ownedIO) end() { o.mu.Lock(); o.active--; o.changed.Broadcast(); o.mu.Unlock() }
func (o *ownedIO) dial(ctx context.Context, network, address string, next func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	if !o.begin() {
		return nil, net.ErrClosed
	}
	defer o.end()
	ctx, cancel := context.WithCancel(ctx)
	canceled := make(chan struct{})
	stop := context.AfterFunc(o.ctx, func() { cancel(); close(canceled) })
	defer func() {
		if !stop() {
			<-canceled
		}
		cancel()
	}()
	connection, err := next(ctx, network, address)
	if connection == nil {
		return nil, err
	}
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	o.mu.Lock()
	if o.sealed {
		o.mu.Unlock()
		_ = connection.Close()
		return nil, net.ErrClosed
	}
	c := &ownedConn{Conn: connection, owner: o, done: make(chan struct{})}
	o.connections[c] = struct{}{}
	o.mu.Unlock()
	return c, nil
}
func (o *ownedIO) closeAndWait(transport *http.Transport) {
	o.mu.Lock()
	o.sealed = true
	connections := make([]*ownedConn, 0, len(o.connections))
	for c := range o.connections {
		connections = append(connections, c)
	}
	o.mu.Unlock()
	o.cancel()
	transport.CloseIdleConnections()
	for _, c := range connections {
		_ = c.Close()
	}
	o.mu.Lock()
	for o.active != 0 {
		o.changed.Wait()
	}
	o.mu.Unlock()
}

type ownedConn struct {
	net.Conn
	owner    *ownedIO
	once     sync.Once
	done     chan struct{}
	closeErr error
}

func (c *ownedConn) Read(p []byte) (int, error) {
	if !c.owner.begin() {
		return 0, net.ErrClosed
	}
	defer c.owner.end()
	return c.Conn.Read(p)
}
func (c *ownedConn) Write(p []byte) (int, error) {
	if !c.owner.begin() {
		return 0, net.ErrClosed
	}
	defer c.owner.end()
	return c.Conn.Write(p)
}
func (c *ownedConn) Close() error {
	c.once.Do(func() {
		c.closeErr = c.Conn.Close()
		c.owner.mu.Lock()
		delete(c.owner.connections, c)
		c.owner.mu.Unlock()
		close(c.done)
	})
	<-c.done
	return c.closeErr
}
