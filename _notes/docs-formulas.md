Communicating with Warpforge: Formulas, Protoformulas, and Replays
==================================================================

A formula looks like this, and has the same rules as always (inputs are hashes *only*):

```json
{
	"formula": {
		"input": {
			"/": "ware:tar:1f4473ffef873a",
			"/i2": "ware:git:ab4543ffef1f4"
		},
		"action": {
			"exec": {"script": [
				"series --of",
				"commands && stuff"
			]},
		},
		"output": {
			"nameit": "pack:tar:/whatever"
		}
	}
}
```

(The values in the input map changed a bit.  Notice how they all prefix with "ware:" now.
For formulas, that's effectively a constant.  We'll see why it's there in a moment.)

---

A protoformula (aka a module) can use more powerful inputs, like catalogs and ingests,
and looks like this:

```json
{
	"protoformula": {
		"input": {
			"/": "ware:tar:1f4473ffef873a",
			"/i2": "ingest:git:.:HEAD",
			"/i3": "catalog:example.org/frob/noz:v0.14:linux-amd64",
			"/i4": "catalog:example.org/zog/kog:candidate:linux-amd64"
		},
		"action": {
			"exec": {"script": [
				"series --of",
				"commands && stuff"
			]},
		},
		"output": {
			"nameit": "pack:tar:/whatever"
		}
	}
}
```

*Notice how it's structurally almost identical to a formula* --
only the initial type indicator field is different.

(Also note how you can still use a literal `"ware:{warekind}:{hash}"` input,
and the syntax for that is the same.)

This is on purpose.
In fact, if you take any formula, and change the type indicator key at the top of the document to "protoformula",
it's guaranteed valid.  Formulas are a subset of protoformulas!
Protoformulas just get to use even more features than formulas.

This is nice for humans who want to manipulate these documents,
and it makes it easy to read things and transform them (even by hand),
because the rough level of indentation and general structure of the document stays pretty consistent.
(But it's also very convenient for our tools and parsers!
It means if a document fails to parse, we can re-try parsing the entire inner body of it
with the body from one of the more powerful forms, and if that passes,
then we can give a good error message to the human: e.g.,
"hey, did you know you used an ingest feature (only allowed in protoformulas) in a formula?".)

---

A third format, the replay, keeps up the structural similarities
but adds slightly more rules back again:


```json
{
	"replay": {
		"input": {
			"/": "ware:tar:1f4473ffef873a",
			"/i2": "ware:git:ab4543ffef1f4",
			"/i3": "catalog:example.org/frob/noz:v0.14:linux-amd64",
			"/i4": "catalog:example.org/zog/kog:v100:linux-amd64"
		},
		"action": {
			"exec": {"script": [
				"series --of",
				"commands && stuff"
			]},
		},
		"output": {
			"nameit": "pack:tar:/whatever"
		}
	}
}
```

In a replay, we can't use ingests again,
and while we can still use catalogs,
we aren't allowed to use "candidate" release versions
(it has to be a real release, one that's properly tracked in the catalog).

The replay format is the one that's stored for audit logs,
and for use by "explain" commands that trace out what went into a computation (recursively!).
Protoformulas can't be used for this directly, because they gather some information from their environment
when being evaluated... but replays freeze all that data into the replay document, so they serve this purpose nicely.
Replays can be automatically reduced from protoformulas during their evaluation.

---

There's one more thing to cover.  It's not quite a fourth format --
technically we still consider it a protoformula -- but it's an expanded form of it,
which includes the ability to describe graphs of actions, with data piped between them.


```json
{
	"protoformula": {
		"input": {
			"i1": "ware:tar:1f4473ffef873a",
			"i2": "ingest:git:.:HEAD",
			"i4": "catalog:example.org/zog/kog:candidate:linux-amd64"
		},
		"action": {
			"pipeline": {
				"step1": {
					"input": {
						"/": "pipe::i1",
						"/i2": "pipe::i2",
						"/i3": "catalog:example.org/frob/noz:v0.14:linux-amd64",
					},
					"action": {
						"exec": {"script": [
							"series --of",
							"commands && stuff"
						]},
					},
					"output": {
						"intermed": "pack:tar:/whatever"
					}
				},
				"step2": {
					"input": {
						"/": "pipe::i1",
						"/ix": "pipe:step1:intermed",
						"/i4": "pipe::i4",
					},
					"action": {
						"exec": {"script": [
							"different commands; different container"
						]},
					},
					"output": {
						"whatsit": "pack:tar:/whatever"
					}
				}
			},
		},
		"output": {
			"nameit": "pipe:step2:whatsit"
		}
	}
}
```

You can probably pretty much guess what all this does.

Pipeline actions can be used recursively.
This looks much the same as the top level one does.
(There's also nothing you can do with recursive pipeline actions that you couldn't do with a flattened one,
but it makes some namespacing available, which can be helpful for making composable snippets as well as for documentational purposes, and is generally handy.)



Differences from Ancient Times
------------------------------

If you worked with previous generations of the Timeless Stack, these specs probably look familiar.
Most of the concepts (formulas, protoformulas, replays) have been around for a while, and aren't conceptually much changed.

The main improvements made with this generation of specs is that we've made things more consistent,
so that it's easier to turn one level of document into another.
Sometimes this has come at a cost of some textual redundancy
(e.g., the "ware:" prefix in formula inputs is 100% redundant in that context; it's the only valid option).
We feel that the textual redundancy is a small price to pay for the increase in consistency and readability.

Protoformulas similarly gained one more level of indentation, because "pipeline" is now a kind of "action".
This increases consistency, makes different kinds of documents more visually similar, and makes describing errors easier.

Some things changed name slightly.  For example, protoformulas have "inputs" now, instead of "imports".
While there *is* a semantic difference between protoformulas and formulas here, it seems easier to describe it as such,
rather than introduce more new words.

A couple of other small but meaningful changes have also been introduced,
such as the "script" feature of the action.exec.
