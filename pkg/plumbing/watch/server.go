package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"strconv"
	"time"

	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/wfapi"
)

// Log tags for the server. Used to easily distinguish server/handler logs from plot/formula/etc logs.
const (
	LogTag_Server         = "╬═  server"
	LogTag_DefaultHandler = "╬═? handler" // Default log tag for handler that doesn't have a connection ID
)

// key for context.Context used to set/retrieve handler logging tag
type handlerTagKey struct{}

// handlerTag retrieves the handler logging tag from context
func handlerTag(ctx context.Context) string {
	value := ctx.Value(handlerTagKey{})
	if value == nil {
		return LogTag_DefaultHandler
	}
	return value.(string)
}

// setHandlerTag returns a new context with the given handler logging tag value
func setHandlerTag(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, handlerTagKey{}, tag)
}

// server stores the current status of the plot execution and responds to clients.
type server struct {
	listener    net.Listener     // Where connections come from.
	handler     rpcHandler       // Handles actual RPCs.
	nowFn       func() time.Time // Should return _now_ for the purposes of setting deadlines. If nil, time.Now will be used.
	readTimeout *time.Duration   // Timeout for deadlines.
	acceptCount int              // Tracing metric. Used in log tags for handlers.
}

const DefaultReadTimeout = 5 * time.Second // Default read timeout for connections. See net.Conn.SetReadDeadline, server.setReadDeadline.

// now returns the result of time.Now
// May be overridden by setting nowFn
func (s *server) now() time.Time {
	if s.nowFn == nil {
		return time.Now()
	}
	return s.nowFn()
}

// setReadDeadline sets the connection read deadline based on server configuration.
// This should prevent connections from getting stuck waiting for a client request.
// The default deadline is DefaultReadTimout from time.Now.
//
// Errors:
//
//   - warpforge-error-internal -- failed to set connection read deadline
func (s *server) setReadDeadline(conn net.Conn) error {
	to := DefaultReadTimeout
	if s.readTimeout != nil {
		to = *s.readTimeout
	}
	deadline := s.now().Add(to)
	err := conn.SetReadDeadline(deadline)
	// err := conn.SetDeadline(deadline)
	if err != nil {
		return serum.Error(wfapi.ECodeInternal,
			serum.WithCause(err),
			serum.WithMessageTemplate("server failed to set read deadline {{deadline}}"),
			serum.WithDetail("deadline", deadline.String()),
		)
	}
	return nil
}

// NextRPC returns the next RPC object on the decoder's stream.
//
// Errors:
//
//    - warpforge-error-rpc-serialization -- bad connection or invalid json
//    - warpforge-error-rpc-serialization -- invalid RPC data
func NextRPC(ctx context.Context, d *json.Decoder) (*workspaceapi.Rpc, error) {
	var err error
	var raw json.RawMessage
	var rpc workspaceapi.Rpc
	log := logging.Ctx(ctx)
	tag := handlerTag(ctx)
	log.Debug(tag, "reading json")
	err = d.Decode(&raw)
	if err != nil {
		return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("unable to read JSON from connection"),
		)
	}
	log.Debug(tag+" ---", string(raw))
	_, err = ipld.Unmarshal(raw, ipldjson.Decode, &rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
	if err != nil {
		return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("unable to read RPC message"),
		)
	}
	return &rpc, nil
}

