package watch

import (
	"context"
	"encoding/json"
	"fmt"

	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"golang.org/x/exp/jsonrpc2"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/wfapi"
)

// server stores the current status of the plot execution and responds to clients
type server struct {
	listener  jsonrpc2.Listener
	rpcServer jsonrpc2.Server
	binder    jsonrpc2.Binder
}

// serve accepts and handles connections to the server.
func (s *server) serve(ctx context.Context) error {
	log := logging.Ctx(ctx)
	log.Debug("", "serve")
	if s.listener == nil {
		err := fmt.Errorf("did not call listen on server")
		log.Info("", err.Error())
		return err
	}
	server, err := jsonrpc2.Serve(ctx, s.listener, s.binder)
	if err != nil {
		return serum.Error(wfapi.ECodeUnknown, serum.WithCause(err),
			serum.WithMessageLiteral("unable to start server"),
		)
	}
	server.Wait()
	return nil
}

// listen will create a unix socket on the given path
// listen should be called before "serve"
func (s *server) listen(ctx context.Context, sockPath string) (err error) {
	listener, err := jsonrpc2.NetListener(ctx, "unix", sockPath, jsonrpc2.NetListenOptions{})
	if err != nil {
		return wfapi.ErrorIo("could not create socket", sockPath, err)
	}
	s.listener = listener
	return nil
}

type binder struct {
	framer    jsonrpc2.Framer
	historian *historian
}

// Errors: none
func (b binder) Bind(ctx context.Context, conn *jsonrpc2.Connection) (jsonrpc2.ConnectionOptions, error) {
	logger := logging.Ctx(ctx)
	logger.Debug("", "bind")
	h := &handler{
		statusFetcher: b.historian.GetStatus,
	}
	return jsonrpc2.ConnectionOptions{
		Framer:    b.framer,
		Preempter: nil,
		Handler:   h,
	}, nil
}

type handler struct {
	statusFetcher func(ctx context.Context, path string) (workspaceapi.ModuleStatus, error)
}

// Handle _MUST_ return jsonrpc2 errors. We should probably use a different library.
// jsonrpc2 is switching directly on error types which have wire significance to the JSON-RPC 2.0 specification.
// Alternatively we can upstream ignoring particular functions in serum.
//
// Errors: ignore
func (h *handler) Handle(ctx context.Context, req *jsonrpc2.Request) (interface{}, error) {
	logger := logging.Ctx(ctx)
	logger.Debug("", "handle")
	switch req.Method {
	case workspaceapi.RpcModuleStatus:
		return h.methodModuleStatus(ctx, req.Params)
	case workspaceapi.RpcPing:
		return h.methodPing(ctx, req.Params)
	default:
		logger.Debug("", "method %q not found", req.Method)
		return nil, jsonrpc2.ErrMethodNotFound
	}
}

func (h *handler) methodPing(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
	logger := logging.Ctx(ctx)
	logger.Debug("", "method: ping")
	var data workspaceapi.Ping
	_, err := ipld.Unmarshal(req, ipldjson.Decode, &data, workspaceapi.TypeSystem.TypeByName("Ping"))
	if err != nil {
		logger.Debug("", "failed to unmarshal ping struct: %s", err)
		return nil, jsonrpc2.ErrParse
	}

	response := &workspaceapi.PingAck{CallID: data.CallID}
	result, err := ipld.Marshal(ipldjson.Encode, response, workspaceapi.TypeSystem.TypeByName("PingAck"))
	if err != nil {
		return nil, jsonrpc2.ErrInternal
	}
	return json.RawMessage(result), nil
}

func (h *handler) methodModuleStatus(ctx context.Context, req json.RawMessage) (json.RawMessage, error) {
	logger := logging.Ctx(ctx)
	logger.Debug("", "method: module status")
	var data workspaceapi.ModuleStatusQuery
	_, err := ipld.Unmarshal(req, ipldjson.Decode, &data, workspaceapi.TypeSystem.TypeByName("ModuleStatusQuery"))
	if err != nil {
		return nil, jsonrpc2.ErrParse
	}

	status, err := h.statusFetcher(ctx, data.Path)
	if err != nil {
		logger.Debug("", "unable to get status")
		return nil, jsonrpc2.ErrInternal
	}
	response := &workspaceapi.ModuleStatusAnswer{
		Path:   data.Path,
		Status: status,
	}

	result, err := ipld.Marshal(ipldjson.Encode, response, workspaceapi.TypeSystem.TypeByName("ModuleStatusAnswer"))
	if err != nil {
		logger.Debug("", "unable to get serialize status answer")
		return nil, jsonrpc2.ErrInternal
	}
	return json.RawMessage(result), nil
}
