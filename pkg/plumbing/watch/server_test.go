package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestServerModuleStatus(t *testing.T) {
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
		binder: binder{
			historian: &historian{status: workspaceapi.ModuleStatus_ExecutedSuccess},
		},
	}
	now := time.Now()
	ctx, cancel := context.WithDeadline(ctx, now.Add(5*time.Second))
	defer cancel()
	go srv.serve(ctx)

	conn, err := jsonrpc2.Dial(ctx, listener.Dialer(), jsonrpc2.ConnectionOptions{})
	qt.Assert(t, err, qt.IsNil)

	query := &workspaceapi.ModuleStatusQuery{
		Path:          "foobar",
		InterestLevel: workspaceapi.ModuleInterestLevel_Query,
	}
	data, err := ipld.Marshal(ipldjson.Encode, query, workspaceapi.TypeSystem.TypeByName("ModuleStatusQuery"))
	qt.Assert(t, err, qt.IsNil)
	async := conn.Call(ctx, workspaceapi.RpcModuleStatus, json.RawMessage(data))

	var msg json.RawMessage
	err = async.Await(ctx, &msg)
	qt.Assert(t, err, qt.IsNil)

	expected := workspaceapi.ModuleStatusAnswer{
		Path:   query.Path,
		Status: workspaceapi.ModuleStatus_ExecutedSuccess,
	}
	var result workspaceapi.ModuleStatusAnswer
	_, err = ipld.Unmarshal([]byte(msg), ipldjson.Decode, &result, workspaceapi.TypeSystem.TypeByName("ModuleStatusAnswer"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.DeepEquals, expected)
}
