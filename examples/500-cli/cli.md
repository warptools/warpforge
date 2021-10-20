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

## Recursively Run Modules

Using `./...` will traverse recursively from `.` and execute all `module.json` files found.

[testmark]:# (runall/sequence)
```
warpforge run ./...
```

This works even if there are no modules found; the output will simply be empty:

[testmark]:# (runall/output)
```
```


## Check Input Files are Valid

This usage will infer the file's type based on its name, then check it for validity.
Multiple files can be provided, and unrecognized filenames will be ignored.

[testmark]:# (check/sequence)
```
warpforge check *
```

If everything passes, the output is silent, as good unix-style tools should be:

[testmark]:# (check/output)
```
```

(TODO: this probably needs a verbose flag, too, so we can more easily see that it's actually doing work!)

## Check a Formula is Valid

Specific formats can also be checked explicitly, using `check` commands specific to that subsystem:

[testmark]:# (checkformula/sequence)
```
warpforge --json --verbose formula check formula.json
```

Together with the verbosity and output formatting flags used above, this will also emit the checked document again:

[testmark]:# (checkformula/output)
```
{
	"formula": {
		"inputs": {
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"
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
	},
	"context": {
		"warehouses": {
			"tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS": "https://dl-cdn.alpinelinux.org/alpine/v3.14/releases/x86_64/alpine-minirootfs-3.14.2-x86_64.tar.gz"
		}
	}
}
```

## Check a Module is Valid

[testmark]:# (checkmodule/sequence)
```
warpforge --verbose module check module.json
```

[testmark]:# (checkmodule/output)
```
ok: struct<Module>{
	name: string<ModuleName>{"test"}
	plot: absent
}
```

## Check a Plot is Valid

[testmark]:# (checkplot/sequence)
```
warpforge plot check plot.json
```

[testmark]:# (checkplot/output)
```
```

## Execute a Formula

Excuting a formula is done with the `warpforge run` command.
When given a formula file, it knows what to do:

[testmark]:# (runformula/sequence)
```
warpforge --json run formula.json
```

The result of this will be a `RunRecord` object printed to stdout:

[testmark]:# (runformula/output)
```
{
	"guid": "389c442f-5343-497e-b74d-d31fd487af53",
	"time": "22222222222",
	"formulaID": "bafyrgqc7oxykn4nsfru4snk33vumhszb25zehgnrqkusuk7rx3umaubnv7u3oye7awaeipif4u3wtkpxisk3cofhjc7gzcd3xscvb3z4xh7qy",
	"exitcode": 0,
	"results": {}
}
```

(Note that we've normalized some of the values in this object for testing purposes.
When you run this command yourself, the time recorded in the runrecord will of course vary, as will the runrecord's guid!)

## Execute a Module

Excuting a module or a plot looks almost exactly the same as executing a formula --
it's still just the `warpforge run` command, which will figure out what to do with whatever you give it:

[testmark]:# (runmodule/sequence)
```
warpforge --json run module.json
```

The output for evaluating a module is a bit terser: it only emits the results object,
which has keys matching the outputs that the plot labels for extraction.
(Future: this will probably change :) and we might expect to see more progress details here as well.)

[testmark]:# (runmodule/output)
```
{
	"test": "tar:3P7pTG7U7ezdpSJMKBHr6mVAUSC6yHsgYgXqwUkDJ9wcVeY4KT9okcZZnsvKwHhRH5"
}
```

## Graph a Plot

[testmark]:# (graphplot/sequence)
```
warpforge plot graph --png graph.png plot.json
```

![Plot Graph](graph.png)
