package workspaceapi

import (
	"reflect"

	"github.com/warptools/warpforge/wfapi"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/serum-errors/go-serum"
)

const (
	ModuleInterestLevel_Query ModuleInterestLevel = "Query"
	ModuleInterestLevel_Run   ModuleInterestLevel = "Run"
)
const (
	ECodeRpcMethodNotFound = "warpforge-error-rpc-method-not-found"
	ECodeRpcInternal       = "warpforge-error-rpc-internal"
	ECodeRpcSerialization  = "warpforge-error-rpc-serialization"
	ECodeRpcUnknown        = "warpforge-error-rpc-unknown"
	ECodeRpcConnection     = "warpforge-error-rpc-connection"
	ECodeRpcMissingData    = "warpforge-error-rpc-missing-data"
	ECodeRpcExtraData      = "warpforge-error-rpc-extra-data"
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

type RpcResponse struct {
	*Echo
	*ModuleStatusAnswer
	*Error
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

// Warning: this function returns an _error struct_ which can mess with go nil types
func (e *Error) Serum() *serum.ErrorValue {
	if e == nil {
		return nil
	}
	var msg string
	if e.Message != nil {
		msg = *e.Message
	}
	return &serum.ErrorValue{
		Data: serum.Data{
			Code:    e.Code,
			Message: msg,
			Details: e.Details.Details(),
			Cause:   e.Cause.Serum(),
		},
	}
}

func bindnodeCopy(data datamodel.Node, i interface{}, t schema.Type) (interface{}, error) {
	np := bindnode.Prototype(i, t)
	nb := np.NewBuilder()
	if err := datamodel.Copy(data, nb); err != nil {
		return nil, err
	}
	result := bindnode.Unwrap(nb.Build())
	return result, nil
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

func UnionField(i interface{}) string {
	rv := reflect.ValueOf(i)
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
