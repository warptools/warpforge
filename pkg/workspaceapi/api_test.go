package workspaceapi

import (
	"fmt"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
)

func TestTypeSystemCompiles(t *testing.T) {
	if errs := TypeSystem.ValidateGraph(); errs != nil {
		qt.Assert(t, errs, qt.IsNil)
	}
}

// file exists just to make sure some test files exist, and thus package init is exercised, and thus we test it doesn't panic.
// drop the above comment when we get more actual test content.

func TestModuleStatusQuerySerialization(t *testing.T) {
	query := ModuleStatusQuery{
		Path:          "a string",
		InterestLevel: ModuleInterestLevel_Query,
	}
	data, err := ipld.Marshal(ipldjson.Encode, &query, TypeSystem.TypeByName("ModuleStatusQuery"))
	qt.Assert(t, err, qt.IsNil)

	var result ModuleStatusQuery
	_, err = ipld.Unmarshal(data, ipldjson.Decode, &result, TypeSystem.TypeByName("ModuleStatusQuery"))
	qt.Assert(t, err, qt.IsNil)
}

func TestModuleStatusAnswerSerialization(t *testing.T) {
	input := ModuleStatusAnswer{
		Path:   "a string",
		Status: ModuleStatus_NoInfo,
	}
	data, err := ipld.Marshal(ipldjson.Encode, &input, TypeSystem.TypeByName("ModuleStatusAnswer"))
	qt.Assert(t, err, qt.IsNil)

	var result ModuleStatusAnswer
	_, err = ipld.Unmarshal(data, ipldjson.Decode, &result, TypeSystem.TypeByName("ModuleStatusAnswer"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.DeepEquals, input)
}

func TestRpcRequest(t *testing.T) {
	input := RpcRequest{
		ModuleStatusQuery: &ModuleStatusQuery{
			Path:          "a string",
			InterestLevel: ModuleInterestLevel_Query,
		},
	}
	data, err := ipld.Marshal(ipldjson.Encode, &input, TypeSystem.TypeByName("RpcRequest"))
	qt.Assert(t, err, qt.IsNil)

	var result RpcRequest
	_, err = ipld.Unmarshal(data, ipldjson.Decode, &result, TypeSystem.TypeByName("RpcRequest"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.DeepEquals, input)
}

func TestRpcResponse(t *testing.T) {
	input := RpcResponse{
		ModuleStatusAnswer: &ModuleStatusAnswer{
			Path:   "a string",
			Status: ModuleStatus_NoInfo,
		},
	}
	data, err := ipld.Marshal(ipldjson.Encode, &input, TypeSystem.TypeByName("RpcResponse"))
	qt.Assert(t, err, qt.IsNil)

	var result RpcResponse
	_, err = ipld.Unmarshal(data, ipldjson.Decode, &result, TypeSystem.TypeByName("RpcResponse"))
	qt.Assert(t, err, qt.IsNil)
	qt.Assert(t, result, qt.DeepEquals, input)
}

func TestRpc(t *testing.T) {
	for _, testcase := range []struct {
		input Rpc
	}{
		{
			input: Rpc{
				ID: "request-ping-foo",
				Data: bindnode.Wrap(&RpcRequest{
					Ping: &Ping{CallID: "foo"},
				}, TypeSystem.TypeByName("RpcRequest")),
			},
		},
		{
			input: Rpc{
				ID: "request-status-foo",
				Data: bindnode.Wrap(&RpcRequest{
					ModuleStatusQuery: &ModuleStatusQuery{Path: "foo", InterestLevel: ModuleInterestLevel_Query},
				}, TypeSystem.TypeByName("RpcRequest")),
			},
		},
		{
			input: Rpc{
				ID: "response-ping-foo",
				Data: bindnode.Wrap(&RpcResponse{
					PingAck: &PingAck{CallID: "foo"},
				}, TypeSystem.TypeByName("RpcResponse")),
			},
		},
		{
			input: Rpc{
				ID: "response-status-foo",
				Data: bindnode.Wrap(&RpcResponse{
					ModuleStatusAnswer: &ModuleStatusAnswer{Path: "foo", Status: ModuleStatus_ExecutedSuccess},
				}, TypeSystem.TypeByName("RpcResponse")),
			},
		},
	} {
		t.Run(testcase.input.ID, func(t *testing.T) {
			testcase := testcase

			data, err := ipld.Marshal(ipldjson.Encode, &testcase.input, TypeSystem.TypeByName("Rpc"))
			qt.Assert(t, err, qt.IsNil)
			t.Logf("\n%s", string(data))

			var output Rpc
			_, err = ipld.Unmarshal(data, ipldjson.Decode, &output, TypeSystem.TypeByName("Rpc"))
			qt.Assert(t, err, qt.IsNil)
			qt.Assert(t, output.ID, qt.Equals, testcase.input.ID)
			t.Logf("\n%#v", output.Data)

			tn := testcase.input.Data.(schema.TypedNode)
			typeName := tn.Type().Name()
			switch typeName {
			case "RpcRequest":
				uut := &RpcRequest{}
				err := output.ExtractData(uut)
				qt.Assert(t, err, qt.IsNil)
				expect := bindnode.Unwrap(testcase.input.Data).(*RpcRequest)
				qt.Assert(t, *uut, qt.CmpEquals(), *expect)

				// i, err := bindnodeCopy(output.Data, uut, tn.Type())
				// qt.Assert(t, err, qt.IsNil)
				// uut = i.(*RpcRequest)
				// expect := bindnode.Unwrap(testcase.input.Data).(*RpcRequest)
				// qt.Assert(t, *uut, qt.Equals, *expect)
			case "RpcResponse":
				uut := &RpcResponse{}
				err := output.ExtractData(uut)
				qt.Assert(t, err, qt.IsNil)
				expect := bindnode.Unwrap(testcase.input.Data).(*RpcResponse)
				qt.Assert(t, *uut, qt.CmpEquals(), *expect)

				// i, err := bindnodeCopy(output.Data, uut, tn.Type())
				// qt.Assert(t, err, qt.IsNil)
				// uut = i.(*RpcResponse)
				// expect := bindnode.Unwrap(testcase.input.Data).(*RpcResponse)
				// qt.Assert(t, *uut, qt.Equals, *expect)
			default:
				t.Fatalf("invalid typename: %q", typeName)
			}
		})
	}
}

func TestRegenerate(t *testing.T) {
	t.Skip("shouldn't need to regenerate types, but might be useful to get a quick idea of what a new struct should look like")
	GenerateSchemaTypes()
}

// helper function to regenerate data types
func GenerateSchemaTypes() {
	f, err := os.Create("_types.go")
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(f, "package workspaceapi\n\n")
	if err := bindnode.ProduceGoTypes(f, TypeSystem); err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
}

func TestModuleStatusUnion(t *testing.T) {
	ms := ModuleStatusUnion{ModuleStatusUnion_NoInfo: &ModuleStatusUnion_NoInfo{}}
	result := ms.Type()
	qt.Assert(t, result, qt.Equals, "ModuleStatusUnion_NoInfo")
	typ := TypeSystem.TypeByName(result)
	qt.Assert(t, typ, qt.IsNotNil)
}
