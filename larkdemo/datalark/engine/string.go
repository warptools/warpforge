package datalarkengine

import (
	"errors"
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/node/basicnode"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

func ConstructString(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// TODO optionally, grab the type info we bound earlier (somehow) into 'b'?

	var str string
	if err := starlark.UnpackPositionalArgs("datalark.String", args, kwargs, 1, &str); err != nil {
		return nil, err
	}

	return WrapString(basicnode.NewString(str))
}

func WrapString(val datamodel.Node) (*String, error) {
	if val.Kind() != datamodel.Kind_String {
		return nil, fmt.Errorf("WrapString must be used on a node of kind 'string'!")
	}
	return &String{val}, nil
}

type String struct {
	val datamodel.Node
}

func (g *String) Node() datamodel.Node {
	return g.val
}
func (g *String) Type() string {
	if tn, ok := g.val.(schema.TypedNode); ok {
		return fmt.Sprintf("datalark_string<%T>", tn.Type().Name())
	}
	return fmt.Sprintf("datalark_string")
}
func (g *String) String() string {
	return printer.Sprint(g.val)
}
func (g *String) Freeze() {}
func (g *String) Truth() starlark.Bool {
	return true
}
func (g *String) Hash() (uint32, error) {
	return 0, errors.New("TODO")
}
