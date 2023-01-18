package workspaceapi

import (
	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/serum-errors/go-serum"
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

type RpcRequest struct {
	*Ping
	*ModuleStatusQuery
}

type RpcResponse struct {
	*PingAck
	*ModuleStatusAnswer
	*Error
}

type RpcData struct {
	*RpcRequest
	*RpcResponse
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

//Errors:
//
//   - warpforge-error-rpc-missing-data -- data has no fields
func (r *Rpc) ExtractData(out interface{}) error {
	if out == nil {
		panic("cannot pass nil reference to extract data")
	}
	iter := r.Data.MapIterator()
	key, value, err := iter.Next()
	if err != nil {
		return serum.Error(ECodeRpcMissingData, serum.WithCause(err))
	}
	if !iter.Done() {
		key2, _, err := iter.Next()
		return serum.Error(ECodeRpcExtraData, serum.WithCause(err),
			serum.WithMessageTemplate("node contained extra fields (key1: {{key1|q}}, key2: {{key2|q}}) and/or returned an error"),
			serum.WithDetail("key1", printer.Sprint(key)),
			serum.WithDetail("key2", printer.Sprint(key2)),
		)
	}
	keyStr, err := key.AsString()
	switch out := out.(type) {
	case *RpcRequest:
		switch keyStr {
		case "ping":
			cp, err := bindnodeCopy(value, out.Ping, TypeSystem.TypeByName("Ping"))
			if err != nil {
				return serum.Error(ECodeRpcSerialization, serum.WithCause(err))
			}
			out.Ping = cp.(*Ping)
			return nil
		case "module_status":
			cp, err := bindnodeCopy(value, out.ModuleStatusQuery, TypeSystem.TypeByName("ModuleStatusQuery"))
			if err != nil {
				return serum.Error(ECodeRpcSerialization, serum.WithCause(err))
			}
			out.ModuleStatusQuery = cp.(*ModuleStatusQuery)
			return nil
		default:
			return serum.Error(ECodeRpcExtraData,
				serum.WithMessageTemplate("unrecognized key: {{key|q}}"),
				serum.WithDetail("key", keyStr),
			)
		}
	case *RpcResponse:
		switch keyStr {
		case "ping":
			cp, err := bindnodeCopy(value, out.PingAck, TypeSystem.TypeByName("PingAck"))
			if err != nil {
				return serum.Error(ECodeRpcSerialization, serum.WithCause(err))
			}
			out.PingAck = cp.(*PingAck)
			return nil
		case "module_status":
			cp, err := bindnodeCopy(value, out.ModuleStatusAnswer, TypeSystem.TypeByName("ModuleStatusAnswer"))
			if err != nil {
				return serum.Error(ECodeRpcSerialization, serum.WithCause(err))
			}
			out.ModuleStatusAnswer = cp.(*ModuleStatusAnswer)
			return nil
		case "error":
			cp, err := bindnodeCopy(value, out.Error, TypeSystem.TypeByName("Error"))
			if err != nil {
				return serum.Error(ECodeRpcSerialization, serum.WithCause(err))
			}
			out.Error = cp.(*Error)
			return nil
		default:
			return serum.Error(ECodeRpcExtraData,
				serum.WithMessageTemplate("unrecognized key: {{key|q}}"),
				serum.WithDetail("key", keyStr),
			)
		}
	default:
		panic("Rpc.ExtractData must take an *RpcRequest or an *RpcResponse")
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
