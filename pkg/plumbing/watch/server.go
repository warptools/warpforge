package watch

import (
	"context"
	"encoding/json"
	"net"
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

const LOG_TAG_SRV = "╬═  server"

// server stores the current status of the plot execution and responds to clients
type server struct {
	listener    net.Listener
	handler     handler
	nowFn       func() time.Time
	readTimeout *time.Duration
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
//    - warpforge-error-rpc-method-not-found -- data is nil
func NextRPC(ctx context.Context, d *json.Decoder) (*workspaceapi.Rpc, error) {
	var err error
	var raw json.RawMessage
	var rpc workspaceapi.Rpc
	err = d.Decode(&raw)
	if err != nil {
		return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("unable to read JSON from connection"),
		)
	}
	log := logging.Ctx(ctx)
	log.Debug("---", string(raw))
	_, err = ipld.Unmarshal(raw, ipldjson.Decode, &rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
	if err != nil {
		return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err),
			serum.WithMessageLiteral("unable to read RPC message"),
		)
	}
	if rpc.Data == nil {
		return nil, serum.Error(workspaceapi.ECodeRpcMethodNotFound,
			serum.WithMessageLiteral("RPC message contains no data"))
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
	defer func() {
		r := recover()
		if r != nil {
			log.Info(LOG_TAG_SRV, "socket handler panic: %s", r)
			return
		}
		if err != nil {
			log.Info(LOG_TAG_SRV, "handler returned with error: %s", err.Error())
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer log.Info(LOG_TAG_SRV, "connection closed")
	defer cancel()
	defer conn.Close()
	dec := json.NewDecoder(conn)
	for {
		if err := s.setReadDeadline(conn); err != nil {
			return err
		}
		log.Info(LOG_TAG_SRV, "reading data")
		rpc, err := NextRPC(ctx, dec)
		if err != nil {
			log.Info(LOG_TAG_SRV, "unable to read RPC: %s", err.Error())
			// TODO: send error response
			return err
		}
		rpc.ErrorCode = nil // ignore errors sent by client
		log.Debug(LOG_TAG_SRV, "%v", rpc)
		log.Info(LOG_TAG_SRV, "binding request")
		request, err := NodeToRpcRequest(rpc.Data)
		if err != nil {
			log.Info(LOG_TAG_SRV, "unable to bind RPC data to struct: %s", err.Error())
			//TODO: send error response
			continue
		}
		log.Info(LOG_TAG_SRV, "handle request")
		response, err := s.handler.handle(ctx, rpc.ID, *request)
		if err != nil {
			log.Info(LOG_TAG_SRV, "RPC handler failed: %s", err.Error())
			//TODO: send error response
			continue
		}
		if response == nil || rpc.ID == "" {
			// ignore responses without IDs
			log.Info(LOG_TAG_SRV, "RPC handler response is nil or request has no ID: %s", rpc.ID)
			continue
		}
		rpc.Data = response
		log.Info(LOG_TAG_SRV, "response")
		err = ipld.MarshalStreaming(conn, ipldjson.Encode, &rpc, workspaceapi.TypeSystem.TypeByName("Rpc"))
		if err != nil {
			return serum.Error(wfapi.ECodeSerialization, serum.WithCause(err))
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
	rpc.ErrorCode = &code
	rpc.Data = nil
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
			log.Info(LOG_TAG_SRV, "server: socket error on accept: %s", err.Error())
			return err
		}
		go s.handle(ctx, conn)
		select {
		case <-ctx.Done():
			log.Info(LOG_TAG_SRV, "server: socket no longer accepting connections")
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

func (h *handler) handle(ctx context.Context, ID string, req workspaceapi.RpcRequest) (datamodel.Node, error) {
	logger := logging.Ctx(ctx)
	logger.Debug("", "handle")
	if ID == "" {
		return nil, nil
	}
	switch {
	case req.ModuleStatusQuery != nil:
		return h.methodModuleStatus(ctx, *req.ModuleStatusQuery)
	case req.Ping != nil:
		return h.methodPing(ctx, *req.Ping)
	default:
		// TODO: reflect magic could tell you more about this, and allow us to do method registration.
		logger.Debug("", "method not found")
		return nil, serum.Error(workspaceapi.ECodeRpcMethodNotFound)
	}
}

func (h *handler) methodPing(ctx context.Context, req workspaceapi.Ping) (datamodel.Node, error) {
	logger := logging.Ctx(ctx)
	logger.Debug("", "method: ping")
	var data workspaceapi.Ping

	response := &workspaceapi.PingAck{CallID: data.CallID}
	result := bindnode.Wrap(response, workspaceapi.TypeSystem.TypeByName("PingAck"))
	// result, err := ipld.Marshal(ipldjson.Encode, response, workspaceapi.TypeSystem.TypeByName("PingAck"))
	// if err != nil {
	// 	return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err))
	// }
	return result, nil
}

func (h *handler) methodModuleStatus(ctx context.Context, req workspaceapi.ModuleStatusQuery) (datamodel.Node, error) {
	logger := logging.Ctx(ctx)
	status, err := h.statusFetcher(ctx, req.Path)
	if err != nil {
		logger.Debug("", "unable to get status")
		return nil, serum.Error(workspaceapi.ECodeRpcInternal, serum.WithCause(err))
	}
	response := &workspaceapi.ModuleStatusAnswer{
		Path:   req.Path,
		Status: status,
	}
	result := bindnode.Wrap(response, workspaceapi.TypeSystem.TypeByName("ModuleStatusAnswer"))

	// result, err := ipld.Marshal(ipldjson.Encode, response, workspaceapi.TypeSystem.TypeByName("ModuleStatusAnswer"))
	// if err != nil {
	// 	logger.Debug("", "unable to get serialize status answer")
	// 	return nil, serum.Error(workspaceapi.ECodeRpcSerialization, serum.WithCause(err))
	// }
	return result, nil
}
