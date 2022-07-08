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
		// Wishlist: if this was a little easier to bind all in one place.
		//  We do need to know the concrete types.  But we should usually only need to say that once in the whole program.
		//   ... ohhey, Rod introduced a bindnode registry recently.  That might be just the thing.
		bindnode.Prototype((*wfapi.Plot)(nil), wfapi.TypeSystem.TypeByName("Plot")),
		bindnode.Prototype((*wfapi.Step)(nil), wfapi.TypeSystem.TypeByName("Step")),
		bindnode.Prototype((*wfapi.Protoformula)(nil), wfapi.TypeSystem.TypeByName("Protoformula")),
		bindnode.Prototype((*wfapi.Action)(nil), wfapi.TypeSystem.TypeByName("Action")),
		bindnode.Prototype((*wfapi.Action_Script)(nil), wfapi.TypeSystem.TypeByName("Action_Script")),
	}))

	// Execute Starlark program in a file.
	thread := &starlark.Thread{Name: "thethreadname"}
	globals, err := starlark.ExecFile(thread, "thefilename.star", `
result = Plot(
	inputs={
		"one": "ware:tar:asdf",
		"two": "literal:foobar",
	},
	steps={
		"beep": Step(Protoformula( # FIXME should also work without 'Step(' wrapper, shouldn't it?
			inputs={
				"/": "pipe::one",
				"$FOO": "pipe::two",
			},
			action=Action(Action_Script( # FIXME handing in a type that's a union member here should work, without the 'Action(' wrapper.  currently doesn't?  appears to be trying to copy the content if I drop the 'Action(' constructor, which ends in the NodeAssembler for an Action getting the key "interpreter" thrown at it, which of course (correctly) doesn't fly.
				interpreter="/bin/bash", # todo soon, make a wrapper func where this kwarg has a default
				contents=["hey", "hello"],
				network=False,
			)),
			outputs={},
		))
	},
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
	// 	steps: map<Map__StepName__Step>{
	// 		string<StepName>{"beep"}: union<Step>{struct<Protoformula>{
	// 			inputs: map<Map__SandboxPort__PlotInput>{
	// 				union<SandboxPort>{string<SandboxPath>{""}}: union<PlotInput>{union<PlotInputSimple>{struct<Pipe>{
	// 					stepName: string<StepName>{""}
	// 					label: string<LocalLabel>{"one"}
	// 				}}}
	// 				union<SandboxPort>{string<SandboxVar>{"FOO"}}: union<PlotInput>{union<PlotInputSimple>{struct<Pipe>{
	// 					stepName: string<StepName>{""}
	// 					label: string<LocalLabel>{"two"}
	// 				}}}
	// 			}
	// 			action: union<Action>{struct<Action_Script>{
	// 				interpreter: string<String>{"/bin/bash"}
	// 				contents: list<List__String>{
	// 					0: string<String>{"hey"}
	// 					1: string<String>{"hello"}
	// 				}
	// 				network: bool<Bool>{false}
	// 			}}
	// 			outputs: map<Map__LocalLabel__GatherDirective>{}
	// 		}}
	// 	}
	// 	outputs: map<Map__LocalLabel__PlotOutput>{}
	// }
}
