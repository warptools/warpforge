package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"golang.org/x/exp/jsonrpc2"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspaceapi"
)

func TestServerPing(t *testing.T) {
	ctx := context.Background()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	logger := logging.NewLogger(stdout, stderr, false, false, true)
	ctx = logger.WithContext(ctx)
	t.Cleanup(func() {
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
	})
	_, err := io.WriteString(stdout, "test\n")
	qt.Assert(t, err, qt.IsNil)
	logger.Info("tag", "test")

	listener, err := jsonrpc2.NetPipe(ctx)
	defer listener.Close()
	qt.Assert(t, err, qt.IsNil)

	srv := &server{
		listener: listener,
		binder:   binder{},
	}
	now := time.Now()
	ctx, cancel := context.WithDeadline(ctx, now.Add(5*time.Second))
	defer cancel()
	go srv.serve(ctx)

	conn, err := jsonrpc2.Dial(ctx, listener.Dialer(), jsonrpc2.ConnectionOptions{})
	qt.Assert(t, err, qt.IsNil)
	defer conn.Close()

	query := workspaceapi.Ping{CallID: "foobar"}
	data, err := ipld.Marshal(ipldjson.Encode, &query, workspaceapi.TypeSystem.TypeByName("Ping"))
	qt.Assert(t, err, qt.IsNil)
	async := conn.Call(ctx, workspaceapi.RpcPing, json.RawMessage(data))

	var msg json.RawMessage
	err = async.Await(ctx, &msg)
	qt.Assert(t, err, qt.IsNil)

	var result workspaceapi.PingAck
	_, err = ipld.Unmarshal([]byte(msg), ipldjson.Decode, &result, workspaceapi.TypeSystem.TypeByName("PingAck"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result.CallID, qt.Equals, query.CallID)
}

func Recover(t *testing.T, f interface{}) {
	defer func() {
		if err := recover(); err != nil {
			t.Log(err)
		}
	}()
	v := reflect.ValueOf(f)
	qt.Assert(t, v.Kind(), qt.Equals, reflect.Func)
	v.Call(nil) // we're doing the absolutely most based thing possible here. we don't care about function signature AT ALL
}

func TestServerShutdown(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	listener, err := jsonrpc2.NetPipe(ctx)
	qt.Assert(t, err, qt.IsNil)

	hist := &historian{}
	srv := &server{
		listener: listener,
		binder: binder{
			historian: hist,
		},
	}

	done := make(chan struct{})
	go func(ch chan<- struct{}) { srv.serve(ctx); ch <- struct{}{} }(done) // this is a lot of crap to ensure that we can actually shut down the server.

	conn, err := jsonrpc2.Dial(ctx, listener.Dialer(), jsonrpc2.ConnectionOptions{})
	qt.Assert(t, err, qt.IsNil)

	cancel()
	conn.Close()
	listener.Close()
	<-done
}

func TestServerModuleStatus(t *testing.T) {
	ctx := context.Background()
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	logger := logging.NewLogger(stdout, stderr, false, false, true)
	ctx = logger.WithContext(ctx)
	now := time.Now()
	ctx, cancel := context.WithDeadline(ctx, now.Add(5*time.Second))
	defer cancel()
	t.Cleanup(func() {
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
	})
	_, err := io.WriteString(stdout, "test\n")
	qt.Assert(t, err, qt.IsNil)
	logger.Info("tag", "test")

	listener, err := jsonrpc2.NetPipe(ctx)
	defer Recover(t, listener.Close)
	qt.Assert(t, err, qt.IsNil)

	hist := &historian{}
	srv := &server{
		listener: listener,
		binder: binder{
			historian: hist,
		},
	}
	hist.SetStatus("foobar", map[string]string{}, workspaceapi.ModuleStatus_ExecutedSuccess)

	done := make(chan struct{})
	go func(ch chan<- struct{}) { srv.serve(ctx); ch <- struct{}{} }(done) // this is a lot of crap to ensure that we can actually shut down the server.

	conn, err := jsonrpc2.Dial(ctx, listener.Dialer(), jsonrpc2.ConnectionOptions{})
	qt.Assert(t, err, qt.IsNil)
	defer func() { defer recover(); conn.Close() }()

	doCall := func(query workspaceapi.ModuleStatusQuery, expected workspaceapi.ModuleStatusAnswer) {
		data, err := ipld.Marshal(ipldjson.Encode, &query, workspaceapi.TypeSystem.TypeByName("ModuleStatusQuery"))
		qt.Assert(t, err, qt.IsNil)
		async := conn.Call(ctx, workspaceapi.RpcModuleStatus, json.RawMessage(data))

		var msg json.RawMessage
		err = async.Await(ctx, &msg)
		qt.Assert(t, err, qt.IsNil)

		var result workspaceapi.ModuleStatusAnswer
		_, err = ipld.Unmarshal([]byte(msg), ipldjson.Decode, &result, workspaceapi.TypeSystem.TypeByName("ModuleStatusAnswer"))
		qt.Assert(t, err, qt.IsNil)
		qt.Assert(t, result, qt.DeepEquals, expected)
	}

	doCall(
		workspaceapi.ModuleStatusQuery{
			Path:          "foobar",
			InterestLevel: workspaceapi.ModuleInterestLevel_Query,
		},
		workspaceapi.ModuleStatusAnswer{
			Path:   "foobar",
			Status: workspaceapi.ModuleStatus_ExecutedSuccess,
		},
	)
	doCall(
		workspaceapi.ModuleStatusQuery{
			Path:          "foobargrill",
			InterestLevel: workspaceapi.ModuleInterestLevel_Query,
		},
		workspaceapi.ModuleStatusAnswer{
			Path:   "foobargrill",
			Status: workspaceapi.ModuleStatus_NoInfo,
		},
	)
	conn.Close()
	cancel()
	listener.Close()
	<-done
}
