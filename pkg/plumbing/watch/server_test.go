package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	ipldfmt "github.com/ipld/go-ipld-prime/printer"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspaceapi"
)

// Uses net.Pipe for testing connections
type pipeListener struct {
	connections chan net.Conn
	ctx         context.Context
	done        chan struct{}
}

// Errors: none
func (p *pipeListener) Close() error {
	// closing channel will unblock accept
	close(p.done)
	return nil
}

// Errors: none
func (p *pipeListener) Accept() (net.Conn, error) {
	select {
	case <-p.done:
		return nil, io.EOF
	case <-p.ctx.Done():
		return nil, p.ctx.Err()
	case conn := <-p.connections:
		return conn, nil
	}
}
func (p *pipeListener) Addr() net.Addr { return nil }

// Errors: none
func (p *pipeListener) Dial(ctx context.Context) (io.ReadWriteCloser, error) {
	serverConn, clientConn := net.Pipe()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case p.connections <- serverConn:
		return clientConn, nil
	}
}

func NewPipeListener(ctx context.Context) *pipeListener {
	return &pipeListener{
		ctx:         ctx,
		connections: make(chan net.Conn),
		done:        make(chan struct{}),
	}
}

func NewLogBuffers(t *testing.T, ctx context.Context) (context.Context, func()) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	logger := logging.NewLogger(stdout, stderr, false, false, true)
	ctx = logger.WithContext(ctx)
	return ctx, func() {
		stdoutData, err := io.ReadAll(stdout)
		if err != nil {
			t.Log("unable to read stdout")
		}
		t.Logf("flush stdout:\n%s", string(stdoutData))

		stderrData, err := io.ReadAll(stderr)
		if err != nil {
			t.Log("unable to read stderr")
		}
		t.Logf("flush stderr:\n%s", string(stderrData))
	}
}

func nodeToRpcResponse(data datamodel.Node) (*workspaceapi.RpcResponse, error) {
	np := bindnode.Prototype(&workspaceapi.RpcResponse{}, workspaceapi.TypeSystem.TypeByName("RpcResponse"))
	nb := np.NewBuilder()
	if err := datamodel.Copy(data, nb); err != nil {
		return nil, err
	}
	result := bindnode.Unwrap(nb.Build()).(*workspaceapi.RpcResponse)
	return result, nil
}

func TestServerShutdown(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(func() { cancel() })
	ctx, flushLogs := NewLogBuffers(t, ctx)
	t.Cleanup(func() { flushLogs() })

	ngo := runtime.NumGoroutine()
	listener := NewPipeListener(ctx)

	hist := &historian{}
	srv := &server{
		listener: listener,
		handler:  handler{statusFetcher: hist.getStatus},
	}

	done := make(chan struct{})
	go func(ch chan<- struct{}) { srv.serve(ctx); ch <- struct{}{} }(done) // this is a lot of crap to ensure that we can actually shut down the server.

	conn, err := listener.Dial(ctx)
	qt.Assert(t, err, qt.IsNil)

	cancel()
	conn.Close()
	listener.Close()
	<-done
	qt.Assert(t, runtime.NumGoroutine() <= ngo, qt.IsTrue) // if this ever fails... maybe delete it
}

func TestRoundtrip_Streaming(t *testing.T) {
	t.Skipf("ipld streaming codec tries to read until EOF. This is not useful where there is no EOF.")

	var err error
	buf := &bytes.Buffer{}
	query := workspaceapi.Ping{CallID: "foobar"}
	dataNode := bindnode.Wrap(&query, workspaceapi.TypeSystem.TypeByName("Ping"))
	request := workspaceapi.Rpc{
		ID:   "1",
		Data: dataNode,
	}
	err = ipld.MarshalStreaming(buf, ipldjson.Encode, &request, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)
	expected := `{"ID":"1","Data":{"callID":"foobar"}}`
	replacer := strings.NewReplacer("\n", "", "\t", "", " ", "")
	qt.Assert(t, replacer.Replace(buf.String()), qt.Equals, expected)
	r, w := io.Pipe()
	go io.Copy(w, buf)
	var uut workspaceapi.Rpc
	t.Cleanup(func() { w.Close(); r.Close() })
	t.Deadline()
	ch := make(chan struct{})
	go func() {
		_, err = ipld.UnmarshalStreaming(r, ipldjson.Decode, &uut, workspaceapi.TypeSystem.TypeByName("Rpc"))
		ch <- struct{}{}
	}()
	select {
	case <-time.After(time.Second):
		t.Fatalf("test timeout")
	case <-ch:
	}
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, uut, qt.Equals, request)
}

