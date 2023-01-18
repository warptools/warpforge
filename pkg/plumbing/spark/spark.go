package spark

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"path/filepath"

	"github.com/google/uuid"
	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/serum-errors/go-serum"

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
	Dialer           Dialer
	OutputMarkup     string
	OutputStyle      string
	OutputColor      bool
}

type Dialer interface {
	// Errors:
	//
	//   - warpforge-error-rpc-connection -- dial fails
	Dial(ctx context.Context) (net.Conn, error)
}

type netDialer struct {
	network string
	address string
	dialer  net.Dialer
}

// Errors:
//
//   - warpforge-error-rpc-connection -- dial fails
func (n *netDialer) Dial(ctx context.Context) (net.Conn, error) {
	conn, err := n.dialer.DialContext(ctx, n.network, n.address)
	if err != nil {
		return nil, serum.Error(workspaceapi.ECodeRpcConnection, serum.WithCause(err),
			serum.WithMessageTemplate("unable to dial server at network {{network|q}} and address {{address|q}}"),
			serum.WithDetail("network", n.network),
			serum.WithDetail("address", n.address),
		)
	}
	return conn, nil
}

const (
	// ECodeDial is an io error resulting from a net dial
	ECodeDial = wfapi.ECodeIo + "-dial"
	// ECodeQuery is used when a query fails for unknown reasons
	ECodeQuery = "warpforge-error-query"
)

const (
	ECodeSparkNoModule = "warpforge-spark-no-module" // locally can't find module path
	ECodeSparkNoSocket = "warpforge-spark-no-socket" // locally can't find socket or can't dial the socket
	ECodeSparkInternal = "warpforge-spark-internal"  // other errors, including serialization, broken comms, invalid data errors
	ECodeSparkServer   = "warpforge-spark-server"    // server error response
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
			serum.WithMessageTemplate("unable to find relative path from {{basePath|q}} to {{targPath|q}}"),
			serum.WithDetail("basePath", basepath),
			serum.WithDetail("targPath", targpath),
		)
	}
	return result, nil
}

func (c *Config) run(ctx context.Context) (workspaceapi.ModuleStatusAnswer, error) {
	var empty workspaceapi.ModuleStatusAnswer
	logger := logging.Ctx(ctx)

	searchPath := c.searchPath()
	logger.Debug("", "search path: %q", searchPath)

	ws, _, err := workspace.FindWorkspace(c.Fsys, "", searchPath[1:])
	if ws == nil || err != nil {
		if err == nil {
			err = serum.Error(wfapi.ECodeWorkspace, serum.WithMessageLiteral("workspace not found"))
		}
		return empty, serum.Error(ECodeSparkNoModule, serum.WithCause(err))
	}
	_, wsPath := ws.Path()
	logger.Debug("", "ws path: %q", wsPath)

	searchPath, err = relativePath("/"+wsPath, searchPath)
	if err != nil {
		return empty, serum.Error(ECodeSparkNoModule, serum.WithCause(err))
	}

	logger.Debug("", "module search path: %q", searchPath)
	modulePath, _, err := dab.FindModule(c.Fsys, wsPath, searchPath[1:])
	if err != nil {
		return empty, serum.Error(ECodeSparkNoModule, serum.WithCause(err))
	}

	query := workspaceapi.ModuleStatusQuery{
		Path:          modulePath,
		InterestLevel: workspaceapi.ModuleInterestLevel_Query,
	}
	logger.Info("", "ModulePath: %s", query.Path)
	return c.remoteResolve(ctx, ws, query)
}

// Run executes spark
//
// Errors:
//
//   - warpforge-spark-no-module -- can't find module
//   - warpforge-spark-no-socket -- when socket does not dial or does not exist
//   - warpforge-spark-internal -- all other errors
//   - warpforge-spark-server  -- server responded with an error
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
//  - warpforge-spark-no-socket -- socket path cannot be created
func (c *Config) getDialer(ws *workspace.Workspace) (Dialer, error) {
	if c.Dialer != nil {
		return c.Dialer, nil
	}
	// use default dialer
	path, err := watch.GenerateSocketPath(ws)
	if err != nil {
		return nil, serum.Error(ECodeSparkNoSocket, serum.WithCause(err))
	}
	return &netDialer{
		network: "unix",
		address: path,
		dialer:  net.Dialer{},
	}, nil
}

