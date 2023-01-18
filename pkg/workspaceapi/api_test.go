package workspaceapi

import (
	"fmt"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	ipld "github.com/ipld/go-ipld-prime"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
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
