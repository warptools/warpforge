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
	*Ping
	*ModuleStatusQuery
}

type RpcResponse struct {
	*PingAck
	*ModuleStatusAnswer
	Error *serum.ErrorValue
}

type Rpc struct {
	ID        string
	Data      datamodel.Node
	ErrorCode *string
}
