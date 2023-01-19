package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	ipld "github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	rfmtjson "github.com/polydawn/refmt/json"
	"github.com/serum-errors/go-serum"

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

// Errors:
//
//   - EOF --
//   - CONTEXT --
func (p *pipeListener) Accept() (net.Conn, error) {
	select {
	case <-p.done:
		return nil, serum.Error("EOF", serum.WithCause(io.EOF))
	case <-p.ctx.Done():
		return nil, serum.Error("CONTEXT", serum.WithCause(p.ctx.Err()))
	case conn := <-p.connections:
		return conn, nil
	}
}
func (p *pipeListener) Addr() net.Addr { return nil }

// Errors:
//
//   - CONTEXT --
func (p *pipeListener) Dial(ctx context.Context) (io.ReadWriteCloser, error) {
	serverConn, clientConn := net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	clientConn.SetDeadline(deadline) // will cause tests to fail if they block
	select {
	case <-ctx.Done():
		return nil, serum.Error("CONTEXT", serum.WithCause(ctx.Err()))
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
		t.Log("---")
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
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("test timeout")
	}
}

func streamEncode(n datamodel.Node, w io.Writer) error {
	return dagjson.Marshal(n, rfmtjson.NewEncoder(w, rfmtjson.EncodeOptions{
		Line:   []byte{'\n'},
		Indent: []byte{'\t'},
	}), dagjson.EncodeOptions{
		EncodeLinks: false,
		EncodeBytes: false,
		MapSortMode: codec.MapSortMode_None,
	})
}

func streamDecode(na datamodel.NodeAssembler, r io.Reader) error {
	return dagjson.DecodeOptions{
		ParseLinks:         false,
		ParseBytes:         false,
		DontParseBeyondEnd: true, // This is critical for streaming over a socket
	}.Decode(na, r)
}

func TestRoundtrip_Streaming(t *testing.T) {
	var err error
	buf := &bytes.Buffer{}
	echo := workspaceapi.Echo("foobar")
	request := workspaceapi.Rpc{
		ID:   "1",
		Data: workspaceapi.RpcData{RpcResponse: &workspaceapi.RpcResponse{Echo: &echo}},
	}
	err = ipld.MarshalStreaming(buf, streamEncode, &request, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)
	t.Log("write:\n", buf.String())
	r, w := io.Pipe()
	go io.Copy(w, buf)
	var uut workspaceapi.Rpc
	t.Cleanup(func() { w.Close(); r.Close() })
	ch := make(chan struct{})
	go func() {
		_, err = ipld.UnmarshalStreaming(r, streamDecode, &uut, workspaceapi.TypeSystem.TypeByName("Rpc"))
		ch <- struct{}{}
	}()
	select {
	case <-time.After(time.Second):
		t.Fatalf("test timeout")
	case <-ch:
	}
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, uut, qt.CmpEquals(), request)
}

func TestRoundtrip_PreReadJson(t *testing.T) {
	var err error
	buf := &bytes.Buffer{}
	echo := workspaceapi.Echo("foobar")
	rpcOut := workspaceapi.Rpc{
		ID:   "1",
		Data: workspaceapi.RpcData{RpcResponse: &workspaceapi.RpcResponse{Echo: &echo}},
	}
	err = ipld.MarshalStreaming(buf, ipldjson.Encode, &rpcOut, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)

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
	t.Log("read:\n", string(raw))

	uut := workspaceapi.Rpc{}
	_, err = ipld.Unmarshal(raw, ipldjson.Decode, &uut, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, uut, qt.CmpEquals(), rpcOut)
}

type rpcPromise struct {
	*workspaceapi.Rpc
	error
}

func promiseNextRpc(ctx context.Context, d *json.Decoder) <-chan rpcPromise {
	ctx = setHandlerTag(ctx, "<-  test recv")
	ch := make(chan rpcPromise)
	go func() {
		result, err := NextRPC(ctx, d)
		ch <- rpcPromise{result, err}
	}()
	return ch
}

func TestServerEcho(t *testing.T) {
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

	echo := workspaceapi.Echo("foobar")
	request := workspaceapi.Rpc{
		ID:   uuid.New().String(),
		Data: workspaceapi.RpcData{RpcResponse: &workspaceapi.RpcResponse{Echo: &echo}},
	}

	t.Log("send request")
	buf := &bytes.Buffer{}
	err = ipld.MarshalStreaming(buf, ipldjson.Encode, &request, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)
	_, err = io.Copy(conn, buf)
	qt.Assert(t, err, qt.IsNil)

	t.Log("read reply")
	ch := promiseNextRpc(ctx, dec)
	select {
	case promise := <-ch:
		qt.Assert(t, promise.error, qt.IsNil)
		qt.Assert(t, *promise.Rpc, qt.CmpEquals(), request)
	case <-time.After(time.Second):
		t.Fatalf("test timeout")
	}
}

type tlog struct {
	logf func(format string, args ...interface{})
	tag  string
}

func (t tlog) Logf(format string, args ...interface{}) {
	t.logf(t.tag+" "+format, args...)
}
func (t tlog) TLogf(tag, format string, args ...interface{}) {
	t.logf(tag+" "+format, args...)
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
		t.Log("┌─ ")
		log := tlog{logf: t.Logf, tag: "│  " + id}
		rpcOut := workspaceapi.Rpc{
			ID:   id,
			Data: workspaceapi.RpcData{RpcRequest: &workspaceapi.RpcRequest{ModuleStatusQuery: &query}},
		}
		expectIn := workspaceapi.Rpc{
			ID:   id,
			Data: workspaceapi.RpcData{RpcResponse: &workspaceapi.RpcResponse{ModuleStatusAnswer: &expected}},
		}
		log.Logf("send request")
		data, err := ipld.Marshal(ipldjson.Encode, &rpcOut, workspaceapi.TypeSystem.TypeByName("Rpc"))
		qt.Assert(t, err, qt.IsNil)
		_, err = conn.Write(data)
		qt.Assert(t, err, qt.IsNil)

		log.Logf("read reply")
		ch := promiseNextRpc(ctx, dec)
		log.Logf("wait...")
		select {
		case promise := <-ch:
			log.Logf("assert")
			qt.Assert(t, promise.error, qt.IsNil)
			qt.Assert(t, *promise.Rpc, qt.CmpEquals(), expectIn)
		case <-time.After(time.Second):
			t.Fatalf("test timeout")
		}
		log.Logf("done")
		t.Log("└─ ")
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
