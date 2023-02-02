package watch

import (
	"context"
	"encoding/json"
	"net"
	"runtime/debug"
	"strconv"
	"time"

	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspaceapi"
	"github.com/warptools/warpforge/wfapi"
)

const (
	LogTag_Server         = "╬═  server"
	LogTag_DefaultHandler = "╬═? handler"
)

type handlerTagKey struct{}

func handlerTag(ctx context.Context) string {
	value := ctx.Value(handlerTagKey{})
	if value == nil {
		return LogTag_DefaultHandler
	}
	return value.(string)
}

func setHandlerTag(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, handlerTagKey{}, tag)
}

// server stores the current status of the plot execution and responds to clients
type server struct {
	listener    net.Listener
	handler     handler
	nowFn       func() time.Time
	readTimeout *time.Duration
	acceptCount int
}

const DefaultReadTimeout = 5 * time.Second

func (s *server) now() time.Time {
	if s.nowFn == nil {
		return time.Now()
	}
	return s.nowFn()
}

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
	defer log.Info(tag, "connection closed")
	defer cancel()
	defer conn.Close()
	dec := json.NewDecoder(conn)
	for {
		if err := s.setReadDeadline(conn); err != nil {
			return err
		}
		log.Info(tag, "getting next RPC")
		rpc, err := NextRPC(ctx, dec)
		if err != nil {
			log.Info(tag, "unable to read RPC: %s", err.Error())
			// TODO: send error response
			return err
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
		log.Info(tag, "handle request")
		response, err := s.handler.handle(ctx, rpc.ID, *rpc.Data.RpcRequest)
		if err != nil {
			log.Info(tag, "RPC handler failed: %s", err.Error())
			panic("TODO")
			//TODO: send error response
		}
		if response == nil || rpc.ID == "" {
			// ignore responses without IDs
			log.Info(tag, "RPC handler response is nil or request has no ID: %s", rpc.ID)
			continue
		}
		rpc.Data.RpcRequest = nil
		rpc.Data.RpcResponse = response
		log.Info(tag, "response")
		data, err := ipld.Marshal(ipldjson.Encode, rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
		if err != nil {
			log.Info(tag, "RPC handler failed to marshal output: %s", err.Error())
			return serum.Error(wfapi.ECodeSerialization, serum.WithCause(err))
		}
		_, err = conn.Write(data)
		if err != nil {
			log.Info(tag, "RPC handler failed to write data: %s", err.Error())
			return serum.Error(wfapi.ECodeIo, serum.WithCause(err))
		}
	}
}

// Errors:
//
//    - warpforge-error-rpc-serialization -- unable to convert ipld node to struct
func NodeToRpcRequest(data datamodel.Node) (*workspaceapi.RpcRequest, error) {
	np := bindnode.Prototype(&workspaceapi.RpcRequest{}, workspaceapi.TypeSystem.TypeByName("RpcRequest"))
	nb := np.NewBuilder()
	if err := datamodel.Copy(data, nb); err != nil {
		return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("RpcRequest is invalid"),
		)
	}
	request := bindnode.Unwrap(nb.Build()).(*workspaceapi.RpcRequest)
	return request, nil
}

func sendError(ctx context.Context, conn net.Conn, rpc workspaceapi.Rpc, err error) error {
	if err == nil {
		panic("server cannot send nil error")
	}
	code := serum.Code(err)
	if code == "" {
		code = workspaceapi.ECodeRpcUnknown
	}
	if err != nil {
		return serum.Error(wfapi.ECodeSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("unable to serialize error"),
		)
	}
	err = ipld.MarshalStreaming(conn, ipldjson.Encode, &rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
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

type handler struct {
	statusFetcher func(ctx context.Context, path string) (workspaceapi.ModuleStatus, error)
}

func (h *handler) handle(ctx context.Context, ID string, req workspaceapi.RpcRequest) (*workspaceapi.RpcResponse, error) {
	logger := logging.Ctx(ctx)
	tag := handlerTag(ctx)
	kind, _ := req.Kind()
	logger.Debug(tag, "request type: %s", kind)
	if ID == "" {
		return nil, nil
	}
	switch {
	case req.ModuleStatusQuery != nil:
		return h.methodModuleStatus(ctx, *req.ModuleStatusQuery)
	default:
		// TODO: reflect magic could tell you more about this, and allow us to do method registration.
		logger.Debug(tag, "method not found")
		return nil, serum.Error(workspaceapi.ECodeRpcMethodNotFound)
	}
}

func (h *handler) methodModuleStatus(ctx context.Context, req workspaceapi.ModuleStatusQuery) (*workspaceapi.RpcResponse, error) {
	logger := logging.Ctx(ctx)
	tag := handlerTag(ctx)
	status, err := h.statusFetcher(ctx, req.Path)
	if err != nil {
		logger.Debug(tag, "unable to get status")
		return nil, serum.Error(workspaceapi.ECodeRpcInternal, serum.WithCause(err))
	}
	result := &workspaceapi.RpcResponse{ModuleStatusAnswer: &workspaceapi.ModuleStatusAnswer{
		Path:   req.Path,
		Status: status,
	}}

	return result, nil
}
