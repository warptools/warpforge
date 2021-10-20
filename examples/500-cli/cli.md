CLI Examples
============

## Recursively Run Modules

Using `./...` will traverse recursively from `.` and execute all `module.json` files found.

[testmark]:# (runall/sequence)
```
warpforge run ./...
```

[testmark]:# (runall/output)
```
```


## Check Input Files are Valid

This usage will infer the file's type based on its name, then check it for validity.
Multiple files can be provided, and unrecoginzed filenames will be ignored.

[testmark]:# (check/sequence)
```
warpforge check *
```

[testmark]:# (check/output)
```
```

## Check a Formula is Valid
[testmark]:# (checkformula/sequence)
```
warpforge --json --verbose formula check formula.json
```

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

[testmark]:# (runformula/sequence)
```
warpforge --json run formula.json
```

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

## Execute a Module

[testmark]:# (runmodule/sequence)
```
warpforge --json run module.json
```

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
