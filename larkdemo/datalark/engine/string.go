package datalarkengine

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

func ConstructString(np datamodel.NodePrototype, _ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var val string
	if err := starlark.UnpackPositionalArgs("datalark.String", args, kwargs, 1, &val); err != nil {
		return nil, err
	}

	nb := np.NewBuilder()
	if err := nb.AssignString(val); err != nil {
		return nil, err
	}
	return Wrap(nb.Build())
}

type String struct {
	val datamodel.Node
}

func (g *String) Node() datamodel.Node {
	return g.val
}
func (g *String) Type() string {
	if tn, ok := g.val.(schema.TypedNode); ok {
		return fmt.Sprintf("datalark.String<%T>", tn.Type().Name())
	}
	return fmt.Sprintf("datalark.String")
}
func (g *String) String() string {
	return printer.Sprint(g.val)
}
func (g *String) Freeze() {}
func (g *String) Truth() starlark.Bool {
	return true
}
func (g *String) Hash() (uint32, error) {
	s, _ := g.val.AsString()
	return starlark.String(s).Hash()
}
