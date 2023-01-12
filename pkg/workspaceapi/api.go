package workspaceapi

import (
	"embed"
	"fmt"
	"reflect"

	"github.com/warptools/warpforge/wfapi"

	"github.com/ipld/go-ipld-prime/node/bindnode"

	"github.com/ipld/go-ipld-prime/datamodel"

	"github.com/ipld/go-ipld-prime/schema"
	schemadmt "github.com/ipld/go-ipld-prime/schema/dmt"
	schemadsl "github.com/ipld/go-ipld-prime/schema/dsl"
)

// Helper strings for JSON-RPC implementation.
const (
	RpcModuleStatus = "RpcModuleStatus"
	RpcPing         = "RpcPing"
)

// embed the wfapi ipld schema from file
//go:embed wfwsapi.ipldsch
var schFs embed.FS

var SchemaDMT, TypeSystem = func() (*schemadmt.Schema, *schema.TypeSystem) {
	r, err := schFs.Open("wfwsapi.ipldsch")
	if err != nil {
		panic(fmt.Sprintf("failed to open embedded wfwsapi.ipldsch: %s", err))
	}
	schemaDmt, err := schemadsl.Parse("wfwsapi.ipldsch", r)
	if err != nil {
		panic(fmt.Sprintf("failed to parse api schema: %s", err))
	}
	schemaDmt = concat(wfapi.SchemaDMT, schemaDmt)

	ts := new(schema.TypeSystem)
	ts.Init()
	if err := schemadmt.Compile(ts, schemaDmt); err != nil {
		panic(fmt.Sprintf("failed to compile api schema: %s", err))
	}
	return schemaDmt, ts
}()

// concat returns a new schemadmt that's got the types from both.
//
// This function could probably be hoisted upstream.
func concat(a, b *schemadmt.Schema) *schemadmt.Schema {
	nb := schemadmt.Type.Schema.NewBuilder()
	if err := datamodel.Copy(bindnode.Wrap(a, schemadmt.Type.Schema.Type()), nb); err != nil {
		panic(err)
	}
	if err := datamodel.Copy(bindnode.Wrap(b, schemadmt.Type.Schema.Type()), nb); err != nil {
		panic(err)
	}
	return bindnode.Unwrap(nb.Build()).(*schemadmt.Schema)
}

type ModuleStatusQuery struct {
	Path              string
	InputReplacements *InputReplacements
	InterestLevel     ModuleInterestLevel
}

type InputReplacements struct {
	Keys   []wfapi.PlotInput
	Values map[wfapi.PlotInput]wfapi.WareID
}

type ModuleInterestLevel string

const (
	ModuleInterestLevel_Query ModuleInterestLevel = "Query"
	ModuleInterestLevel_Run   ModuleInterestLevel = "Run"
)

type ModuleStatus string

type ModuleStatusAnswer struct {
	Path   string
	Status ModuleStatus
}

type Ping struct {
	CallID string
}

type PingAck struct {
	CallID string
}

const (
	ModuleStatus_NoInfo             ModuleStatus = "NoInfo"
	ModuleStatus_Queuing            ModuleStatus = "Queuing"
	ModuleStatus_InProgress         ModuleStatus = "InProgress"
	ModuleStatus_FailedProvisioning ModuleStatus = "FailedProvisioning"
	ModuleStatus_ExecutedSuccess    ModuleStatus = "ExecutedSuccess"
	ModuleStatus_ExecutedFailed     ModuleStatus = "ExecutedFailed"
)

type ModuleStatusUnion struct {
	ModuleStatusUnion_NoInfo             *ModuleStatusUnion_NoInfo
	ModuleStatusUnion_Queuing            *ModuleStatusUnion_Queuing
	ModuleStatusUnion_InProgress         *ModuleStatusUnion_InProgress
	ModuleStatusUnion_FailedProvisioning *ModuleStatusUnion_FailedProvisioning
	ModuleStatusUnion_ExecutedSuccess    *ModuleStatusUnion_ExecutedSuccess
	ModuleStatusUnion_ExecutedFailed     *ModuleStatusUnion_ExecutedFailed
}

func (ms ModuleStatusUnion) Type() string {
	rv := reflect.ValueOf(ms)
	unionIdx := -1
	var unionField reflect.Value
	for idx := 0; idx < rv.NumField(); idx++ {
		field := rv.Field(idx)
		if field.IsNil() {
			continue
		}
		if unionIdx == -1 {
			unionIdx = idx
			unionField = field
			continue
		}
		panic("union has multiple types")
	}
	if unionIdx == -1 {
		panic("union has no type")
	}
	return unionField.Type().Elem().Name()
}

type ModuleStatusUnion_NoInfo struct {
}
type ModuleStatusUnion_Queuing struct {
}
type ModuleStatusUnion_InProgress struct {
}
type ModuleStatusUnion_FailedProvisioning struct {
}
type ModuleStatusUnion_ExecutedSuccess struct {
}
type ModuleStatusUnion_ExecutedFailed struct {
}
