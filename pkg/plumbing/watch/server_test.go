package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/uuid"
	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/testutil/nettest"
	"github.com/warptools/warpforge/pkg/workspaceapi"
)

func NewLogBuffers(t *testing.T, ctx context.Context) (context.Context, func()) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	logger := logging.NewLogger(stdout, stderr, false, false, true)
	ctx = logger.WithContext(ctx)
	return ctx, func() {
		t.Log("---")
		t.Logf("flush stdout:\n%s", stdout.String())
		t.Logf("flush stderr:\n%s", stderr.String())
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

	listener := nettest.NewPipeListener(ctx)

	hist := &historian{}
	srv := &server{
		listener: listener,
		handler:  rpcHandler{statusFetcher: hist.getStatus},
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

	listener := nettest.NewPipeListener(ctx)
	t.Cleanup(func() { listener.Close() })

	srv := &server{
		listener: listener,
		handler:  rpcHandler{},
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

	listener := nettest.NewPipeListener(ctx)
	t.Cleanup(func() { listener.Close() })

	hist := &historian{}
	srv := &server{
		listener: listener,
		handler:  rpcHandler{statusFetcher: hist.getStatus},
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
