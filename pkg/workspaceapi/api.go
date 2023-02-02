package workspaceapi

import (
	"github.com/warptools/warpforge/wfapi"

	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/serum-errors/go-serum"
)

const (
	ModuleInterestLevel_Query ModuleInterestLevel = "Query"
	ModuleInterestLevel_Run   ModuleInterestLevel = "Run"
)

// ECodeRpc* errors have significance over the wire for the workspace api.
const (
	ECodeRpcConnection     = "warpforge-error-rpc-connection"       // ECodeRpcConnection for connection errors
	ECodeRpcInternal       = "warpforge-error-rpc-internal"         // ECodeRpcInternal means the RPC had an internal error
	ECodeRpcMethodNotFound = "warpforge-error-rpc-method-not-found" // ECodeRpcMethodNotFound means an RPC has an invalid request type
	ECodeRpcSerialization  = "warpforge-error-rpc-serialization"    // ECodeRpcSerialization errors due to serialization or deserialization of RPC data
	ECodeRpcUnknown        = "warpforge-error-rpc-unknown"          // ECodeRpcUnknown for unknown errors such as receiving an error response without a code.
)

const (
	ModuleStatus_NoInfo             ModuleStatus = "NoInfo"
	ModuleStatus_Queuing            ModuleStatus = "Queuing"
	ModuleStatus_InProgress         ModuleStatus = "InProgress"
	ModuleStatus_FailedProvisioning ModuleStatus = "FailedProvisioning"
	ModuleStatus_ExecutedSuccess    ModuleStatus = "ExecutedSuccess"
	ModuleStatus_ExecutedFailed     ModuleStatus = "ExecutedFailed"
)

type RpcRequest struct {
	*ModuleStatusQuery
}

// Errors:
//
//   - warpforge-error-unknown -- Unable to extract union key
func (r *RpcRequest) Kind() (string, error) {
	sch := TypeSystem.TypeByName("RpcRequest")
	return unionField(r, sch)
}

// Errors:
//
//   - warpforge-error-unknown -- Unable to extract union key
func unionField(i interface{}, sch schema.Type) (string, error) {
	n := bindnode.Wrap(i, sch)
	iter := n.MapIterator()
	key, _, err := iter.Next()
	if err != nil {
		return "", serum.Error(wfapi.ECodeUnknown, serum.WithCause(err))
	}
	result, err := key.AsString()
	if err != nil {
		return "", serum.Error(wfapi.ECodeUnknown, serum.WithCause(err))
	}
	return result, nil
}

type RpcResponse struct {
	*Echo
	*ModuleStatusAnswer
	*Error
}

// Errors:
//
//   - warpforge-error-unknown -- Unable to extract union key
func (r *RpcResponse) Kind() (string, error) {
	sch := TypeSystem.TypeByName("RpcResponse")
	return unionField(r, sch)
}

type RpcData struct {
	*RpcRequest
	*RpcResponse
}

type Rpc struct {
	ID   string
	Data RpcData
}

type Error struct {
	Code    string
	Message *string
	Details *Details
	Cause   *Error
}

type Details struct {
	Keys   []string
	Values map[string]string
}

func (d *Details) Details() [][2]string {
	if d == nil {
		return [][2]string{}
	}
	result := make([][2]string, 0, len(d.Keys))
	for _, key := range d.Keys {
		result = append(result, [2]string{key, d.Values[key]})
	}
	return result
}

func (e *Error) serum() *serum.ErrorValue {
	data := e.AsSerumData()
	if data == nil {
		return nil
	}
	return &serum.ErrorValue{Data: *data}
}

func (e *Error) AsSerumData() *serum.Data {
	if e == nil {
		return nil
	}
	var msg string
	if e.Message != nil {
		msg = *e.Message
	}
	return &serum.Data{
		Code:    e.Code,
		Message: msg,
		Details: e.Details.Details(),
		Cause:   e.Cause.serum(),
	}
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

type ModuleStatus string

type ModuleStatusAnswer struct {
	Path   string
	Status ModuleStatus
}

type Echo string

type ModuleStatusUnion struct {
	ModuleStatusUnion_NoInfo             *ModuleStatusUnion_NoInfo
	ModuleStatusUnion_Queuing            *ModuleStatusUnion_Queuing
	ModuleStatusUnion_InProgress         *ModuleStatusUnion_InProgress
	ModuleStatusUnion_FailedProvisioning *ModuleStatusUnion_FailedProvisioning
	ModuleStatusUnion_ExecutedSuccess    *ModuleStatusUnion_ExecutedSuccess
	ModuleStatusUnion_ExecutedFailed     *ModuleStatusUnion_ExecutedFailed
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
