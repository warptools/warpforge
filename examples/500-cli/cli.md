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

Some of these examples require network connectivity. To run without network,
use the `-offline` flag when running tests.

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
				"tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9": "https://warpsys.s3.amazonaws.com/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
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
			"warehouses": {
				"tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9": "https://warpsys.s3.amazonaws.com/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			}
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
	"plot.v1":{
    "inputs": {},
    "steps": {},
    "outputs": {}
	}
}
```

Because it was successfully checked, the output is nothing:

[testmark]:# (checkplot/output)
```
```

## Execute a Formula

Excuting a formula is done with the `warpforge run` command.
When given a formula file, it knows what to do:

[testmark]:# (runformula/tags=net/sequence)
```
warpforge --json --quiet run formula.json
```

We'll run this in a filesystem that contains a `formula.json`
(the same one we used in the check example earlier).

[testmark]:# (runformula/tags=net/fs/formula.json)
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
				"tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9": "https://warpsys.s3.amazonaws.com/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			}
		}
    }
}
```

(TODO: this should probably use the testexec "then" feature to do several things on the same documents.)

The result of this will be a `RunRecord` object printed to stdout:

[testmark]:# (runformula/tags=net/stdout)
```
{ "runrecord": { "guid": "857b9774-e251-4bbc-9268-1883919fab23", "time": 1674646900, "formulaID": "zM5K3Zz8R3ioVVWZ6o6GocxPKvubAJfv4iQmDH3GCq9UjtDjHtRWrry4DRoEBPvfUEYFx1D", "exitcode": 0, "results": {} } } 
```

(Note that we've normalized some of the values in this object for testing purposes.
When you run this command yourself, the time recorded in the runrecord will of course vary, as will the runrecord's guid!)

## Execute a Module

Excuting a module or a plot looks almost exactly the same as executing a formula --
it's still just the `warpforge run` command, which will figure out what to do with whatever you give it:

[testmark]:# (base-workspace/then-runmodule/sequence)
```
warpforge --json --quiet run module.wf
```

A module is declared with two files.  One is the `module.wf` file:

[testmark]:# (base-workspace/then-runmodule/fs/module.wf)
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

[testmark]:# (base-workspace/then-runmodule/fs/plot.wf)
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
						}
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

These items are setup at the end of this file.

The output for evaluating a module is a bit terser: it only emits the results object,
which has keys matching the outputs that the plot labels for extraction.
Because we only had one output named for export at the end of the module,
there's only one record in this map.
(Future: this will probably change :) and we might expect to see more progress details here as well.)

[testmark]:# (base-workspace/then-runmodule/stdout)
```
{ "runrecord": { "guid": "c7fedecf-79b7-434b-9521-bf79a210cb2d", "time": 1692910078, "formulaID": "zM5K3RvfmKy9zLfHk1T6kPafmvzAGt9Ls1QYFS4BvWTaCBgxYoJLDkkqP7SD7QWuoRTYw3j", "exitcode": 0, "results": { "test": "ware:tar:2En3zD1ho1qNeLpPryZVM1UTGnqPvnt48WY36TzCGJwSCudxPXkDtN3UuS4J3AYWAM" } } } 
{ "runrecord": { "guid": "25f4a58b-2d45-4c01-b3ca-3d15165535a4", "time": 1692910079, "formulaID": "zM5K3Rqj146W38bBjgU8yeJ4i37YtydoZGvpsqaHbNE2akLWfDYp8vi2KAh7vvU3XdUoy12", "exitcode": 0, "results": { "test": "ware:tar:4tvpCNb1XJ3gkH25MREMPBHRWa7gLUiYt7pF6AHNbqgwBrs3btvvmijebyZrYsi6Y9" } } } 
{ "plotresults": { "test": "tar:4tvpCNb1XJ3gkH25MREMPBHRWa7gLUiYt7pF6AHNbqgwBrs3btvvmijebyZrYsi6Y9" } } 
{ "runrecord": { "guid": "df7923fd-1ff6-4600-8984-f896d79610fd", "time": 1692910079, "formulaID": "zM5K3T8946y1A7Z4ZEuoCizPdDuneUQMqXqyfxXSh93CtK3n6gzgJgz9PMTUzJiexPErUqM", "exitcode": 0, "results": {} } } 
{ "plotresults": { "test": "tar:4tvpCNb1XJ3gkH25MREMPBHRWa7gLUiYt7pF6AHNbqgwBrs3btvvmijebyZrYsi6Y9" } } 
```

