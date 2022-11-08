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
		//   ... ohhey, Rod introduced a bindnode registry recently.  That might be just the thing.
		bindnode.Prototype((*wfapi.Plot)(nil), wfapi.TypeSystem.TypeByName("Plot")),
		bindnode.Prototype((*wfapi.Step)(nil), wfapi.TypeSystem.TypeByName("Step")),
		bindnode.Prototype((*wfapi.Protoformula)(nil), wfapi.TypeSystem.TypeByName("Protoformula")),
		bindnode.Prototype((*wfapi.Action)(nil), wfapi.TypeSystem.TypeByName("Action")),
		bindnode.Prototype((*wfapi.Action_Script)(nil), wfapi.TypeSystem.TypeByName("Action_Script")),
		bindnode.Prototype((*wfapi.SandboxPort)(nil), wfapi.TypeSystem.TypeByName("SandboxPort")),
		bindnode.Prototype((*wfapi.PlotInput)(nil), wfapi.TypeSystem.TypeByName("PlotInput")),
	}))

	// Execute Starlark program in a file.
	thread := &starlark.Thread{Name: "thethreadname"}
	globals, err := starlark.ExecFile(thread, "thefilename.star", `
print(SandboxPort("/"))
print(SandboxPort("/asdf"))
print(SandboxPort("$"))
print(SandboxPort("$FOO"))
#print(SandboxPort("FOO")) # this provokes the baddie, which is... okay, not expected.
print("---")

print({SandboxPort("$FOO"): ""})

print("---")
print(PlotInput("pipe:asdf:two"))
print(PlotInput("pipe::two"))

print("--- fine so far!? ---")
print(Protoformula(
	inputs={
		"/": "pipe::qwer",
	},
	action=Action(Action_Script(
		interpreter="", contents=[], network=False,
	)),
	outputs={},
))

print("--- HOW IS THIS STILL FINE so far!? ---")
pf = Protoformula(
	inputs={
		"/": "pipe::qwer",
	},
	action=Action(Action_Script(
		interpreter="", contents=[], network=False,
	)),
	outputs={},
)
print(Plot(
	inputs={
		#"one": "ware:tar:asdf", # barfs if uncommented
	},
	steps={
		"lol": Step(pf),
	},
	outputs={},
))

# okay so all those results... seem to say that...
# - the problem isn't in SandboxPort or PlotInput.  Those work fine in isolation.
# - the problem isn't in Protoformula.  It can accept inputs as strings and figure it all out.
# - using the Protoformula, after construction, in a Plot, is fine.  (it should be!; the variable assignment step in starlark should basically not matter.  still, checked, passed.)
# - but plot inputs barfs when using strings, above.
# - but plot inputs *doesn't* barf when using strings, below...?!?!
# I'm at a loss.
# Is there an outright stateful memory pointer bug somewhere here?


pt1 = Plot(
	inputs={
		"one": "ware:tar:asdf",
		"two": "literal:foobar",
	},
	steps={
		"beep": Step(Protoformula( # FIXME datalark should be able to do a mild magic to also have this work without 'Step(' wrapper, shouldn't it?  doesn't work yet.  simmilar below for "Action(Action_Script(".
			inputs={
				#SandboxPort("/"): "pipe::one", # FIXME something about uncommenting these causes a panic, and I can't figure it out at all
				#SandboxPort("$FOO"): PlotInput("pipe::two"), # THEORY: it's getting string'd without the prefix, then the map receives that and tries to parse it, and then dies.
			},
			action=Action(Action_Script( # FIXME handing in a type that's a union member here should work, without the 'Action(' wrapper.  currently doesn't?  appears to be trying to copy the content if I drop the 'Action(' constructor, which ends in the NodeAssembler for an Action getting the key "interpreter" thrown at it, which of course (correctly) doesn't fly.
				interpreter="/bin/bash", # future: an early goal for our templating helper funcs will probably be to have a wrapper func where this kwarg has a default.  (but this is a warpforge library thing; not deeper.)
				contents=["hey", "hello"],
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
	// 	steps: map<Map__StepName__Step>{}
	// 	outputs: map<Map__LocalLabel__PlotOutput>{}
	// }
}
