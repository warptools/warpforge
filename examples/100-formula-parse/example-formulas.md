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

FUTURE: we should add sections that contain the debug printout format for the same datum too,
which may help human readers of this file see what's going on more clearly.

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