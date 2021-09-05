package datalarkengine

import (
	"errors"
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

type StructPrototype struct {
	np schema.TypedPrototype
}

func (g *StructPrototype) Type() string {
	return fmt.Sprintf("datalark_prototype_struct<%T>", g.np.Type().Name())
}
func (g *StructPrototype) String() string        { return g.Type() }
func (g *StructPrototype) Freeze()               {}
func (g *StructPrototype) Truth() starlark.Bool  { return true }
func (g *StructPrototype) Hash() (uint32, error) { return 0, nil }

func ConstructStruct(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	npt := b.Receiver().(*StructPrototype).np

	// Try parsing two different ways: either positional, or kwargs (but not both).
	switch {
	case len(args) > 0 && len(kwargs) > 0:
		return starlark.None, fmt.Errorf("ConstructStruct: can either use positional or keyword arguments, but not both")

	case len(args) > 0:
		// TODO dang, can't use starlark.UnpackPositionalArgs generically either.
		//  ... maybe, with clever use of "unpacker" wrappers?  unsure, maybe worth looking into.
		panic("positional args nyi")

	case len(kwargs) > 0:
		nb := npt.NewBuilder()
		ma, err := nb.BeginMap(int64(len(kwargs)))
		if err != nil {
			return starlark.None, err
		}
		for _, kwarg := range kwargs {
			if err := assignish(ma.AssembleKey(), kwarg[0]); err != nil {
				return starlark.None, err
			}
			if err := assignish(ma.AssembleValue(), kwarg[1]); err != nil {
				return starlark.None, err
			}
		}
		if err := ma.Finish(); err != nil {
			return starlark.None, err
		}
		return &Struct{nb.Build()}, nil

	default:
		panic("unreachable")
	}
}

func WrapStruct(val datamodel.Node) (*Struct, error) {
	if tn, ok := val.(schema.TypedNode); !ok {
		return nil, fmt.Errorf("WrapStruct must be used on a typed node!")
	} else {
		if tn.Type().TypeKind() != schema.TypeKind_Struct {
			return nil, fmt.Errorf("WrapStruct must be used on a node with typekind 'struct'!")
		}
	}
	return &Struct{val}, nil
}

type Struct struct {
	val datamodel.Node
}

func (g *Struct) Node() datamodel.Node {
	return g.val
}
func (g *Struct) Type() string {
	return fmt.Sprintf("datalark_struct<%T>", g.val.(schema.TypedNode).Type().Name())
}
func (g *Struct) String() string {
	return printer.Sprint(g.val)
}
func (g *Struct) Freeze() {}
func (g *Struct) Truth() starlark.Bool {
	return true
}
func (g *Struct) Hash() (uint32, error) {
	return 0, errors.New("TODO")
}

func (g *Struct) Attr(name string) (starlark.Value, error) {
	// TODO: distinction between 'Attr' and 'Get'.  This can/should list functions, I think.  'Get' makes it unambiguous.  I think.
	// TODO: perhaps also add a "__constr__" or "__proto__" function to everything?
	n, err := g.val.LookupByString(name)
	if err != nil {
		return nil, err
	}
	return Wrap(n)
}

func (g *Struct) AttrNames() []string {
	names := make([]string, 0, g.val.Length())
	for itr := g.val.MapIterator(); !itr.Done(); {
		k, _, err := itr.Next()
		if err != nil {
			panic(fmt.Errorf("error while iterating: %w", err)) // should *really* not happen for structs, incidentally.
		}
		ks, _ := k.AsString()
		names = append(names, ks)
	}
	return names
}

func (g *Struct) SetField(name string, val starlark.Value) error {
	return fmt.Errorf("datalark values are immutable")
}
