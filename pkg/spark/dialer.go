package spark

import (
	"context"
	"net"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

// Dialer determines how to contact the RPC server
type Dialer interface {
	// Dial creates a connection to the RPC server
	//
	// Errors:
	//
	//   - warpforge-error-connection -- dial fails
	Dial(ctx context.Context) (net.Conn, error)
}

// netDialer is a wrapper around net.Dialer to conform to the Dialer interface
type netDialer struct {
	network string
	address string
	dialer  net.Dialer
}

// Dial uses netDialer's stored network and address to start a connection.
// See net.Dial, net.Dialer.DialContext
//
// Errors:
//
//   - warpforge-error-connection -- dial fails
func (n *netDialer) Dial(ctx context.Context) (net.Conn, error) {
	conn, err := n.dialer.DialContext(ctx, n.network, n.address)
	if err != nil {
		return nil, serum.Error(wfapi.ECodeConnection, serum.WithCause(err),
			serum.WithMessageTemplate("unable to dial server at network {{network|q}} and address {{address|q}}"),
			serum.WithDetail("network", n.network),
			serum.WithDetail("address", n.address),
		)
	}
	return conn, nil
}
