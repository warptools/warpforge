package nettest

import (
	"context"
	"io"
	"net"
	"time"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

const DefaultTimeout = 5 * time.Second

// Uses net.Pipe for testing connections
type PipeListener struct {
	connections chan net.Conn
	ctx         context.Context
	done        chan struct{}
	Timeout     time.Duration // Sets default deadline for new connections.
}

// Errors: none
func (p *PipeListener) Close() error {
	// closing channel will unblock accept
	close(p.done)
	return nil
}

// Errors:
//
//  - warpforge-error-connection --
func (p *PipeListener) Accept() (net.Conn, error) {
	select {
	case <-p.done:
		return nil, serum.Error(wfapi.ECodeConnection, serum.WithCause(io.EOF))
	case <-p.ctx.Done():
		return nil, serum.Error(wfapi.ECodeConnection, serum.WithCause(p.ctx.Err()))
	case conn := <-p.connections:
		return conn, nil
	}
}
func (p *PipeListener) Addr() net.Addr { return nil }

// Errors:
//
//  - warpforge-error-connection --
func (p *PipeListener) Dial(ctx context.Context) (net.Conn, error) {
	serverConn, clientConn := net.Pipe()
	deadline := time.Now().Add(p.Timeout)
	clientConn.SetDeadline(deadline) // will cause tests to fail if they block
	select {
	case <-ctx.Done():
		return nil, serum.Error(wfapi.ECodeConnection, serum.WithCause(ctx.Err()))
	case p.connections <- serverConn:
		return clientConn, nil
	case <-time.After(p.Timeout):
		return nil, serum.Error(wfapi.ECodeConnection, serum.WithMessageLiteral("dial timeout"))
	}
}

func NewPipeListener(ctx context.Context) *PipeListener {
	return &PipeListener{
		ctx:         ctx,
		connections: make(chan net.Conn),
		done:        make(chan struct{}),
		Timeout:     DefaultTimeout,
	}
}
