package larkdemo

import (
	"fmt"

	"github.com/ipld/go-datalark"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"go.starlark.net/starlark"

	"github.com/warpfork/warpforge/wfapi"
)

func ExampleHello() {
	// Build the globals map that makes our API's surfaces available in starlark.
	globals := starlark.StringDict{}
	datalark.InjectGlobals(globals, datalark.PrimitiveConstructors())
	datalark.InjectGlobals(globals, datalark.MakeConstructors([]schema.TypedPrototype{
		bindnode.Prototype((*wfapi.Plot)(nil), wfapi.TypeSystem.TypeByName("Plot")),
	}))

	// Execute Starlark program in a file.
	thread := &starlark.Thread{Name: "thethreadname"}
	globals, err := starlark.ExecFile(thread, "thefilename.star", `
result = Plot(
	inputs={
		"one": "ware:tar:asdf",
		"two": "literal:foobar",
	},
	steps={},
	outputs={},
)
`, globals)
	if err != nil {
		panic(err)
	}

	// Retrieve a module global.  (This is probably not how we'll have warpforge's system extract results, but it's interesting.)
	fmt.Printf("result = %v\n", globals["result"])

	// Output:
	// result = struct<Plot>{
	// 	inputs: map<Map__LocalLabel__PlotInput>{
	// 		string<LocalLabel>{"one"}: union<PlotInput>{union<PlotInputSimple>{struct<WareID>{
	// 			packtype: string<Packtype>{"tar"}
	// 			hash: string<String>{"asdf"}
	// 		}}}
	// 		string<LocalLabel>{"two"}: union<PlotInput>{union<PlotInputSimple>{string<Literal>{"foobar"}}}
	// 	}
	// 	steps: map<Map__StepName__Step>{}
	// 	outputs: map<Map__LocalLabel__PlotOutput>{}
	// }
}
