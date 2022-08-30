CLI Examples
============

This document contains examples of using the `warpforge` CLI to do a variety of operations.

---

These fixtures are executed by tests in the `cmd/warpforge` package.

These fixtures are executable by parsing them using
the [testmark](https://github.com/warpfork/go-testmark) format.
Some of the code blocks below are config files and content,
and each scenario also has one code block which is a script,
which is exactly what you would execute in a normal shell.

---

FUTURE: this might be a little more readable (and less redundant on fixtures)
if reorganized to show topic areas like "here's the things you can do on modules", etc.

---

## Recursively Run Modules

Using `./...` will traverse recursively from `.` and execute all `module.wf` files found.

[testmark]:# (runall/sequence)
```
warpforge run ./...
```

This works even if there are no modules found; the output will simply be empty:

[testmark]:# (runall/output)
```
```

## Check a Formula is Valid

Specific formats can also be checked explicitly, using `check` commands specific to that subsystem:

[testmark]:# (checkformula/sequence)
```
warpforge --verbose check formula.json
```

We'll run this in a filesystem that contains a `formula.json`:

[testmark]:# (checkformula/fs/formula.json)
```
{
    "formula": {
		"formula.v1": {
			"inputs": {
				"/": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			},
			"action": {
				"exec": {
					"command": [
						"/bin/sh",
						"-c",
						"echo hello from warpforge!"
					]
				}
			},
			"outputs": {}
		}
    },
    "context": {
		"context.v1": {
			"warehouses": {
			}
		}
    }
}
```

Together with the verbosity and output formatting flags used above, this will also emit the checked document again:

[testmark]:# (checkformula/output)
```
{
	"formula": {
		"formula.v1": {
			"inputs": {
				"/": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			},
			"action": {
				"exec": {
					"command": [
						"/bin/sh",
						"-c",
						"echo hello from warpforge!"
					]
				}
			},
			"outputs": {}
		}
	},
	"context": {
		"context.v1": {
			"warehouses": {}
		}
	}
}
```

## Check a Module is Valid

There's also an explicit `check` subcommand for dealing with modules:

[testmark]:# (checkmodule/sequence)
```
warpforge check module.wf
```

We'll run this in a filesystem that contains a `module.wf` (albeit a pretty silly one):

[testmark]:# (checkmodule/fs/module.wf)
```
{
	"module.v1": {
		"name": "test"
	}
}
```

Because the module is valid, there is no output to this command.

[testmark]:# (checkmodule/output)
```
```

## Check a Plot is Valid

There's also an explicit `check` subcommand for dealing with plots:

[testmark]:# (checkplot/sequence)
```
warpforge check plot.wf
```

We'll run this in a filesystem that contains a `plot.wf` (albeit a pretty silly one):

[testmark]:# (checkplot/fs/plot.wf)
```
{
    "inputs": {},
    "steps": {},
    "outputs": {}
}
```

Because it was successfully checked, the output is nothing:

[testmark]:# (checkplot/output)
```
```

## Execute a Formula

Excuting a formula is done with the `warpforge run` command.
When given a formula file, it knows what to do:

[testmark]:# (runformula/sequence)
```
warpforge --json --quiet run formula.json
```

We'll run this in a filesystem that contains a `formula.json`
(the same one we used in the check example earlier):

[testmark]:# (runformula/fs/formula.json)
```
{
    "formula": {
		"formula.v1": {
			"inputs": {
				"/": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			},
			"action": {
				"exec": {
					"command": [
						"/bin/sh",
						"-c",
						"echo hello from warpforge!"
					]
				}
			},
			"outputs": {}
		}
    },
    "context": {
		"context.v1": {
			"warehouses": {
			}
		}
    }
}
```

(TODO: this should probably use the testexec "then" feature to do several things on the same documents.)

The result of this will be a `RunRecord` object printed to stdout:

[testmark]:# (runformula/stdout)
```
{ "runrecord": { "guid": "389c442f-5343-497e-b74d-d31fd487af53", "time": "22222222222", "formulaID": "zM5K3Zz8R3ioVVWZ6o6GocxPKvubAJfv4iQmDH3GCq9UjtDjHtRWrry4DRoEBPvfUEYFx1D", "exitcode": 0, "results": {} } } 
```

(Note that we've normalized some of the values in this object for testing purposes.
When you run this command yourself, the time recorded in the runrecord will of course vary, as will the runrecord's guid!)

## Execute a Module

Excuting a module or a plot looks almost exactly the same as executing a formula --
it's still just the `warpforge run` command, which will figure out what to do with whatever you give it:

[testmark]:# (runmodule/sequence)
```
warpforge --json --quiet run module.wf
```

A module is declared with two files.  One is the `module.wf` file:

[testmark]:# (runmodule/fs/module.wf)
```
{
	"module.v1": {
		"name": "test"
	}
}
```

The module declaration is fairly short.
Mostly it just marks that a module is "here" on the filesystem,
and gives it a name.
(There can be other config here too, but it's all optional.)
Most of the data is in the plot, which is another file.

Here's the `plot.wf` file -- this one's a bit bigger and more involved:

[testmark]:# (runmodule/fs/plot.wf)
```
{
	"plot.v1": {
		"inputs": {
			"rootfs": "catalog:warpsys.org/busybox:v1.35.0:amd64-static"
		},
		"steps": {
			"zero-outer": {
				"plot": {
					"inputs": {
						"rootfs": "catalog:warpsys.org/busybox:v1.35.0:amd64-static"
					},
					"steps": {
						"zero-inner": {
							"protoformula": {
								"inputs": {
									"/": "pipe::rootfs"
								},
								"action": {
									"exec": {
										"command": [
											"/bin/sh",
											"-c",
											"mkdir /test; echo 'hello from step zero-inner' > /test/file"
										]
									}
								},
								"outputs": {
									"test": {
										"packtype": "tar",
										"from": "/test"
									}
								}
							}
						},
						"one-inner": {
							"protoformula": {
								"inputs": {
									"/": "pipe::rootfs",
									"/test": "pipe:zero-inner:test"
								},
								"action": {
									"exec": {
										"command": [
											"/bin/sh",
											"-c",
											"cat /test/file && echo 'hello from step one-inner' >> /test/file"
										]
									}
								},
								"outputs": {
									"test": {
										"packtype": "tar",
										"from": "/test"
									}
								}
							}
						},
					},
					"outputs": {
						"test": "pipe:one-inner:test"
					}
				}
			},
			"one-outer": {
				"protoformula": {
					"inputs": {
						"/": "pipe::rootfs",
						"/test": "pipe:zero-outer:test"
					},
					"action": {
						"exec": {
							"command": [
								"/bin/sh",
								"-c",
								"echo 'in one-outer'; cat /test/file"
							]
						}
					},
					"outputs": {}
				}
			}
		},
		"outputs": {
			"test": "pipe:zero-outer:test"
		}
	}
}
```

(That's not the smallest plot you could have -- it's actually quite complex,
and demonstrates multiple steps, including subplots, and how to wire them all up!)

This will also require a catalog entry for the referenced input (`catalog:warpsys.org/busybox:v1.35.0:amd64-static`).
This consists of a `module.json` file at the path of the module name, a `releases/[release name].json` file, and a
a `mirrors.json` file to show where the ware can be fetched.

[testmark]:# (runmodule/fs/.warpforge/catalog/warpsys.org/busybox/_module.json)
```json
{
        "catalogmodule.v1": {
                "name": "warpsys.org/busybox",
                "releases": {
                        "v1.35.0": "zM5K3XDQrBjGiewvGYS5f4pNHfGNM7P6dWJKYeVFq9vtYqzp5GicARaqsS27BCrTEyDT7qb"
                },
                "metadata": {}
        }
}
```

[testmark]:# (runmodule/fs/.warpforge/catalog/warpsys.org/busybox/_releases/v3.15.0.json)
```json
{
        "releaseName": "v1.35.0",
        "items": {
                "amd64-static": "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
        },
        "metadata": {
        }
}
```

[testmark]:# (runmodule/fs/.warpforge/catalog/warpsys.org/busybox/_mirrors.json)
```json
{
	"catalogmirrors.v1": {
		"byWare": {
			"tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9": [
				"https://example.com/this/is/not/a/real/mirror.tgz"
			]
		}
	}
}
```

The output for evaluating a module is a bit terser: it only emits the results object,
which has keys matching the outputs that the plot labels for extraction.
Because we only had one output named for export at the end of the module,
there's only one record in this map.
(Future: this will probably change :) and we might expect to see more progress details here as well.)

[testmark]:# (runmodule/stdout)
```
{ "runrecord": { "guid": "fb16d767-266a-4fc2-a4a2-b59105c1b3e7", "time": 1648067390, "formulaID": "zM5K3RvfmKy9zLfHk1T6kPafmvzAGt9Ls1QYFS4BvWTaCBgxYoJLDkkqP7SD7QWuoRTYw3j", "exitcode": 0, "results": { "test": "ware:tar:2En3zD1ho1qNeLpPryZVM1UTGnqPvnt48WY36TzCGJwSCudxPXkDtN3UuS4J3AYWAM" } } } 
{ "runrecord": { "guid": "16531b2e-6087-4ecb-b48d-a377d4dace90", "time": 1648067390, "formulaID": "zM5K3Rqj146W38bBjgU8yeJ4i37YtydoZGvpsqaHbNE2akLWfDYp8vi2KAh7vvU3XdUoy12", "exitcode": 0, "results": { "test": "ware:tar:4tvpCNb1XJ3gkH25MREMPBHRWa7gLUiYt7pF6AHNbqgwBrs3btvvmijebyZrYsi6Y9" } } } 
{ "plotresults": { "test": "tar:4tvpCNb1XJ3gkH25MREMPBHRWa7gLUiYt7pF6AHNbqgwBrs3btvvmijebyZrYsi6Y9" } } 
{ "runrecord": { "guid": "10941145-2d3e-44f9-ac0c-3dd2f6b6773c", "time": 1648067391, "formulaID": "zM5K3T8946y1A7Z4ZEuoCizPdDuneUQMqXqyfxXSh93CtK3n6gzgJgz9PMTUzJiexPErUqM", "exitcode": 0, "results": {} } } 
{ "plotresults": { "test": "tar:4tvpCNb1XJ3gkH25MREMPBHRWa7gLUiYt7pF6AHNbqgwBrs3btvvmijebyZrYsi6Y9" } } 
```

## Catalog Operations

[testmark]:# (catalog/fs/.warpforge/root)
```
this file marks the workspace as a root workspace
```

### Initialize a Catalog

[testmark]:# (catalog/sequence)
```
warpforge catalog init my-catalog
```

### List Catalogs

[testmark]:# (catalog/then-ls/sequence)
```
warpforge catalog ls
```

### Add an Item to a Catalog

#### tar

[testmark]:# (catalog/then-add-tar/sequence)
```
warpforge catalog --name my-catalog add tar warpsys.org/busybox:v1.35.0:amd64-static file://.warpforge/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9
```

[testmark]:# (catalog/then-add-tar/then-check/script)
```
cat .warpforge/catalogs/my-catalog/warpsys.org/busybox/_module.json
cat .warpforge/catalogs/my-catalog/warpsys.org/busybox/_releases/v1.35.0.json
cat .warpforge/catalogs/my-catalog/warpsys.org/busybox/_mirrors.json
```

[testmark]:# (catalog/then-add-tar/then-check/output)
```
{
	"catalogmodule.v1": {
		"name": "warpsys.org/busybox",
		"releases": {
			"v1.35.0": "zM5K3Z62CY9X6QkccuptyiC3a1tC32Fh2n1ujF8KH5Fz1BvKqppWJZgQJxEgypvF3pqzhdE"
		},
		"metadata": {}
	}
}
{
	"releaseName": "v1.35.0",
	"items": {
		"amd64-static": "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
	},
	"metadata": {}
}
{
	"catalogmirrors.v1": {
		"byWare": {
			"tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9": [
				"file://.warpforge/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			]
		}
	}
}
```

### Bundle Catalog

Test module that uses a catalog input:

[testmark]:# (catalog/then-add/then-bundle/fs/module.wf)
```
{
	"name": "bundle-test",
}
```

[testmark]:# (catalog/then-add/then-bundle/fs/plot.wf)
```
{
	"inputs": {
		"rootfs": "catalog:warpsys.org/busybox:v1.35.0:amd64-static"
	},
	"steps": {},
	"outputs": {}
}
```


[testmark]:# (catalog/then-add/then-bundle/sequence)
```
warpforge -v catalog bundle module.wf
```

[testmark]:# (catalog/then-add/then-bundle/stdout)
```
bundled "warpsys.org/busybox:v1.35.0:amd64-static"
```

# Ferk

The `ferk` command rapidly spawns a container in interactive mode. If the directory 
`/out` is created, its contents will be packed into a ware on exit.

Run `ferk` using Busybox as the rootfs and invoke `/bin/echo`.

[testmark]:# (catalog/then-add/then-ferk/sequence)
```
warpforge --json --quiet ferk --rootfs catalog:warpsys.org/busybox:v1.35.0:amd64-static --cmd /bin/echo --no-interactive
```

Check that `ferk` ran successfully, no outputs are expected.

[testmark]:# (catalog/then-add/then-ferk/stdout)
```
{ "runrecord": { "guid": "055a7ca6-4ea8-49d1-8053-e01e05202495", "time": 1648067779, "formulaID": "bafyrgqa3vklfqcqd6pjj6roc6vzny4p2rx4cqnptgo3rgze3qvemajrlpraiutycb2bebfk2lobgcmvaqpdnoip6zsfwooaulqqoraweyln6k", "exitcode": 0, "results": { "out": "ware:tar:-" } } } 
{ "plotresults": { "out": "tar:-" } } 
```

# Quickstart

The `quickstart` command creates a minimal Plot and Module. This requires content from
the default catalog, which was installed and updated in the previous section.

[testmark]:# (catalog/then-update/then-quickstart/sequence)
```
warpforge --quiet quickstart warpforge.org/my-quickstart-module
```

[testmark]:# (catalog/then-update/then-quickstart/stdout)
```
```

This "hello world" example can the be run normally.

[testmark]:# (catalog/then-update/then-quickstart/then-run/sequence)
```
warpforge --json run
```

[testmark]:# (catalog/then-update/then-quickstart/then-run/stdout)
```
{ "log": { "Msg": "inputs:" } } 
{ "log": { "Msg": "type = catalog ref = catalog:warpsys.org/busybox:v1.35.0:amd64-static" } } 
{ "log": { "Msg": "wareId = tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9 wareAddr = none" } } 
{ "log": { "Msg": "(hello-world) evaluating protoformula" } } 
{ "log": { "Msg": "ware mount: wareId = tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9 destPath = /" } } 
{ "log": { "Msg": "executing script interpreter = /bin/sh" } } 
{ "log": { "Msg": "packed \"out\": path = /output wareId=tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA" } } 
{ "runrecord": { "guid": "755c9be7-ca53-4d92-a600-8bcb25fee985", "time": 1651158797, "formulaID": "zM5K3ZMzLiBwQB93yZ4nFUsVSSgVtNPjpY72hKHxDjc9FRk9KnJSoCvkHFEPWfxARdjaguZ", "exitcode": 0, "results": { "out": "ware:tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA" } } } 
{ "log": { "Msg": "(hello-world) collected output hello-world:out" } } 
{ "log": { "Msg": "(hello-world) complete" } } 
{ "plotresults": { "output": "tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA" } } 
```