## Relative Paths

Plots can have relative paths for mounts. These paths are relative to the
plot file itself. This example executes a module from a different directory
to demonstrate how relative paths work.

[testmark]:# (base-workspace/then-test-mounts/sequence)
```
warpforge --json --quiet run module
```

[testmark]:# (base-workspace/then-test-mounts/fs/module/module.wf)
```
{
	"module.v1": {
		"name": "test"
	}
}
```

[testmark]:# (base-workspace/then-test-mounts/fs/module/test.txt)
```
hello from a mounted file!
```

[testmark]:# (base-workspace/then-test-mounts/fs/module/plot.wf)
```
{
	"plot.v1": {
		"inputs": {
			"rootfs": "catalog:warpsys.org/busybox:v1.35.0:amd64-static"
		},
		"steps": {
			"one": {
				"protoformula": {
					"inputs": {
						"/": "pipe::rootfs"
						"/mnt": "mount:overlay:."
					},
					"action": {
						"exec": {
							"command": [
								"/bin/sh",
								"-c",
								"cat /mnt/test.txt"
							]
						}
					},
					"outputs": {}
				}
			}
		},
		"outputs": {}
	}
}
```

## Catalog Operations

[testmark]:# (catalog/tags=net/fs/.warpforge/root)
```
this file marks the workspace as a root workspace
```

### Initialize a Catalog

[testmark]:# (catalog/tags=net/sequence)
```
warpforge catalog init my-catalog
```

### List Catalogs

[testmark]:# (catalog/tags=net/then-ls/sequence)
```
warpforge catalog ls
```

### Generate Catalog HTML

[testmark]:# (base-workspace/then-generatehtml/sequence)
```
warpforge catalog --name=test generate-html
```

### Mirror a Catalog

The base workspace does not have mirror information to avoid using the network by default. We will override
the `_mirrors.json` file to test mirroring to a mock remote warehouse.

[testmark]:# (base-workspace/then-mirror/fs/.warpforge/catalogs/test/warpsys.org/busybox/_mirrors.json)
```json
{
	"catalogmirrors.v1": {
		"byModule": {
			"warpsys.org/busybox": {
				"tar": ["ca+mock://example.warp.tools"]
			}
		}
	}
}
```

Now we can test mirroring:

[testmark]:# (base-workspace/then-mirror/sequence)
```
warpforge catalog --name=test mirror
```

### Add an Item to a Catalog

#### tar
[testmark]:# (catalog/tags=net/then-add-tar/sequence)
```
warpforge catalog --name my-catalog add tar warpsys.org/busybox:v1.35.0:amd64-static https://warpsys.s3.amazonaws.com/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9
```

[testmark]:# (catalog/tags=net/then-add-tar/then-check/script)
```
cat .warpforge/catalogs/my-catalog/warpsys.org/busybox/_module.json
cat .warpforge/catalogs/my-catalog/warpsys.org/busybox/_releases/v1.35.0.json
cat .warpforge/catalogs/my-catalog/warpsys.org/busybox/_mirrors.json
```

