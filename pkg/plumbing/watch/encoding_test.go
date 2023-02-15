package watch

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"

	"github.com/warptools/warpforge/pkg/workspaceapi"
)

// Tests that the streaming ipld API can be used
func TestRoundtrip_Streaming(t *testing.T) {
	var err error
	buf := &bytes.Buffer{}
	echo := workspaceapi.Echo("foobar")
	request := workspaceapi.Rpc{
		ID:   "1",
		Data: workspaceapi.RpcData{RpcResponse: &workspaceapi.RpcResponse{Echo: &echo}},
	}
	err = ipld.MarshalStreaming(buf, Encoder, &request, workspaceapi.TypeSystem.TypeByName("Rpc"))
	qt.Assert(t, err, qt.IsNil)
	t.Log("write:\n", buf.String())
	r, w := io.Pipe()
	go io.Copy(w, buf)
	var uut workspaceapi.Rpc
	t.Cleanup(func() { w.Close(); r.Close() })
	ch := make(chan struct{})
	go func() {
		_, err = ipld.UnmarshalStreaming(r, Decoder, &uut, workspaceapi.TypeSystem.TypeByName("Rpc"))
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

// Tests pre-reading a json object. This would be useful to separate schema validation errors from "sent a json object" errors.
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

// Tests hypothesis that a stream which receives a bad object can receive a valid object afterwards.
//
// I.E. A valid JSON object is sent over the connection but that JSON object does not conform to the RPC schema.
// Then a valid JSON object which conforms to the RPC schema is sent and can be parsed by the receiving end.
//
// NOTE: This doesn't work. This means that for a stream which receives a bad object, the primary recovery method
// is to break the connection and have the client retry. Alternatively we could try to blindly consume whatever data is on the stream.
func TestStreaming_RecoverFromBadData(t *testing.T) {
	buf := &bytes.Buffer{}
	t.Cleanup(func() { buf = nil })
	_, err := buf.WriteString(`{"foo": "bar"}`)
	qt.Assert(t, err, qt.IsNil)
	var uut workspaceapi.Rpc
	sch := workspaceapi.TypeSystem.TypeByName("Rpc")
	n, err := ipld.UnmarshalStreaming(buf, Decoder, &uut, sch)
	qt.Assert(t, err, qt.IsNotNil, qt.Commentf("invalid object should cause unmarshal to error"))
	qt.Assert(t, n, qt.IsNil, qt.Commentf("invalid objects should not return data nodes"))

	echo := workspaceapi.Echo("foobar")
	request := workspaceapi.Rpc{
		ID:   "1",
		Data: workspaceapi.RpcData{RpcResponse: &workspaceapi.RpcResponse{Echo: &echo}},
	}
	err = ipld.MarshalStreaming(buf, Encoder, &request, sch)
	qt.Assert(t, err, qt.IsNil)

	_, err = ipld.UnmarshalStreaming(buf, Decoder, &uut, sch)
	// NOTE: This does NOT work. A stream that receives bad data can not recover.
	// qt.Assert(t, err, qt.IsNil)
	// qt.Assert(t, uut, qt.CmpEquals(), request)
	qt.Assert(t, err, qt.IsNotNil, qt.Commentf("ideally this would fail and the stream could recover as long as the client sent a json object."))
}
