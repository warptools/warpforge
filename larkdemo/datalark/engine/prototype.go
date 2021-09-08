package datalarkengine

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/datamodel"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

// Prototype wraps an IPLD `datamodel.NodePrototype`, and in starlark,
// is a `Callable` which acts like a constructor for that NodePrototype.
//
// There is only one Prototype type, and its behavior varies based on
// the `datamodel.NodePrototype` its bound to.
type Prototype struct {
	np datamodel.NodePrototype
}

// -- starlark.Value -->

var _ starlark.Value = (*Prototype)(nil)

func (g *Prototype) Type() string {
	if npt, ok := g.np.(schema.TypedPrototype); ok {
		return fmt.Sprintf("datalark.Prototype<%s>", npt.Type().Name())
	}
	return fmt.Sprintf("datalark.Prototype")
}
func (g *Prototype) String() string {
	return fmt.Sprintf("<built-in function %s>", g.Type())
}
func (g *Prototype) Freeze() {}
func (g *Prototype) Truth() starlark.Bool {
	return true
}
func (g *Prototype) Hash() (uint32, error) {
	return 0, nil
}

// -- starlark.Callable -->

var _ starlark.Callable = (*Prototype)(nil)

func (g *Prototype) Name() string {
	return g.String()
}

func (g *Prototype) CallInternal(thread *starlark.Thread, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// If we have a TypedPrototype, try the appropriate constructors for its typekind.
	if npt, ok := g.np.(schema.TypedPrototype); ok {
		switch npt.Type().TypeKind() {
		case schema.TypeKind_Struct:
			return ConstructStruct(npt, thread, args, kwargs)
		case schema.TypeKind_Map:
			return ConstructMap(npt, thread, args, kwargs)
		default:
			panic(fmt.Errorf("nyi: datalark.Prototype.CallInternal for typed nodes with typekind %s", npt.Type().TypeKind()))
		}
	}
	// If we have an untyped NodePrototype... just try whatever our args look like.
	nb := g.np.NewBuilder()
	switch {
	case len(args) > 0 && len(kwargs) > 0:
		return starlark.None, fmt.Errorf("datalark.Prototype.__call__: can either use positional or keyword arguments, but not both")
	case len(args) == 1:
		if err := assignish(nb, args[0]); err != nil {
			return starlark.None, fmt.Errorf("datalark.Prototype.__call__: %w", err)
		}
	case len(args) > 1:
		// TODO list
		panic("nyi")
	case len(kwargs) > 0:
		return ConstructMap(g.np, thread, args, kwargs)
	}
	return Wrap(nb.Build())
}

// FUTURE: We can choose to implement Attrs and GetAttr on this, if we want to expose the ability to introspect things or look at types from skylark!