[testmark]:# (catalog/tags=net/then-add-tar/then-check/output)
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
				"https://warpsys.s3.amazonaws.com/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			]
		}
	}
}
```

#### git

[testmark]:# (catalog-git/tags=net/fs/.warpforge/root)
```
this file marks the workspace as a root workspace
```

[testmark]:# (catalog-git/tags=net/sequence)
```
warpforge catalog add git github.com/githubtraining/training-manual:v1.0:src https://github.com/githubtraining/training-manual v1.0
```

[testmark]:# (catalog-git/tags=net/then-check/script)
```
cat .warpforge/catalogs/default/github.com/githubtraining/training-manual/_module.json
cat .warpforge/catalogs/default/github.com/githubtraining/training-manual/_releases/v1.0.json
cat .warpforge/catalogs/default/github.com/githubtraining/training-manual/_mirrors.json
```

[testmark]:# (catalog-git/tags=net/then-check/output)
```
{
	"catalogmodule.v1": {
		"name": "github.com/githubtraining/training-manual",
		"releases": {
			"v1.0": "zM5K3W15SFfQZ5uJVdcEDgeHCoGhxLYLHKsMXvmUad4MUZ9raT2ropMsE66FqaeHDsaVWc7"
		},
		"metadata": {}
	}
}
{
	"releaseName": "v1.0",
	"items": {
		"src": "git:d5af19cebecb2a162bffcf1994cb87f8c9041ae1"
	},
	"metadata": {}
}
{
	"catalogmirrors.v1": {
		"byModule": {
			"github.com/githubtraining/training-manual": {
				"git": [
					"https://github.com/githubtraining/training-manual"
				]
			}
		}
	}
}
```

### Bundle Catalog

Test module that uses a catalog input:

[testmark]:# (base-workspace/then-bundle/fs/workspace/.warpforge/notrelevant)
```
this file creates directories for a local workspace
```

[testmark]:# (base-workspace/then-bundle/fs/workspace/module.wf)
```
{
	"name": "bundle-test",
}
```

[testmark]:# (base-workspace/then-bundle/fs/workspace/plot.wf)
```
{
	"plot.v1": {
		"inputs": {
			"rootfs": "catalog:warpsys.org/busybox:v1.35.0:amd64-static"
		},
		"steps": {},
		"outputs": {}
	}
}
```


[testmark]:# (base-workspace/then-bundle/sequence)
```
cd workspace
warpforge -v catalog bundle module.wf
```

[testmark]:# (base-workspace/then-bundle/output)
```
  bundled "warpsys.org/busybox:v1.35.0:amd64-static"
  
