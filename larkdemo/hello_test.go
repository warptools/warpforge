package larkdemo

import (
	"fmt"

	"github.com/ipld/go-datalark"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"go.starlark.net/starlark"

	"github.com/warpfork/warpforge/wfapi"
)

func ExampleHello() {
	// Build the globals map that makes our API's surfaces available in starlark.
	globals := starlark.StringDict{}
	datalark.InjectGlobals(globals, datalark.ObjOfConstructorsForPrimitives())
	datalark.InjectGlobals(globals, datalark.ObjOfConstructorsForPrototypes(
		bindnode.Prototype((*wfapi.Plot)(nil), wfapi.TypeSystem.TypeByName("Plot")),
	))

	// Execute Starlark program in a file.
	thread := &starlark.Thread{Name: "thethreadname"}
	globals, err := starlark.ExecFile(thread, "thefilename.star", `
result = Plot(
	inputs={},
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
	// 	inputs: map<Map__LocalLabel__PlotInput>{}
	// 	steps: map<Map__StepName__Step>{}
	// 	outputs: map<Map__LocalLabel__PlotOutput>{}
	// }
}
