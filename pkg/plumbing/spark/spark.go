package spark

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"path/filepath"

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
	Fsys             fs.FS
	Path             string // path to module
	WorkingDirectory string // absolute path to working directory
	Dialer           jsonrpc2.Dialer
	OutputMarkup     string
	OutputStyle      string
	OutputColor      bool
}

const (
	// ECodeDial is an io error resulting from a net dial
	ECodeDial = wfapi.ECodeIo + "-dial"
	// ECodeQuery is used when a query fails for unknown reasons
	ECodeQuery = "warpforge-error-query"
)
const (
	SCodeNoModule = "warpforge-spark-no-module"
	SCodeNoSocket = "warpforge-spark-no-socket"
	SCodeUnknown  = "warpforge-spark-unknown"
)

func (c *Config) searchPath() string {
	path := c.Path
	if !filepath.IsAbs(c.Path) {
		path = filepath.Join(c.WorkingDirectory, c.Path)
	}
	return path
}

// filepath.Rel + serum
func relativePath(basepath, targpath string) (string, error) {
	result, err := filepath.Rel(basepath, targpath)
	if err != nil {
		return "", serum.Error(wfapi.ECodeSearchingFilesystem, serum.WithCause(err),
			serum.WithMessageLiteral("unable to find relative path from {{basePath|q}} to {{targPath|q}}"),
			serum.WithDetail("basePath", basepath),
			serum.WithDetail("targPath", targpath),
		)
	}
	return result, nil
}

func (c *Config) run(ctx context.Context) (workspaceapi.ModuleStatusAnswer, error) {
	result := workspaceapi.ModuleStatusAnswer{}
	logger := logging.Ctx(ctx)

	searchPath := c.searchPath()
	logger.Debug("", "search path: %q", searchPath)

	ws, _, err := workspace.FindWorkspace(c.Fsys, "", searchPath[1:])
	if ws == nil || err != nil {
		if err == nil {
			err = serum.Error(wfapi.ECodeWorkspace, serum.WithMessageLiteral("workspace not found"))
		}
		return result, serum.Error(SCodeNoModule, serum.WithCause(err))
	}
	_, wsPath := ws.Path()
	logger.Debug("", "ws path: %q", wsPath)

	searchPath, err = relativePath("/"+wsPath, searchPath)
	if err != nil {
		return result, serum.Error(SCodeNoModule, serum.WithCause(err))
	}

	logger.Debug("", "module search path: %q", searchPath)
	modulePath, _, err := dab.FindModule(c.Fsys, wsPath, searchPath[1:])
	if err != nil {
		return result, serum.Error(SCodeNoModule, serum.WithCause(err))
	}

	query := workspaceapi.ModuleStatusQuery{
		Path:          modulePath,
		InterestLevel: workspaceapi.ModuleInterestLevel_Query,
	}
	logger.Info("", "ModulePath: %s", query.Path)
	return c.resolve(ctx, ws, query)
}

// Run executes spark
//
// Did I confuse the crap out of serum. Yes I did. It's tradition now.
// Errors:
//
//   - warpforge-spark-no-module -- can't find module
//   - warpforge-spark-no-socket -- when socket does not dial or does not exist
//   - warpforge-spark-unknown -- all other errors
func (c *Config) Run(ctx context.Context) error {
	answer, err := c.run(ctx)
	frm := formatter{
		Markup: ValidateMarkup(c.OutputMarkup),
		Style:  ValidateStyle(c.OutputStyle),
		color:  c.OutputColor,
	}
	output := frm.format(ctx, answer, err)
	fmt.Println(output)
	return err
}

// Errors:
//
//   - warpforge-spark-no-socket -- when socket does not dial or does not exist
//   - warpforge-spark-unknown -- all other errors
func (c *Config) resolve(ctx context.Context, ws *workspace.Workspace, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	result, err := c.remoteResolve(ctx, ws, query)
	if err != nil {
		switch serum.Code(err) {
		case ECodeDial:
			return result, serum.Error(SCodeNoSocket, serum.WithCause(err))
		case SCodeNoSocket, SCodeUnknown:
			// Error Codes = warpforge-spark-no-socket, warpforge-spark-unknown
			return result, err
		default:
			return result, serum.Error(SCodeUnknown, serum.WithCause(err))
		}
	}
	return result, nil
}

// Errors:
//
//  - warpforge-spark-no-socket -- socket path cannot be created
func (c *Config) setupDialer(ws *workspace.Workspace) (jsonrpc2.Dialer, error) {
	if c.Dialer != nil {
		return c.Dialer, nil
	}
	path, err := watch.GenerateSocketPath(ws)
	if err != nil {
		return nil, serum.Error(SCodeNoSocket, serum.WithCause(err))
	}
	return jsonrpc2.NetDialer("unix", path, net.Dialer{}), nil
}

// remoteResolve attempts to resolve over a socket
// Errors:
//
//  - warpforge-spark-no-socket -- socket path cannot be created
//  - warpforge-spark-no-socket -- socket dial fails
//  - warpforge-error-query --
//  - warpforge-error-serialization --
func (c *Config) remoteResolve(ctx context.Context, ws *workspace.Workspace, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	dialer, err := c.setupDialer(ws)
	if err != nil {
		return workspaceapi.ModuleStatusAnswer{}, err
	}
	conn, err := dial(ctx, dialer, jsonrpc2.ConnectionOptions{})
	if err != nil {
		return workspaceapi.ModuleStatusAnswer{}, err
	}
	defer conn.Close()
	return moduleStatusQuery(ctx, conn, query)
}

// Errors:
//
//   - warpforge-spark-no-socket -- dial fails
func dial(ctx context.Context, dialer jsonrpc2.Dialer, opts jsonrpc2.ConnectionOptions) (*jsonrpc2.Connection, error) {
	conn, err := jsonrpc2.Dial(ctx, dialer, opts)
	if err != nil {
		return nil, serum.Error(SCodeNoSocket,
			serum.WithMessageTemplate("could not dial server"),
			serum.WithCause(err),
		)
	}
	return conn, nil
}

// Errors:
//
//   - warpforge-error-query --
//   - warpforge-error-serialization --
func moduleStatusQuery(ctx context.Context, conn *jsonrpc2.Connection, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	var result workspaceapi.ModuleStatusAnswer
	data, err := ipld.Marshal(ipldjson.Encode, &query, workspaceapi.TypeSystem.TypeByName("ModuleStatusQuery"))
	if err != nil {
		return result, serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("failed to serialize Module Status Query"))
	}
	async := conn.Call(ctx, workspaceapi.RpcModuleStatus, json.RawMessage(data))

	var msg json.RawMessage
	if err := async.Await(ctx, &msg); err != nil {
		return result, serum.Error(ECodeQuery, serum.WithCause(err),
			serum.WithMessageLiteral("Module Status Query failed"),
		)
	}

	_, err = ipld.Unmarshal([]byte(msg), ipldjson.Decode, &result, workspaceapi.TypeSystem.TypeByName("ModuleStatusAnswer"))
	if err != nil {
		return result, serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("failed to deserialize ModuleStatusAnswer"),
		)
	}
	return result, nil
}
