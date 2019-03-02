package ice

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/pions/stun"
)

// Dial connects to the remote agent, acting as the controlling ice agent.
// Dial blocks until at least one ice candidate pair has successfully connected.
func (a *Agent) Dial(ctx context.Context, remoteUfrag, remotePwd string) (*Conn, error) {
	return a.connect(ctx, true, remoteUfrag, remotePwd)
}

// Accept connects to the remote agent, acting as the controlled ice agent.
// Accept blocks until at least one ice candidate pair has successfully connected.
func (a *Agent) Accept(ctx context.Context, remoteUfrag, remotePwd string) (*Conn, error) {
	return a.connect(ctx, false, remoteUfrag, remotePwd)
}

// Conn represents the ICE connection.
// At the moment the lifetime of the Conn is equal to the Agent.
type Conn struct {
	agent *Agent
}

func (a *Agent) connect(ctx context.Context, isControlling bool, remoteUfrag, remotePwd string) (*Conn, error) {
	err := a.ok()
	if err != nil {
		return nil, err
	}
	if a.opened {
		return nil, errors.New("a connection is already opened")
	}
	err = a.startConnectivityChecks(isControlling, remoteUfrag, remotePwd)
	if err != nil {
		return nil, err
	}

	// block until pair selected
	select {
	case <-ctx.Done():
		// TODO: Stop connectivity checks?
		return nil, errors.New("connecting canceled by caller")
	case <-a.onConnected:
	}

	return &Conn{
		agent: a,
	}, nil

}

// Read implements the Conn Read method.
func (c *Conn) Read(p []byte) (int, error) {
	err := c.agent.ok()
	if err != nil {
		return 0, err
	}

	return c.agent.buffer.Read(p)
}

// Write implements the Conn Write method.
func (c *Conn) Write(p []byte) (int, error) {
	err := c.agent.ok()
	if err != nil {
		return 0, err
	}

	if stun.IsSTUN(p) {
		return 0, errors.New("the ICE conn can't write STUN messages")
	}

	pair, err := c.agent.getBestPair()
	if err != nil {
		return 0, err
	}

	return pair.Write(p)
}

// Close implements the Conn Close method. It is used to close
// the connection. Any calls to Read and Write will be unblocked and return an error.
func (c *Conn) Close() error {
	return c.agent.Close()
}

// TODO: Maybe just switch to using io.ReadWriteCloser?

// LocalAddr is a stub
func (c *Conn) LocalAddr() net.Addr {
	return nil
}

// RemoteAddr is a stub
func (c *Conn) RemoteAddr() net.Addr {
	return nil
}

// SetDeadline is a stub
func (c *Conn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is a stub
func (c *Conn) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline is a stub
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return nil
}