// handle is expected to respond to client connections.
// This function should recover from panics and log errors before returning.
// It is expected that handle is run as a goroutine and that errors may not be handled.
//
// handle emits the current status of the watch command over the connection
func (s *server) handle(ctx context.Context, conn net.Conn) (err error) {
	log := logging.Ctx(ctx)
	tag := handlerTag(ctx)
	defer func() {
		r := recover()
		if r != nil {
			log.Info(tag, "socket handler panic: %s", r)
			log.Info(tag, string(debug.Stack()))
			return
		}
		if err != nil {
			log.Info(tag, "handler returned with error: %s", err.Error())
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer log.Debug(tag, "connection closed")
	defer cancel()
	defer conn.Close()
	dec := json.NewDecoder(conn)
	for {
		if err := s.setReadDeadline(conn); err != nil {
			return err
		}
		log.Debug(tag, "getting next RPC")
		rpc, err := NextRPC(ctx, dec)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			{
				err := sendError(ctx, conn, rpc.ID, err)
				if err != nil {
					log.Debug(tag, "watch: unable to send error response")
				}
			}
			return fmt.Errorf("watch: unable to read RPC: %w", err)
		}
		if rpc.Data.RpcResponse != nil {
			if rpc.ID == "" {
				//ignore echo messages with no ID
				continue
			}
			// echo any RPC received that is a response type
			ipld.MarshalStreaming(conn, ipldjson.Encode, rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
			continue
		}
		log.Debug(tag, "handle request")
		response, err := s.handler.handle(ctx, rpc.ID, *rpc.Data.RpcRequest)
		if err != nil {
			{
				err := sendError(ctx, conn, rpc.ID, err)
				if err != nil {
					log.Debug(tag, "watch: unable to send error response")
				}
			}
			return fmt.Errorf("watch: RPC handler failed: %w", err)
		}
		if response == nil || rpc.ID == "" {
			// ignore responses without IDs
			log.Debug(tag, "RPC handler response is nil or request has no ID: %s", rpc.ID)
			continue
		}
		rpc.Data.RpcRequest = nil
		rpc.Data.RpcResponse = response
		log.Debug(tag, "response")
		data, err := ipld.Marshal(ipldjson.Encode, rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
		if err != nil {
			return serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
				serum.WithMessageLiteral("RPC handler failed to serialize response"),
			)
		}
		_, err = conn.Write(data)
		if err != nil {
			return serum.Error(wfapi.ECodeIo, serum.WithCause(err),
				serum.WithMessageLiteral("RPC handler failed to write data to connection"),
			)
		}
	}
}

func recurseError(err error) *workspaceapi.Error {
	if err == nil {
		return nil
	}
	var msg *string
	if m := serum.Message(err); m != "" {
		msg = &m
	}
	var deets *workspaceapi.Details
	for _, d := range serum.Details(err) {
		deets.Keys = append(deets.Keys, d[0])
		if deets.Values == nil {
			deets.Values = make(map[string]string)
		}
		deets.Values[d[0]] = d[1]
	}

	return &workspaceapi.Error{
		Code:    serum.Code(err),
		Message: msg,
		Details: deets,
		Cause:   recurseError(err),
	}
}

// sendError is a helper to serialize error responses to the client.
// sendError will panic if the error response to serialize is nil.
//
// Errors:
//
//   - warpforge-error-serialization --
func sendError(ctx context.Context, conn net.Conn, id string, response error) error {
	if response == nil {
		panic("server cannot send nil error")
	}
	output := recurseError(response)
	if output.Code == "" {
		output.Code = workspaceapi.ECodeRpcUnknown
	}
	rpc := &workspaceapi.Rpc{
		ID: id,
		Data: workspaceapi.RpcData{
			RpcResponse: &workspaceapi.RpcResponse{
				Error: output,
			},
		},
	}
	err := ipld.MarshalStreaming(conn, ipldjson.Encode, &rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
	if err != nil {
		return serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("unable to send rpc error response"),
		)
	}
	return nil
}

// serve accepts and handles connections to the server.
// Serve should not return under normal circumstances, however if an error occurs then it will log that error and return it.
// Context will only be checked between accepted connections, canceling this function while it's blocking requires the server's listener to be closed.
func (s *server) serve(ctx context.Context) error {
	// It is expected that serve will be called as a goroutine and the returned error may not be handled.
	// Any errors should be logged before returning.
	log := logging.Ctx(ctx)
	if s.listener == nil {
		panic("server has nil listener")
	}
	for {
		conn, err := s.listener.Accept() // blocks, doesn't accept a context.
		if err != nil {
			log.Info(LogTag_Server, "server: socket error on accept: %s", err.Error())
			return err
		}
		handlerCtx := setHandlerTag(ctx, LogTag_Server+"["+strconv.Itoa(s.acceptCount)+"]")
		s.acceptCount++
		go s.handle(handlerCtx, conn)
		select {
		case <-ctx.Done():
			log.Info(LogTag_Server, "server: socket no longer accepting connections")
			return nil
		default:
		}
	}
}

// listener will create a unix socket on the given path
func listener(ctx context.Context, sockPath string) (net.Listener, error) {
	cfg := net.ListenConfig{}
	l, err := cfg.Listen(ctx, "unix", sockPath)
	if err != nil {
		return nil, wfapi.ErrorIo("could not create socket", sockPath, err)
	}
	return l, nil
}

// rpcHandler handles the actual RpcRequests.
type rpcHandler struct {
	statusFetcher func(ctx context.Context, path string) (workspaceapi.ModuleStatus, error)
}

// handle selects and calls method to handle request and errors if the method is not implemented.
func (h *rpcHandler) handle(ctx context.Context, ID string, req workspaceapi.RpcRequest) (*workspaceapi.RpcResponse, error) {
	logger := logging.Ctx(ctx)
	tag := handlerTag(ctx)
	kind, err := req.Kind()
	logger.Debug(tag, "request type: %q: err: %v", kind, err)
	if ID == "" {
		return nil, nil
	}
	switch {
	case req.ModuleStatusQuery != nil:
		return h.methodModuleStatus(ctx, *req.ModuleStatusQuery)
	default:
		// NOTE: strictly speaking, this should be unreachable by virtue of ipld schema validation.
		// Reaching this means we failed to implement a method in the schema.
		logger.Debug(tag, "method not found")
		return nil, serum.Error(workspaceapi.ECodeRpcMethodNotFound)
	}
}

// methodModuleStatus finds the status for module based on the ModuleStatusQuery.
func (h *rpcHandler) methodModuleStatus(ctx context.Context, req workspaceapi.ModuleStatusQuery) (*workspaceapi.RpcResponse, error) {
	logger := logging.Ctx(ctx)
	tag := handlerTag(ctx)
	status, err := h.statusFetcher(ctx, req.Path)
	if err != nil {
		logger.Debug(tag, "unable to get status")
		return nil, serum.Error(workspaceapi.ECodeRpcMethodInternal, serum.WithCause(err))
	}
	result := &workspaceapi.RpcResponse{ModuleStatusAnswer: &workspaceapi.ModuleStatusAnswer{
		Path:   req.Path,
		Status: status,
	}}

	return result, nil
}