```

# Ferk

The `ferk` command rapidly spawns a container in interactive mode. If the directory 
`/out` is created, its contents will be packed into a ware on exit.

Run `ferk` using Busybox as the rootfs and invoke `/bin/echo`.

[testmark]:# (base-workspace/then-ferk/sequence)
```
warpforge --json --quiet ferk --rootfs catalog:warpsys.org/busybox:v1.35.0:amd64-static --cmd /bin/echo --no-interactive
```

Check that `ferk` ran successfully, no outputs are expected.

[testmark]:# (base-workspace/then-ferk/stdout)
```
{ "runrecord": { "guid": "61940b67-024a-476e-996e-740ff80356c7", "time": 1692910079, "formulaID": "zM5K3V1fXVjExjfVd8d7ByUQ7HP16QAcZcoRd1bh3X4uvms1Xbpb87c1a7WNaw8Hw2B3uF6", "exitcode": 0, "results": { "out": "ware:tar:-" } } } 
{ "plotresults": { "out": "tar:-" } } 
```

[testmark]:# (base-workspace/then-ferk-with-plot/fs/plot.wf)
```
{
  "plot.v1": {
    "inputs": {
      "rootfs": "catalog:warpsys.org/busybox:v1.35.0:amd64-static"
    },
    "steps": {
      "ferk": {
        "protoformula": {
          "inputs": {
            "/": "pipe::rootfs"
          },
          "action": {
            "script": {
              "interpreter": "/bin/bash",
              "contents": [
                "echo 'APT::Sandbox::User \"root\";' > /etc/apt/apt.conf.d/01ferk",
                "echo 'Dir::Log::Terminal \"\";' >> /etc/apt/apt.conf.d/01ferk",
                "/bin/bash"
              ],
              "network": true
            }
          },
          "outputs": {}
        }
      }
    },
    "outputs": {}
  }
}
```

[testmark]:# (base-workspace/then-ferk-with-plot/sequence)
```
warpforge --json --quiet ferk --plot ./plot.wf --cmd /bin/echo --no-interactive
```

[testmark]:# (base-workspace/then-ferk-with-plot/stdout)
```
{ "runrecord": { "guid": "b964aa2f-bb7a-44c4-b4fa-bfc9d15d0ace", "time": 1692910079, "formulaID": "zM5K3YWRYqSgvxgMkAA9KbzPpqtRPbufF2z397SNJ1mKTkp9SpmxA8jD3YmTPu3EWvijMSv", "exitcode": 0, "results": {} } } 
{ "plotresults": {} } 
```

# Quickstart

The `quickstart` command creates a minimal Plot and Module. This requires content from
the default catalog, which was installed and updated in the previous section.

[testmark]:# (base-workspace/then-quickstart/sequence)
```
warpforge quickstart warpforge.org/my-quickstart-module
```

[testmark]:# (base-workspace/then-quickstart/stdout)
```
Successfully created module.wf and plot.wf for module "warpforge.org/my-quickstart-module".
Ensure your catalogs are up to date by running `warpforge catalog update`.
You can check status of this module with `warpforge status`.
You can run this module with `warpforge run`.
Once you've run the Hello World example, edit the 'script' section of plot.wf to customize what happens.
```

This "hello world" example can the be run normally.

[testmark]:# (base-workspace/then-quickstart/then-run/sequence)
```
warpforge --json run
```

[testmark]:# (base-workspace/then-quickstart/then-run/stdout)
```
{ "log": { "Msg": "inputs:" } } 
{ "log": { "Msg": "type = catalog ref = catalog:warpsys.org/busybox:v1.35.0:amd64-static" } } 
{ "log": { "Msg": "wareId = tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9 wareAddr = none" } } 
{ "log": { "Msg": "(hello-world) evaluating protoformula" } } 
{ "log": { "Msg": "ware mount: wareId = tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9 destPath = /" } } 
{ "log": { "Msg": "executing script interpreter = /bin/sh" } } 
{ "log": { "Msg": "packed \"out\": path = /output wareId=tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA" } } 
{ "runrecord": { "guid": "6785b641-6e41-4ecb-9207-4ecd60b85bd6", "time": 1692910079, "formulaID": "zM5K3ZMzLiBwQB93yZ4nFUsVSSgVtNPjpY72hKHxDjc9FRk9KnJSoCvkHFEPWfxARdjaguZ", "exitcode": 0, "results": { "out": "ware:tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA" } } } 
{ "log": { "Msg": "(hello-world) collected output hello-world:out" } } 
{ "log": { "Msg": "(hello-world) complete" } } 
{ "plotresults": { "output": "tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA" } } 
```

## Catalog

These tests require a workspace with a catalog entry,  which is setup here:
[testmark]:# (base-workspace/fs/.warpforge/catalogs/test/warpsys.org/busybox/_module.json)
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
```

[testmark]:# (base-workspace/fs/.warpforge/catalogs/test/warpsys.org/busybox/_mirrors.json)
```json
{
	"catalogmirrors.v1": {
		"byWare": {
		},
	}
}
```

[testmark]:# (base-workspace/fs/.warpforge/catalogs/test/warpsys.org/busybox/_releases/v1.35.0.json)
```
{
	"releaseName": "v1.35.0",
	"items": {
		"amd64-static": "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
	},
	"metadata": {}
}
```

[testmark]:# (base-workspace/fs/.warpforge/root)
```
this file marks the workspace as a root workspace
```

[testmark]:# (base-workspace/fs/.warpforge/config/mirroring.json)
```
{
	"mirroring.v1": {
		"ca+mock://mock.warp.tools": {
			"pushConfig": {
				"mock": {}
			}
		}
	}
}
```

[testmark]:# (base-workspace/script)
```
# no-op -- this is required to ensure the tests actually run.
```

[testmark]:# (base-workspace/stdout)
```
```

# Unpack

This shows the general usage of unpacking a ware.
Without the `--path` flag, unpack targets the current working directory.

