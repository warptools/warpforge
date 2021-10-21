Example Formulas and Runrecords (dummy data)
============================================

This document contains a bunch of formulas -- or more specifically,
the `FormulaAndContext` type -- in JSON encoding.

Also, some `RunRecord` documents, since those are often seen together.

They're all full of dummy data.
(The formulas are not expected to be executable or refer to data that exists;
and the runrecord outputs certainly don't describe real output data that you can compute.)
They're just for asserting that the API parsers and serializers work.

These documents should all round-trip.

FUTURE: we should add fixtures of the hash outputs to each datum.

FUTURE: optionally add some fixtures that path into the parsed data and pluck out parts of it?
May be somewhat overkill (roundtripping gives a lot of confidence already), but may also be usefully demonstrative.

---

These fixtures are executed by tests over in the `wfapi` package
(which makes sense, because they're just parse and serialize tests).

These fixtures are executable by parsing them using
the [testmark](https://github.com/warpfork/go-testmark) format.

If editing this document:

- all the testmark data block annotations should start with a "directory"
- each directory should contain either "formula" or "runrecord",
  and that's how the test executor will decide what type to unmarshal into.
- other "file" names in that "directory" can be used in the future
  for things like the hash, etc, that are matched with the main document in that "directory".

---

The Empty Formula
-----------------

Here's about the simplest formula one could have:

[testmark]:# (zero-formula/formula)
```json
{
	"formula": {
		"inputs": {},
		"action": {
			"exec": {
				"command": []
			}
		},
		"outputs": {}
	}
}
```

This formula is obviously quite useless, having no outputs, nor inputs, nor a command.
It's the minimal document that will parse.
(You can't have fewer fields than this because per the Schema, none of these are optional fields;
and "action" is a union type, so it has to have _some_ member, which in this case was the "exec" member.)

Formulas can be hashed.  This is the CID of the above formula
(using the DAG-CBOR code, and sha2-512 as a hash):

[testmark]:# (zero-formula/cid)
```
bafyrgqelzywg34w5h55k5tqw6zn4beeeonglldq5q7oihmdlpalav4imqco3zkdf5llnpncdwmlgjdk7o6rwema2dg34qp4hhwocxjwk6y4a4
```

---

A Formula With Inputs
---------------------

Formulas usually have inputs.  And the most common type is a "ware", which will be mounted to some directory.

[testmark]:# (hello-input/formula)
```json
{
	"formula": {
		"inputs": {
			"/mount/path": "ware:tar:qwerasdf",
			"/other/place": "ware:git:abcd1234"
		},
		"action": {
			"exec": {
				"command": []
			}
		},
		"outputs": {}
	}
}
```

---

Inputs can also be targetted at variables, rather than filesystems.
And in this case, they also are more likely to use a "literal" form.

[testmark]:# (hello-input-vars/formula)
```json
{
	"formula": {
		"inputs": {
			"$USER": "literal:hello",
			"$HOME": "literal:/home/hello",
			"$PATH": "literal:/bin:/usr/bin:/local/bin"
		},
		"action": {
			"exec": {
				"command": []
			}
		},
		"outputs": {}
	}
}
```

(Note that while parsing inputs, one looks for ":" as a delimiter,
you don't want to _split_ on that... at some point the parser needs to
stop matching on this after it's figured out what input type the value is,
and then the rest of the string can contain more characters that shouldnt be parser specially.
You can see this in the `$PATH` variable's value above.)

---

How Formulas are Parsed
-----------------------

Let's show how these values are being parsed.

[testmark]:# (hello-and-debug/formula)
```json
{
	"formula": {
		"inputs": {
			"$HOME": "literal:/home/hello",
			"/mount/me": "ware:tar:qwerasdf",
			"/fancy/stuff": {
				"basis": "ware:tar:asdfzxcv",
				"filters": {
					"demo": "value"
				}
			}
		},
		"action": {
			"exec": {
				"command": []
			}
		},
		"outputs": {}
	}
}
```

Here's a debug representation that shows how those things are parsed using the Warpforge API schema,
and what types we see after parsing the information:

[testmark]:# (hello-and-debug/formula.debug)
```text
struct<FormulaAndContext>{
	formula: struct<Formula>{
		inputs: map<Map__SandboxPort__FormulaInput>{
			union<SandboxPort>{string<SandboxVar>{"HOME"}}: union<FormulaInput>{union<FormulaInputSimple>{string<String>{"/home/hello"}}}
			union<SandboxPort>{string<SandboxPath>{"mount/me"}}: union<FormulaInput>{union<FormulaInputSimple>{struct<WareID>{
				packtype: string<Packtype>{"tar"}
				hash: string<String>{"qwerasdf"}
			}}}
			union<SandboxPort>{string<SandboxPath>{"fancy/stuff"}}: union<FormulaInput>{struct<FormulaInputComplex>{
				basis: union<FormulaInputSimple>{struct<WareID>{
					packtype: string<Packtype>{"tar"}
					hash: string<String>{"asdfzxcv"}
				}}
				filters: map<FilterMap>{
					string<String>{"demo"}: string<String>{"value"}
				}
			}}
		}
		action: union<Action>{struct<Action_Exec>{
			command: list<List__String>{
			}
			network: absent
		}}
		outputs: map<Map__OutputName__GatherDirective>{
		}
	}
	context: absent
}
```

As you can see, unions (sometimes also called sum types, or variants, or other terms like that in some literature) are used heavily here.

- The type of the formula's `inputs` map keys is a union type called `SandboxPort`.
	- We can see it containing member types `SandboxVar` and `SandboxPath` in this example.
	- Notice how each of those is missing their leading character from the JSON.  That's because those were the indicators for which type to see that data as!
- The type `FormulaInputSimple` is seen, containing both `WareID` types and simple string literals.
	- These are indicated again by string prefix, although it's a little more obvious: "`literal:`" and "`ware:`".
	- ... Notice that the `WareID` type has been further parsed into a struct with two fields!  This used a "`:`" as a delimiter again, but is a slightly different kind of parse than the union was (this one will extract any two strings, rather than having a known fixed list of options).
- The type `FormulaInput` wraps where we see `FormulaInputSimple`.  And in the third input, we see another option: `FormulaInputComplex`.
	- These were indicated a totally different way: this is called a "kinded union".  The fact that a map is used in the JSON is what indicates a `FormulaInputComplex`, vs a string in the JSON indicating `FormulaInputSimple`!
- In the formula's `action` field, we see another union type, named `Action`.  The value we see in it is of type `Action_Exec`.
	- This morphs from JSON in yet another way: this one was indicated by the key in the JSON map.

Unions are used wherever there's an exclusive choice (such as: `SandboxPath` and `SandboxVar` for whether something is a mount path or a variable name),
or, whenever we need an extension mechanism (that's what `Action` and its various possible members are for: so we can add new ones in the future).

A Formula With Script Action
----------------------------

The `script` action type allows for a script to be executed. This takes a path to the interpreter to be used,
and a `contents` array. Each element of `contents` will be executed as an individual statement.

[testmark]:# (script-action/formula)
```json
{
	"formula": {
		"inputs": {
			"/mount/path": "ware:tar:qwerasdf",
			"/other/place": "ware:git:abcd1234"
		},
		"action": {
			"script": {
				"interpreter": "/bin/sh",
				"contents": [
					"echo hello!",
					"echo this is a script action!"
				]
			}
		},
		"outputs": {}
	}
}
```

---