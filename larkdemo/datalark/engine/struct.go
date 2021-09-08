package datalarkengine

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

func ConstructStruct(npt schema.TypedPrototype, _ *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Parsing args for struct construction is *very* similar to for maps...
	//  Except structs also allow positional arguments; maps can't make sense of that.

	// Try parsing two different ways: either positional, or kwargs (but not both).
	nb := npt.NewBuilder()
	switch {
	case len(args) > 0 && len(kwargs) > 0:
		return starlark.None, fmt.Errorf("datalark.Struct: can either use positional or keyword arguments, but not both")

	case len(kwargs) > 0:
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

	case len(args) == 0:
		// Well, okay.  Hope the whole struct is optional fields though, or you're probably gonna get a schema validation error.
		ma, err := nb.BeginMap(0)
		if err != nil {
			return starlark.None, err
		}
		if err := ma.Finish(); err != nil {
			return starlark.None, err
		}

	case len(args) == 1:
		// If there's one arg, and it's a starlark dict, 'assignish' will do the right thing and restructure that into us.
		if err := assignish(nb, args[0]); err != nil {
			return starlark.None, fmt.Errorf("datalark.Struct: %w", err)
		}

	case len(args) > 1:
		return starlark.None, fmt.Errorf("datalark.Struct: if using positional arguments, only one is expected: a dict which we can restructure to match this type")

	default:
		panic("unreachable")
	}
	return &Struct{nb.Build()}, nil
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
	return fmt.Sprintf("datalark.Struct<%T>", g.val.(schema.TypedNode).Type().Name())
}
func (g *Struct) String() string {
	return printer.Sprint(g.val)
}
func (g *Struct) Freeze() {}
func (g *Struct) Truth() starlark.Bool {
	return true
}
func (g *Struct) Hash() (uint32, error) {
	// Riffing off Starlark's algorithm for Tuple, which is in turn riffing off Python.
	var x, mult uint32 = 0x345678, 1000003
	l := g.val.Length()
	for itr := g.val.MapIterator(); !itr.Done(); {
		_, v, err := itr.Next()
		if err != nil {
			return 0, err
		}
		w, err := Wrap(v)
		if err != nil {
			return 0, err
		}
		y, err := w.Hash()
		if err != nil {
			return 0, err
		}
		x = x ^ y*mult
		mult += 82520 + uint32(l+l)
	}
	return x, nil
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