[testmark]:# (unpack/sequence)
```
warpforge ware unpack tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9
```

[testmark]:# (unpack/output)
```

```

[testmark]:# (unpack/then-ls/script)
```
ls
```

[testmark]:# (unpack/then-ls/output)
```
bin
linuxrc
sbin
usr
```

[testmark]:# (unpack/then-unpack-again/sequence)
```
warpforge ware unpack tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9
```

[testmark]:# (unpack/then-unpack-again/output)
```
error: path is a non-empty directory: file already exists
```

[testmark]:# (unpack/then-unpack-again/exitcode)
```
1
```
Adding a `--path` flag will create a directory if required before unpacking inside it.

[testmark]:# (unpack/then-unpack-to-path/sequence)
```
warpforge ware unpack --path tmp tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9
```

[testmark]:# (unpack/then-unpack-to-path/output)
```
```

[testmark]:# (unpack/then-unpack-to-path/then-ls/script)
```
ls tmp
```

[testmark]:# (unpack/then-unpack-to-path/then-ls/output)
```
bin
linuxrc
sbin
usr
```

# Plan

The `plan` subcommand is used to generate Warpforge input files from higher
level languages.

## Generate

The `generate` subcommand performs generation on a directory. This example
uses warplark to create an empty plot.

This command supports operations on a single file, a single directory,
the working directory (default with no arguments), or a whole directory
tree (with `...`). This tests each usage.

Use on a single file:

[testmark]:# (plan-generate/sequence)
```
warpforge plan generate plot.star
```

(note, we setup the fs for all the generate tests here to avoid repetition)

[testmark]:# (plan-generate/fs/plot.star)
```
#+warplark version 0
result = {"inputs": {}, "steps": {}, "outputs": {}}
```

[testmark]:# (plan-generate/fs/a/plot.star)
```
#+warplark version 0
result = {"inputs": {}, "steps": {}, "outputs": {}}
```

[testmark]:# (plan-generate/fs/b/plot.star)
```
#+warplark version 0
result = {"inputs": {}, "steps": {}, "outputs": {}}
```

[testmark]:# (plan-generate/output)
```

```

[testmark]:# (plan-generate/then-cat/script)
```
cat plot.wf
```

[testmark]:# (plan-generate/then-cat/output)
```
{
	"plot.v1": {
		"inputs": {},
		"outputs": {},
		"steps": {}
	}
}
```

Use on specific directory:

[testmark]:# (plan-generate/then-dir/sequence)
```
warpforge plan generate a
```

[testmark]:# (plan-generate/then-dir/output)
```
```

[testmark]:# (plan-generate/then-dir/then-cat/script)
```
cat a/plot.wf
```

[testmark]:# (plan-generate/then-dir/then-cat/output)
```
{
	"plot.v1": {
		"inputs": {},
		"outputs": {},
		"steps": {}
	}
}
```

Use on working directory (no args):

[testmark]:# (plan-generate/then-pwd/sequence)
```
warpforge plan generate
```

[testmark]:# (plan-generate/then-pwd/output)
```
```

[testmark]:# (plan-generate/then-pwd/then-cat/script)
```
cat plot.wf
```

[testmark]:# (plan-generate/then-pwd/then-cat/output)
```
{
	"plot.v1": {
		"inputs": {},
		"outputs": {},
		"steps": {}
	}
}
```

Recursive usage (`...`):


[testmark]:# (plan-generate/then-recursive/sequence)
```
warpforge plan generate ./...
```

[testmark]:# (plan-generate/then-recursive/output)
```
```

[testmark]:# (plan-generate/then-recursive/then-cat/script)
```
cat a/plot.wf
cat b/plot.wf
```

[testmark]:# (plan-generate/then-recursive/then-cat/output)
```
{
	"plot.v1": {
		"inputs": {},
		"outputs": {},
		"steps": {}
	}
}
{
	"plot.v1": {
		"inputs": {},
		"outputs": {},
		"steps": {}
	}
}
```

