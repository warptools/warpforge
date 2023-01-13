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
	OutputStyle      string
	OutputColor      bool
}

const (
	// ECodeDial is an io error resulting from a net dial
	ECodeDial = wfapi.ECodeIo + "-dial"
	// ECodeQuery is used when a query fails for unknown reasons
	ECodeQuery = "warpforge-error-query"
)

func (c *Config) format(a workspaceapi.ModuleStatusAnswer, err error) string {
	phase, _ := Status2Phase[a.Status]
	if err != nil {
		phase = Code2Phase(serum.Code(err))
	}
	switch Style(c.OutputStyle) {
	case MarkupBash:
		if c.OutputColor {
			return fmt.Sprintf("\\[%s\\]%s\\[%s\\]", dasAnsiColorMap[phase], dasMap[phase], AnsiColorReset)
		}
		return fmt.Sprintf("%s", dasMap[phase])
	case MarkupAnsi:
		if c.OutputColor {
			return fmt.Sprintf("%s%s%s", dasAnsiColorMap[phase], dasMap[phase], AnsiColorReset)
		}
		return fmt.Sprintf("%s", dasMap[phase])
	case MarkupPango:
		if c.OutputColor {
			return fmt.Sprintf("<span %s>%s</span>", dasPangoColorMap[phase], dasMap[phase])
		}
		return fmt.Sprintf("<span>%s</span>", dasMap[phase])
	case MarkupSimple:
		fallthrough
	default:
		if err != nil {
			return serum.Code(err)
		}
		return string(a.Status)
	}
}

func (c *Config) searchPath() string {
	path := c.Path
	if !filepath.IsAbs(c.Path) {
		path = filepath.Join(c.WorkingDirectory, c.Path)
	}
	return path
}

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
	logger.Debug("", "output style: %s; with color: %t", c.OutputStyle, c.OutputColor)

	searchPath := c.searchPath()
	logger.Debug("", "search path: %q", searchPath)

	ws, _, err := workspace.FindWorkspace(c.Fsys, "", searchPath[1:])
	if err != nil {
		return err
	}
	_, wsPath := ws.Path()
	logger.Debug("", "ws path: %q", wsPath)

	searchPath, err = filepath.Rel("/"+wsPath, searchPath)
	if err != nil {
		return serum.Error(wfapi.ECodeSearchingFilesystem, serum.WithCause(err),
			serum.WithMessageLiteral("unable to find relative path from workspace {{basisPath|q}} to search path {{searchPath|q}}"),
			serum.WithDetail("basisPath", wsPath),
			serum.WithDetail("searchPath", searchPath),
		)
	}

	logger.Debug("", "module search path: %q", searchPath)
	modulePath, _, err := dab.FindModule(c.Fsys, wsPath, searchPath[1:])
	if err != nil {
		return err
	}

	query := workspaceapi.ModuleStatusQuery{
		Path:          modulePath,
		InterestLevel: workspaceapi.ModuleInterestLevel_Query,
	}
	logger.Info("", "ModulePath: %s", query.Path)
	answer, err := c.resolve(ctx, ws, query)
	output := c.format(answer, err)
	fmt.Println(output)
	return err
}

// Errors:
//
//   - warpforge-error-io -- when socket path is too long
//   - warpforge-error-io -- when socket path cannot be canonicalized
//   - warpforge-error-io -- when unable to read from socket
//   - warpforge-error-io-dial -- when unable to connect to socket
//   - warpforge-error-query --
//   - warpforge-error-serialization --
//   - warpforge-error-unknown -- not implemented
func (c *Config) resolve(ctx context.Context, ws *workspace.Workspace, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	result, err := c.remoteResolve(ctx, ws, query)
	if serum.Code(err) == ECodeDial {
		logger := logging.Ctx(ctx)
		logger.Debug("", "%s", err)
		logger.Info("", "Failed to dial, falling back to local resolve")
		return c.localResolve(ctx, query)
	}
	// ErrorCodes -= warpforge-error-io-dial
	return result, nil
}

// localResolve attempts to find the information by scraping workspace information
// Errors:
//
//   - warpforge-error-unknown -- not implemented
func (c *Config) localResolve(ctx context.Context, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	return workspaceapi.ModuleStatusAnswer{}, serum.Error(wfapi.ECodeUnknown, serum.WithMessageLiteral("not implemented"))
}

func (c *Config) setupDialer(ws *workspace.Workspace) (jsonrpc2.Dialer, error) {
	if c.Dialer != nil {
		return c.Dialer, nil
	}
	path, xerr := watch.GenerateSocketPath(ws)
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
//   - warpforge-error-io-dial --
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
