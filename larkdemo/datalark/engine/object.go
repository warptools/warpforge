package datalarkengine

import (
	"fmt"

	"go.starlark.net/starlark"
)

// Object is a starlark.Value that contains arbitrary attributes (like a starlark.Dict),
// and also lets you access things with dot notation (e.g. as "attrs") in addition to with map key notation.
//
// We use this for creating hierarchical namespaces.
// (For example, this is how we make typed constructors available without cluttering global namespaces.)
type Object starlark.Dict

func NewObject(size int) *Object {
	return (*Object)(starlark.NewDict(size))
}

func (d *Object) Get(k starlark.Value) (v starlark.Value, found bool, err error) {
	return ((*starlark.Dict)(d)).Get(k)
}
func (d *Object) Items() []starlark.Tuple          { return ((*starlark.Dict)(d)).Items() }
func (d *Object) Keys() []starlark.Value           { return ((*starlark.Dict)(d)).Keys() }
func (d *Object) Len() int                         { return ((*starlark.Dict)(d)).Len() }
func (d *Object) Iterate() starlark.Iterator       { return ((*starlark.Dict)(d)).Iterate() }
func (d *Object) SetKey(k, v starlark.Value) error { return ((*starlark.Dict)(d)).SetKey(k, v) }
func (d *Object) String() string                   { return ((*starlark.Dict)(d)).String() }
func (d *Object) Type() string                     { return "object" }
func (d *Object) Freeze()                          { ((*starlark.Dict)(d)).Freeze() }
func (d *Object) Truth() starlark.Bool             { return ((*starlark.Dict)(d)).Truth() }
func (d *Object) Hash() (uint32, error)            { return 0, fmt.Errorf("unhashable type: object") }

var _ starlark.HasAttrs = (*Object)(nil)

func (g *Object) Attr(name string) (starlark.Value, error) {
	v, found, err := g.Get(starlark.String(name))
	if !found {
		return nil, nil
	}
	return v, err
}
func (g *Object) AttrNames() []string {
	res := make([]string, 0, g.Len())
	itr := g.Iterate()
	defer itr.Done()
	var k starlark.Value
	for itr.Next(&k) {
		ks, _ := starlark.AsString(k)
		res = append(res, ks)
	}
	return res
}
