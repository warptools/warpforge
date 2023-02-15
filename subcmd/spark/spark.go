package spark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"path/filepath"

	"github.com/google/uuid"
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/subcmd/watch"
	"github.com/warptools/warpforge/wfapi"
)

type Config struct {
	Fsys             fs.FS     // Root filesystem. Generally os.DirFS("/").
	SearchPath       string    // Path to find module. Will attempt to find closest module within the workspace.
	WorkingDirectory string    // Absolute path to working directory
	Dialer           Dialer    // Method of dialing the server. May be nil for default.
	OutputMarkup     string    // Output Markup type
	OutputStyle      string    // Output style
	OutputColor      bool      // True for colored output
	OutputStream     io.Writer // where output should go
}

// ECodeSpark* error codes are sentinels used for spark output.
const (
	ECodeSparkNoModule    = "warpforge-spark-no-module"    // locally can't find module path
	ECodeSparkNoWorkspace = "warpforge-spark-no-workspace" // locally can't find workspace
	ECodeSparkNoSocket    = "warpforge-spark-no-socket"    // locally can't find socket or can't dial the socket
	ECodeSparkInternal    = "warpforge-spark-internal"     // other errors, including serialization, broken comms, invalid data errors
	ECodeSparkServer      = "warpforge-spark-server"       // server error response
)

// searchPath will canonicalize the ModulePath argument.
// May panic if WorkingDirectory is not an absolute path.
func (c *Config) searchPath() string {
	if filepath.IsAbs(c.SearchPath) {
		return filepath.Clean(c.SearchPath)
	}
	if !filepath.IsAbs(c.WorkingDirectory) {
		panic("working directory must be an absolute path")
	}
	return filepath.Join(c.WorkingDirectory, c.SearchPath)
}

func (c *Config) run(ctx context.Context) (workspaceapi.ModuleStatusAnswer, error) {
	var empty workspaceapi.ModuleStatusAnswer
	logger := logging.Ctx(ctx)

	searchPath := c.searchPath()
	logger.Debug("", "Search Path: %q", searchPath)

	ws, _, err := workspace.FindWorkspace(c.Fsys, "", searchPath)
	if err != nil {
		return empty, serum.Error(ECodeSparkNoWorkspace, serum.WithCause(err))
	}
	if ws == nil {
		return empty, serum.Error(ECodeSparkNoWorkspace, serum.WithMessageLiteral("workspace not found"))
	}
	_, wsPath := ws.Path()
	wsPath = filepath.Join("/", wsPath)
	logger.Debug("", "Workspace Path: %q", wsPath)

	modulePath, _, err := dab.FindModule(c.Fsys, wsPath, searchPath)
	if err != nil {
		return empty, serum.Error(ECodeSparkNoModule, serum.WithCause(err))
	}
	if modulePath == "" {
		return empty, serum.Error(ECodeSparkNoModule, serum.WithMessageLiteral("module not found"))
	}

	logger.Debug("", "Module Path rel: %q", modulePath)
	modulePath = filepath.Join("/", modulePath)
	logger.Debug("", "Module Path: %q", modulePath)
	relPath, err := dab.SubPathRel(wsPath, modulePath)
	if err != nil {
		panic(fmt.Errorf("unreachable: %w", err))
	}

	query := workspaceapi.ModuleStatusQuery{
		Path:          relPath,
		InterestLevel: workspaceapi.ModuleInterestLevel_Query,
	}
	return c.remoteResolve(ctx, ws, query)
}

func (c *Config) formatter() (*formatter, error) {
	markup, err := ValidateMarkup(c.OutputMarkup)
	if err != nil {
		return nil, err
	}
	style, err := ValidateStyle(c.OutputStyle)
	if err != nil {
		return nil, err
	}
	frm := formatter{
		Markup: markup,
		Style:  style,
		color:  c.OutputColor,
	}
	return &frm, nil
}

// Run executes spark
//
// Errors:
//
//   - warpforge-spark-no-workspace -- can't find workspace
//   - warpforge-spark-no-module -- can't find module
//   - warpforge-spark-no-socket -- when socket does not dial or does not exist
//   - warpforge-spark-internal -- all other errors
//   - warpforge-spark-server  -- server responded with an error
//   - warpforge-error-invalid -- invalid configuration
func (c *Config) Run(ctx context.Context) error {
	frm, err := c.formatter()
	if err != nil {
		return err
	}
	answer, err := c.run(ctx)
	output := frm.format(ctx, answer, err)
	fmt.Fprintln(c.OutputStream, output)
	return err
}

// Creates a default netDialer if no dialer is provided.
//
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

// remoteResolve attempts to resolve a module status query over a socket
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
	{ // code scope block to prevent serum from complaining about above errors
		answer, err := moduleStatusQuery(ctx, conn, query)
		if err != nil {
			return empty, err
		}
		return answer, nil
	}
}

// moduleStatusQuery will send a query over the connection and read the response.
// Returned errors will have "spark" sentinel codes that may be used to generate output.
//
// Errors:
//
//    - warpforge-spark-internal -- unable to send|receive request
//    - warpforge-spark-server -- server sent an error response
func moduleStatusQuery(ctx context.Context, conn net.Conn, query workspaceapi.ModuleStatusQuery) (workspaceapi.ModuleStatusAnswer, error) {
	var empty workspaceapi.ModuleStatusAnswer
	rpc := workspaceapi.Rpc{
		ID:   uuid.New().String(),
		Data: workspaceapi.RpcData{RpcRequest: &workspaceapi.RpcRequest{ModuleStatusQuery: &query}},
	}
	data, err := ipld.Marshal(watch.Encoder, &rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
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
	_, err = ipld.Unmarshal(raw, watch.Decoder, &response, workspaceapi.TypeSystem.TypeByName("Rpc"))
	if err != nil {
		return empty, serum.Error(ECodeSparkInternal,
			serum.WithCause(serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
				serum.WithMessageLiteral("failed to deserialize ModuleStatusAnswer"),
			)))
	}
	if err != nil {
		return empty, serum.Error(ECodeSparkInternal, serum.WithCause(err))
	}
	if response.Data.RpcResponse == nil {
		return empty, serum.Error(ECodeSparkInternal,
			serum.WithMessageLiteral("response missing data"),
		)
	}
	rpcResp := response.Data.RpcResponse
	switch {
	case rpcResp.ModuleStatusAnswer != nil:
		return *rpcResp.ModuleStatusAnswer, nil
	case rpcResp.Error != nil:
		data := rpcResp.Error.AsSerumData()
		return empty, serum.Error(ECodeSparkServer, serum.WithCause(&serum.ErrorValue{Data: *data}))
	default:
		return empty, serum.Error(ECodeSparkInternal, serum.WithMessageLiteral("unrecognized RPC response"))
	}
}
