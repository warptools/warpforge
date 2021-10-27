Example Plots (dummy data)
==========================

This document contains a bunch of "plot" documents, in JSON encoding.

They're all full of dummy data.
(The plots are not expected to be executable or refer to data that exists.)
They're just for asserting that the API parsers and serializers work.

These documents should all round-trip.

---

These fixtures are executed by tests over in the `wfapi` package
(whichs should make sense, because they're just parse and serialize tests).

These fixtures are executable by parsing them using
the [testmark](https://github.com/warpfork/go-testmark) format.

If editing this document:

- all the testmark data block annotations should start with a "directory", which is the name of the fixture they're part of
- the second hunk of the data block's name should say what the data is (e.g. "plot"),
  and that's how the test executor will decide what type to unmarshal into.
- the test code recognizes some of these names and will determine what to test based on them,
  so you'll need to look at the test code to see what names cause magic to happen.

---


A Simple Plot
-------------

Plots look initially similar to Formulas -- they have inputs and outputs --
but instead of an "action", they have "steps" (several of them!);
and what exactly the inputs and outputs are, and how they're labeled, are a bit different (and more expressive!).

Here is a relatively simple plot, which has a single step:

[testmark]:# (simple-plot/plot)
```json
{
	"inputs": {
		"thingy": "ware:tar:qwerasdf",
		"ingest": "ingest:git:.:HEAD"
	},
	"steps": {
		"one": {
			"protoformula": {
				"inputs": {
					"/": "pipe::thingy"
				},
				"action": {
					"exec": {
						"command": [
							"/bin/echo",
							"hi"
						],
						"network": false
					}
				},
				"outputs": {
					"stuff": {
						"from": "/",
						"packtype": "tar"
					}
				}
			}
		}
	},
	"outputs": {
		"test": "pipe:one:stuff"
	}
}
```

There's actually *several* new concepts here.

- The "inputs" field for a plot is just a string.  It doesn't start with "/" or "$".  It's a local name, not a mount path or a variable for within a container.
- The "steps" field is, of course, a map.  The keys?  Those are "step names".
- Each step can be either a "protoformula" or a plot (...!  Yes, they can be nested, recursively).
- A protoformula... well, it looks a lot like a formula, except the inputs are more diverse.
- In this protoformula, we see a `Pipe` input.  This is indicated by the "pipe:" prefix.
- The protoformula's piped input refers to the input named in the plot input.
- The plot's output uses another pipe... this time to refer to the output named by the protoformula in the step inside the plot.

The debug dump of this data may be illustrative:

[testmark]:# (simple-plot/plot.debug)
```text
struct<Plot>{
	inputs: map<Map__LocalLabel__PlotInput>{
		string<LocalLabel>{"thingy"}: union<PlotInput>{union<PlotInputSimple>{struct<WareID>{
			packtype: string<Packtype>{"tar"}
			hash: string<String>{"qwerasdf"}
		}}}
		string<LocalLabel>{"ingest"}: union<PlotInput>{union<PlotInputSimple>{union<Ingest>{struct<GitIngest>{
			hostPath: string<String>{"."}
			ref: string<String>{"HEAD"}
		}}}}
	}
	steps: map<Map__StepName__Step>{
		string<StepName>{"one"}: union<Step>{struct<Protoformula>{
			inputs: map<Map__SandboxPort__PlotInput>{
				union<SandboxPort>{string<SandboxPath>{""}}: union<PlotInput>{union<PlotInputSimple>{struct<Pipe>{
					stepName: string<StepName>{""}
					label: string<LocalLabel>{"thingy"}
				}}}
			}
			action: union<Action>{struct<Action_Exec>{
				command: list<List__String>{
					0: string<String>{"/bin/echo"}
					1: string<String>{"hi"}
				}
				network: bool<Boolean>{false}
			}}
			outputs: map<Map__LocalLabel__GatherDirective>{
				string<LocalLabel>{"stuff"}: struct<GatherDirective>{
					from: union<SandboxPort>{string<SandboxPath>{""}}
					packtype: string<Packtype>{"tar"}
					filters: absent
				}
			}
		}}
	}
	outputs: map<Map__LocalLabel__PlotOutput>{
		string<LocalLabel>{"test"}: union<PlotOutput>{struct<Pipe>{
			stepName: string<StepName>{"one"}
			label: string<LocalLabel>{"stuff"}
		}}
	}
}
```

This probably seems like a lot of things, compared to a formula.
It is.
These features will start looking more and more useful when you see
how we can use them to make multiple steps inside a plot,
and how we can even use them to attach plots to each other (indirectly, through catalogs).

---

TODO: a plot with a catalog ref input like `"thingy": "catalog:foo.org/bar:v1000:linux-amd64"`

- We've got a new kind of input -- it's a `CatalogRef` type.  This is indicated by the "catalog:" prefix.
	- `CatalogRef` is a struct -- see the debug dump below for how it's broken down.

---

TODO: a plot with more steps and more interesting pipes.

---

TODO: a plot where the outputs refer to one of the plot inputs.

---

TODO: recursive plots.
