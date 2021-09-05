package datalarkengine

import (
	"fmt"

	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"
)

func Example_hello() {
	thread := &starlark.Thread{
		Name: "thethreadname",
		Print: func(thread *starlark.Thread, msg string) {
			//caller := thread.CallFrame(1)
			//fmt.Printf("%s: %s: %s\n", caller.Pos, caller.Name, msg)
			fmt.Printf("%s\n", msg)
		},
	}
	_, err := starlark.ExecFile(thread, "thefilename.star", `
print(ConstructString("yo"))
x = {"bz": "zoo"}
print(ConstructMap(hey="hai", zonk="wot", **x))
#print(ConstructMap({ConstructString("fun"): "heeey"}))
`,
		// FUTURE: may want to make a module, interpret it first, and then make it available in the globals...
		//  as a sheer way of getting dotted notation in a consistent way.
		//   We can also probably do this with a map and achieve similar effects, but that seems less cool.
		starlark.StringDict{
			"ConstructString": starlark.NewBuiltin("ConstructString", ConstructString),
			"ConstructMap":    starlark.NewBuiltin("ConstructMap", ConstructMap),
		})
	if err != nil {
		panic(err)
	}

	// Output:
	// string{"yo"}
	// map{
	// 	string{"hey"}: string{"hai"}
	// 	string{"zonk"}: string{"wot"}
	// 	string{"bz"}: string{"zoo"}
	// }
}

func Example_structs() {
	ts := schema.MustTypeSystem(
		schema.SpawnString("String"),
		schema.SpawnStruct("FooBar", []schema.StructField{
			schema.SpawnStructField("foo", "String", false, false),
			schema.SpawnStructField("bar", "String", false, false),
		}, nil),
	)
	type FooBar struct{ Foo, Bar string }
	npt := bindnode.Prototype((*FooBar)(nil), ts.TypeByName("FooBar"))

	thread := &starlark.Thread{
		Name: "thethreadname",
		Print: func(thread *starlark.Thread, msg string) {
			fmt.Printf("%s\n", msg)
		},
	}
	_, err := starlark.ExecFile(thread, "thefilename.star", `
print(ConstructStructFoo(foo="hai", bar="wot"))
`,
		starlark.StringDict{
			"ConstructStructFoo": starlark.NewBuiltin("ConstructStructFoo", ConstructStruct).BindReceiver(&StructPrototype{npt}),
		})
	if err != nil {
		panic(err)
	}

	// Output:
	// struct<FooBar>{
	// 	foo: string<String>{"hai"}
	// 	bar: string<String>{"wot"}
	// }
}