// remoteResolve attempts to resolve over a socket
// Errors:
//
//    - warpforge-spark-no-socket -- socket path cannot be created
//    - warpforge-spark-no-socket -- socket dial fails
//    - warpforge-spark-internal -- unable to send|receive request
//    - warpforge-spark-server -- server sent an error response
func (c *Config) remoteResolve(ctx context.Context, ws *workspace.Workspace, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	var empty workspaceapi.ModuleStatusAnswer
	dialer, err := c.getDialer(ws)
	if err != nil {
		return empty, serum.Error(ECodeSparkNoSocket, serum.WithCause(err))
	}
	conn, err := dialer.Dial(ctx)
	if err != nil {
		return empty, serum.Error(ECodeSparkNoSocket, serum.WithCause(err),
			serum.WithMessageTemplate("could not dial server"),
		)
	}
	defer conn.Close()
	{ // code block to prevent serum from complaining
		answer, err := moduleStatusQuery(ctx, conn, query)
		if err != nil {
			return empty, err
		}
		return answer, nil
	}
}

// Errors:
//
//    - warpforge-spark-internal -- unable to send|receive request
//    - warpforge-spark-server -- server sent an error response
func moduleStatusQuery(ctx context.Context, conn net.Conn, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	var empty workspaceapi.ModuleStatusAnswer
	queryNode := bindnode.Wrap(query, workspaceapi.TypeSystem.TypeByName("ModuleStatusQuery"))
	rpc := workspaceapi.Rpc{
		ID:   uuid.New().String(),
		Data: queryNode,
	}
	data, err := ipld.Marshal(ipldjson.Encode, &rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
	if err != nil {
		return empty, serum.Error(ECodeSparkInternal,
			serum.WithCause(serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
				serum.WithMessageLiteral("failed to serialize Module Status Query"),
			)))
	}

	if _, err := conn.Write(data); err != nil {
		return empty, serum.Error(ECodeSparkInternal,
			serum.WithCause(serum.Error(workspaceapi.ECodeRpcConnection, serum.WithCause(err),
				serum.WithMessageLiteral("unable to send RPC request"),
			)))
	}

	var raw json.RawMessage
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&raw); err != nil {
		return empty, serum.Error(ECodeSparkInternal,
			serum.WithCause(serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
				serum.WithMessageLiteral("unable to retrieve json from socket"),
			)))
	}

	var response workspaceapi.Rpc
	_, err = ipld.Unmarshal(raw, ipldjson.Decode, &response, workspaceapi.TypeSystem.TypeByName("Rpc"))
	if err != nil {
		return empty, serum.Error(ECodeSparkInternal,
			serum.WithCause(serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
				serum.WithMessageLiteral("failed to deserialize ModuleStatusAnswer"),
			)))
	}

	rpcResp, err := NodeToRpcResponse(response.Data)
	if err != nil {
		return empty, serum.Error(ECodeSparkInternal, serum.WithCause(err))
	}
	switch {
	case rpcResp.ModuleStatusAnswer != nil:
		return *rpcResp.ModuleStatusAnswer, nil
	case rpcResp.Error != nil:
		return empty, serum.Error(ECodeSparkServer, serum.WithCause(rpcResp.Error))
	default:
		return empty, serum.Error(ECodeSparkInternal, serum.WithMessageLiteral("unrecognized RPC response"))
	}
}

// Errors:
//
//    - warpforge-error-rpc-serialization -- unable to convert ipld node to struct
func NodeToRpcResponse(data datamodel.Node) (*workspaceapi.RpcResponse, error) {
	np := bindnode.Prototype(&workspaceapi.RpcResponse{}, workspaceapi.TypeSystem.TypeByName("RpcResponse"))
	nb := np.NewBuilder()
	if err := datamodel.Copy(data, nb); err != nil {
		return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("server response is invalid"),
		)
	}
	response := bindnode.Unwrap(nb.Build()).(*workspaceapi.RpcResponse)
	return response, nil
}