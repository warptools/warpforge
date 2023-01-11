package spark

import (
	"context"
	"net"
	// "os"
	"fmt"
	"io"
	"strings"

	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/plumbing/watch"
	"github.com/warptools/warpforge/wfapi"
)

type Config struct {
	WorkingDirectory string
}

// Run executes spark
// Errors:
//
//   - warpforge-error-io -- when socket path is too long
//   - warpforge-error-io -- when socket path cannot be canonicalized
//   - warpforge-error-io -- when unable to read from socket
//   - warpforge-error-io -- when unable to connect to socket
func (c *Config) Run(ctx context.Context) error {
	// fsys := os.DirFS("/")
	// wss, err := workspace.FindWorkspaceStack(fsys, "", c.WorkingDirectory)xd
	// if err != nil {
	// 	return err
	// }
	// find
	return c.remoteResolve(ctx)
}

// localResolve attempts to find the information by scraping workspace information
// Errors:
//
//   - warpforge-error-unknown -- not implemented
func (c *Config) localResolve(ctx context.Context) error {
	return serum.Error(wfapi.ECodeUnknown, serum.WithMessageLiteral("not implemented"))
}

// remoteResolve attempts to resolve over a socket
// Errors:
//
//   - warpforge-error-io -- when socket path is too long
//   - warpforge-error-io -- when socket path cannot be canonicalized
//   - warpforge-error-io -- when unable to read from socket
//   - warpforge-error-io -- when unable to connect to socket
func (c *Config) remoteResolve(ctx context.Context) error {
	path, xerr := watch.GenerateSocketPath(c.WorkingDirectory)
	if xerr != nil {
		return xerr
	}
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", path)
	if err != nil {
		return serum.Error(wfapi.ECodeIo,
			serum.WithMessageTemplate("could not connect to socket at path: {{path|q}}"),
			serum.WithDetail("path", path),
			serum.WithCause(err),
		)
	}
	defer conn.Close()
	data, err := io.ReadAll(conn)
	if err != nil {
		return serum.Error(wfapi.ECodeIo,
			serum.WithMessageLiteral("unable to read from socket"),
			serum.WithCause(err),
		)
	}
	fmt.Println(strings.TrimSpace(string(data)))
	return nil
}
