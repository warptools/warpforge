package spark

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"golang.org/x/exp/jsonrpc2"

	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/plumbing/watch"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/wfapi"
)

type Config struct {
	WorkingDirectory string
	Dialer           jsonrpc2.Dialer
}

const (
	// ECodeDial is an io error resulting from a net dial
	ECodeDial = wfapi.ECodeIo + "-dial"
	// ECodeQuery is used when a query fails for unknown reasons
	ECodeQuery = "warpforge-error-query"
)

// Run executes spark
// Errors:
//
//   - warpforge-error-io -- when socket path is too long
//   - warpforge-error-io -- when socket path cannot be canonicalized
//   - warpforge-error-io -- when unable to read from socket
//   - warpforge-error-query --
//   - warpforge-error-serialization --
//   - warpforge-error-io-dial --
//   - warpforge-error-searching-filesystem --
//   - warpforge-error-unknown --
func (c *Config) Run(ctx context.Context) error {
	logger := logging.Ctx(ctx)
	fsys := os.DirFS("/")
	wss, err := workspace.FindWorkspaceStack(fsys, "", c.WorkingDirectory)
	if err != nil {
		return err
	}

	wsfs, path := wss.Local().Path()
	modulePath, _, err := dab.FindModule(wsfs, path, c.WorkingDirectory)
	if err != nil {
		return err
	}

	query := workspaceapi.ModuleStatusQuery{
		Path:          modulePath,
		InterestLevel: workspaceapi.ModuleInterestLevel_Query,
	}

	if err = c.remoteResolve(ctx, query); serum.Code(err) == ECodeDial {
		logger.Debug("", "%s", err)
		logger.Info("", "Failed to dial, falling back to local resolve")
		return c.localResolve(ctx, query)
	} else if err != nil {
		// ErrorCodes -= warpforge-error-io-dial
		return err
	}
	return nil
}

// localResolve attempts to find the information by scraping workspace information
// Errors:
//
//   - warpforge-error-unknown -- not implemented
func (c *Config) localResolve(ctx context.Context, query workspaceapi.ModuleStatusQuery) error {
	return serum.Error(wfapi.ECodeUnknown, serum.WithMessageLiteral("not implemented"))
}

func (c *Config) setupDialer() (jsonrpc2.Dialer, error) {
	if c.Dialer != nil {
		return c.Dialer, nil
	}
	path, xerr := watch.GenerateSocketPath(c.WorkingDirectory)
	if xerr != nil {
		return nil, xerr
	}
	return jsonrpc2.NetDialer("unix", path, net.Dialer{}), nil
}

// remoteResolve attempts to resolve over a socket
// Errors:
//
//   - warpforge-error-io -- when socket path is too long
//   - warpforge-error-io -- when socket path cannot be canonicalized
//   - warpforge-error-io -- when unable to read from socket
//   - warpforge-error-io-dial -- when unable to connect to socket
//   - warpforge-error-query --
//   - warpforge-error-serialization --
func (c *Config) remoteResolve(ctx context.Context, query workspaceapi.ModuleStatusQuery) error {
	dialer, err := c.setupDialer()
	if err != nil {
		return err
	}
	conn, err := dial(ctx, dialer, jsonrpc2.ConnectionOptions{})
	if err != nil {
		return err
	}
	defer conn.Close()
	result, err := (*connection)(conn).moduleStatusQuery(ctx, query)
	if err != nil {
		return err
	}
	fmt.Println(result.Status)
	return nil
}

func dial(ctx context.Context, dialer jsonrpc2.Dialer, opts jsonrpc2.ConnectionOptions) (*jsonrpc2.Connection, error) {
	conn, err := jsonrpc2.Dial(ctx, dialer, opts)
	if err != nil {
		return nil, serum.Error(ECodeDial,
			serum.WithMessageTemplate("could not dial server"),
			serum.WithCause(err),
		)
	}
	return conn, nil
}

type connection jsonrpc2.Connection

// Errors:
//
//   - warpforge-error-query --
//   - warpforge-error-serialization --
func (c *connection) moduleStatusQuery(ctx context.Context, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	var result workspaceapi.ModuleStatusAnswer
	data, err := ipld.Marshal(ipldjson.Encode, query, workspaceapi.TypeSystem.TypeByName("ModuleStatusQuery"))
	if err != nil {
		return result, serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("failed to serialize Module Status Query"))
	}
	async := (*jsonrpc2.Connection)(c).Call(ctx, workspaceapi.RpcModuleStatus, json.RawMessage(data))
	var raw json.RawMessage
	if err := async.Await(ctx, raw); err != nil {
		return result, serum.Error(ECodeQuery, serum.WithCause(err),
			serum.WithMessageLiteral("Module Status Query failed"),
		)
	}
	_, err = ipld.Unmarshal(raw, ipldjson.Decode, &result, workspaceapi.TypeSystem.TypeByName("ModuleStatusAnswer"))
	if err != nil {
		return result, serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("failed to deserialize ModuleStatusAnswer"),
		)
	}
	return result, nil
}