func TestRoundtrip_PreReadJson(t *testing.T) {
	var err error
	buf := &bytes.Buffer{}
	request := workspaceapi.RpcRequest{
		Ping: &workspaceapi.Ping{CallID: "foobar"},
	}
	dataNode := bindnode.Wrap(&request, workspaceapi.TypeSystem.TypeByName("RpcRequest"))
	rpcOut := workspaceapi.Rpc{
		ID:   "1",
		Data: dataNode,
	}
	err = ipld.MarshalStreaming(buf, ipldjson.Encode, &rpcOut, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)

	expected := `{"ID":"1","Data":{"ping":{"callID":"foobar"}}}`
	replacer := strings.NewReplacer("\n", "", "\t", "", " ", "") // ipld json always adds whitespace
	qt.Assert(t, replacer.Replace(buf.String()), qt.Equals, expected)

	t.Log("write:\n", buf.String())

	r, w := io.Pipe()
	go io.Copy(w, buf)
	t.Cleanup(func() { w.Close(); r.Close() })
	t.Deadline()

	// Using the json decoder as an intermediate step works but is ugly
	var raw json.RawMessage
	dec := json.NewDecoder(r)
	err = dec.Decode(&raw)
	qt.Assert(t, err, qt.IsNil)

	uut := workspaceapi.Rpc{}
	_, err = ipld.Unmarshal(raw, ipldjson.Decode, &uut, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)

	// Convert data to RPC Request
	t.Log("read:\n", string(raw))
	t.Log("ipldprint:\n", ipldfmt.Sprint(uut.Data))
	np := bindnode.Prototype(&workspaceapi.RpcRequest{}, workspaceapi.TypeSystem.TypeByName("RpcRequest"))
	nb := np.NewBuilder()
	err = datamodel.Copy(uut.Data, nb)
	qt.Assert(t, err, qt.IsNil)

	result := bindnode.Unwrap(nb.Build()).(*workspaceapi.RpcRequest)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.Equals, request)
}

func TestServerPing(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(func() { cancel() })
	ctx, flushLogs := NewLogBuffers(t, ctx)
	t.Cleanup(func() { flushLogs() })

	listener := NewPipeListener(ctx)
	t.Cleanup(func() { listener.Close() })

	srv := &server{
		listener: listener,
		handler:  handler{},
	}
	go srv.serve(ctx)

	conn, err := listener.Dial(ctx)
	qt.Assert(t, err, qt.IsNil)
	t.Cleanup(func() { conn.Close() })
	dec := json.NewDecoder(conn)

	request := workspaceapi.RpcRequest{
		Ping: &workspaceapi.Ping{CallID: "foobar"},
	}
	dataNode := bindnode.Wrap(&request, workspaceapi.TypeSystem.TypeByName("RpcRequest"))
	rpcOut := workspaceapi.Rpc{
		ID:   uuid.New().String(),
		Data: dataNode,
	}

	t.Log("send request")
	buf := &bytes.Buffer{}
	err = ipld.MarshalStreaming(buf, ipldjson.Encode, &rpcOut, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)
	_, err = io.Copy(conn, buf)
	qt.Assert(t, err, qt.IsNil)

	t.Log("read reply")
	response, err := NextRPC(ctx, dec)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, rpcOut.ID, qt.Equals, response.ID)

	ack, err := nodeToRpcResponse(response.Data)
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, ack.PingAck, qt.IsNotNil)
	qt.Assert(t, ack.ModuleStatusAnswer, qt.IsNil)
	qt.Assert(t, ack.PingAck.CallID, qt.Equals, request.CallID)
}

func TestServerModuleStatus(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(func() { cancel() })
	ctx, flushLogs := NewLogBuffers(t, ctx)
	t.Cleanup(func() { flushLogs() })

	listener := NewPipeListener(ctx)
	t.Cleanup(func() { listener.Close() })

	hist := &historian{}
	srv := &server{
		listener: listener,
		handler:  handler{statusFetcher: hist.getStatus},
	}
	hist.setStatus("foobar", map[string]string{}, workspaceapi.ModuleStatus_ExecutedSuccess)
	go srv.serve(ctx)

	conn, err := listener.Dial(ctx)
	qt.Assert(t, err, qt.IsNil)
	t.Cleanup(func() { conn.Close() })
	dec := json.NewDecoder(conn)

	doCall := func(id string, query workspaceapi.ModuleStatusQuery, expected workspaceapi.ModuleStatusAnswer) {
		request := workspaceapi.RpcRequest{
			ModuleStatusQuery: &query,
		}
		dataNode := bindnode.Wrap(&request, workspaceapi.TypeSystem.TypeByName("RpcRequest"))
		rpcOut := workspaceapi.Rpc{
			ID:   id,
			Data: dataNode,
		}
		err = ipld.MarshalStreaming(conn, ipldjson.Encode, &rpcOut, workspaceapi.TypeSystem.TypeByName("Rpc"))
		qt.Assert(t, err, qt.IsNil)

		response, err := NextRPC(ctx, dec)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, rpcOut.ID, qt.Equals, response.ID)

		ack, err := nodeToRpcResponse(response.Data)
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, ack, qt.IsNotNil)
		qt.Assert(t, ack.PingAck, qt.IsNil)
		qt.Assert(t, ack.ModuleStatusAnswer, qt.IsNotNil)
		qt.Assert(t, ack.ModuleStatusAnswer, qt.DeepEquals, expected)
	}

	doCall("0",
		workspaceapi.ModuleStatusQuery{
			Path:          "foobar",
			InterestLevel: workspaceapi.ModuleInterestLevel_Query,
		},
		workspaceapi.ModuleStatusAnswer{
			Path:   "foobar",
			Status: workspaceapi.ModuleStatus_ExecutedSuccess,
		},
	)
	doCall("1",
		workspaceapi.ModuleStatusQuery{
			Path:          "foobargrill",
			InterestLevel: workspaceapi.ModuleInterestLevel_Query,
		},
		workspaceapi.ModuleStatusAnswer{
			Path:   "foobargrill",
			Status: workspaceapi.ModuleStatus_NoInfo,
		},
	)
}
