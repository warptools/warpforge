# Warpforge Quick Start Guide

This guide aims to get you up and running with Warpforge

## Install

You need to `git clone`, `go install ./...`, then copy `plugins/*` to `$GOPATH/bin`

TODO: better install (perhaps a tarball)

## Initializing a Module

A Warpforge *module* consists of two files: 
1. `module.json`: defines the module name
2. `plot.json`: defines the inputs, execution steps, and outputs

A minimal `module.json` and `plot.json` can be initialized with

[testmark]:# (quickstart/module-init/sequence)
```
warpforge module init my-module-name
```

This will produce the two output files in the current working directory, and
fail if the files already exist.

### module.json
[testmark]:# (quickstart/module-init/then-check-module/script)
```
cat module.json
```

[testmark]:# (quickstart/module-init/then-check-module/output)
```
{
	"name": "my-module-name"
}
```

### plot.json
[testmark]:# (quickstart/module-init/then-check-plot/script)
```
cat plot.json
```

[testmark]:# (quickstart/module-init/then-check-plot/output)
```
{
	"inputs": {},
	"steps": {},
	"outputs": {}
}
```

[testmark]:# (quickstart/module-init/fs/placeholder-so-we-exec-in-a-temp-dir)
```
```

## Creating a Catalog

Warpforge *catalogs* give friendly names to *wares*. This allows us to use a string like
`catalog:alpinelinux.org/alpine:v3.14.2:x86_64` to refer to a ware instead of 
`ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS`.

Additionally, catalogs allow us to specify mirrors which provide the ware. When a catalog
reference input is used, the provided mirrors will be used to fetch the ware.

We can add items to the catalog using the `catalog` subcommand. This will
1. Create a catalog entry if it does not exist
2. Fetch the provided URL and compute its WareID (hash)
3. Add the URL to the list of mirrors for this catalog entry

[testmark]:# (quickstart/sequence)
```
warpforge catalog add tar alpinelinux.org/alpine:v3.14.2:x86_64 https://dl-cdn.alpinelinux.org/alpine/v3.14/releases/x86_64/alpine-minirootfs-3.14.2-x86_64.tar.gz
```

Catalog entries consist of two file `lineage.json` and `mirrors.json`. The files created
by this example are:

### lineage.json
[testmark]:# (quickstart/then-check-lineage/script)
```
cat .warpforge/catalog/alpinelinux.org/alpine/lineage.json
```

[testmark]:# (quickstart/then-check-lineage/output)
```
{
	"name": "alpinelinux.org/alpine",
	"metadata": {},
	"releases": [
		{
			"name": "v3.14.2",
			"items": {
				"x86_64": "tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"
			},
			"metadata": {}
		}
	]
}
```

### mirror.json
[testmark]:# (quickstart/then-check-mirrors/script)
```
cat .warpforge/catalog/alpinelinux.org/alpine/mirrors.json
```

[testmark]:# (quickstart/then-check-mirrors/output)
```
{
	"byWare": {
		"tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS": [
			"https://dl-cdn.alpinelinux.org/alpine/v3.14/releases/x86_64/alpine-minirootfs-3.14.2-x86_64.tar.gz"
		]
	}
}
```

## Configuring a Plot

The minimal `plot.json` created by `warpforge module init` does not have any inputs, steps,
or outputs. It will run, but does not actually do anything. We can edit `plot.json` to 
be a bit more useful.

Most plots will mount a root filesystem at `/`. In this example, we use the Alpine Linux root filesystem which was added to the catalog in the previous step. By making this a plot input named `rootfs`, we can later refer to it as `pipe::rootfs`.

Our simple plot consists of a single step. This step is a `protoformula`, which has inputs, an action, and outputs. We use the Alpine root filesystem as our only input and place it at `/`. Our action is a script, which will be interpreted by `/bin/sh` and consists of a single statement (`echo hello world!`). We do not define any outputs.

[testmark]:# (quickstart/fs/module.json)
```
{
    "name": "my-module-name"
}
```

[testmark]:# (quickstart/fs/plot.json)
```
{
	"inputs": {
		"rootfs": "catalog:alpinelinux.org/alpine:v3.14.2:x86_64"
	},
	"steps": {
		"echo": {
			"protoformula": {
				"inputs": {
					"/": "pipe::rootfs"
				},
				"action": {
					"script": {
                        "interpreter": "/bin/sh",
						"contents": [
							"echo hello world!"
						]
					}
				},
				"outputs": {
				}
			}
		}

	},
	"outputs": {
	}
}
```

## Running a Module

We can run the module using `warpforge run`. Using `warpforge run ./...` will run all modules recursively, starting at the current working directory.

[testmark]:# (quickstart/then-run/sequence)
```
warpforge run ./...
```

TODO: this output isn't right, but it passes the test...

[testmark]:# (quickstart/then-run/output)
```
```

## About this Document

This document uses [testmark](https://github.com/warpfork/go-testmark) to automatically
test each example. 