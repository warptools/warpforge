package workspaceapi

import (
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/serum-errors/go-serum"
)

const (
	ECodeRpcMethodNotFound = "warpforge-error-rpc-method-not-found"
	ECodeRpcInternal       = "warpforge-error-rpc-internal"
	ECodeRpcSerialization  = "warpforge-error-rpc-serialization"
	ECodeRpcUnknown        = "warpforge-error-rpc-unknown"
	ECodeRpcConnection     = "warpforge-error-rpc-connection"
)

type RpcRequest struct {
	Ping              *Ping
	ModuleStatusQuery *ModuleStatusQuery
}

type RpcResponse struct {
	PingAck            *PingAck
	ModuleStatusAnswer *ModuleStatusAnswer
	Error              *Error
}

type Rpc struct {
	ID   string
	Data datamodel.Node
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
