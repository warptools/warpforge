Example of executing single processes with a formula
====================================================

This document contains examples of formulas that execute and produce a result.

These examples make use of a minimal Alpine rootfs which must be present in the workspace
for tests to work. This test workspace can be setup by running the `setup-test-workspace.sh` script.

---

These fixtures are executed by tests in the `pkg/formulaexec` package.

These fixtures are executable by parsing them using
the [testmark](https://github.com/warpfork/go-testmark) format.
Each example should have a `example/formula` value and optionally a 
`example/runrecord` value. 

---

## Example: Echo

This formula echos a value to stdout.

### Formula

[testmark]:# (echo/formula)
```json
{
	"formula": {
		"inputs": {
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"
		},
		"action": {
			"exec": {
				"command": ["/bin/sh", "-c", "echo hello from warpforge!"]
			}
		},
		"outputs": {
		}
	},
	"context": {
		"warehouses": {}
	}
}
```

### RunRecord

[testmark]:# (echo/runrecord)
```json
{
	"guid": "4f1c8e65-9875-4482-bdd5-f1fa78625e88",
	"time": 1631717098,
	"formulaID": "bafyrgqc7oxykn4nsfru4snk33vumhszb25zehgnrqkusuk7rx3umaubnv7u3oye7awaeipif4u3wtkpxisk3cofhjc7gzcd3xscvb3z4xh7qy",
	"exitcode": 0,
	"results": {}
}
```

## Example: Packing

This formula creates a file (`/out/test`), then packs the `/out` directory containing that file.
Note the RunRecord now contains a `results` value which includes the output.

### Formula
[testmark]:# (pack/formula)
```json
{
	"formula": {
		"inputs": {
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"
		},
		"action": {
			"exec": {
				"command": ["/bin/sh", "-c", "mkdir /out; echo hello from warpforge! > /out/test"]
			}
		},
		"outputs": {
			"test": {
				"from": "/out",
				"packtype": "tar"
			},

		}
	},
	"context": {
		"warehouses": {}
	}
}
```

### RunRecord
[testmark]:# (pack/runrecord)
```json
{
	"guid": "f63741da-e6e9-4d34-95ab-2342fe000a1c",
	"time": 1631717580,
	"formulaID": "bafyrgqeajmqooakjt2ich4hsehl2n7lljavunopvxt5hwkuvdbazc2y6m5ylztr3x6pvf4ydrvjze5zhtebhyca7iffba7yumptvfhmi3ug56",
	"exitcode": 0,
	"results": {
		"test": "ware:tar:1we9CNTxLVQWRu2KvfyhugbAydJezzh4DWU4rmvNzds7PxgE7fbRQi2MQwC76LSo5"
	}
}
```

## Example: Directory Mount Input

This example mounts the current working directory (`.`) to `/work` using the input
`mount:type:.`.

TODO: the mount type is set to `type` here, since mount types currently have no impact

### Formula
[testmark]:# (dirmount/formula)
```json
{
	"formula": {
		"inputs": {
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS",
			"/work": "mount:type:."
		},
		"action": {
			"exec": {
				"command": ["/bin/sh", "-c", "ls -al /work"]
			}
		},
		"outputs": {
		}
	},
	"context": {
		"warehouses": {}
	}
}
```

### RunRecord
[testmark]:# (dirmount/runrecord)
```json
{
	"guid": "dd774fa1-07f2-4403-8fdf-aa4272389a1c",
	"time": 1631717847,
	"formulaID": "bafyrgqg3gmutfoop2zr7ohe5ntdd3jfr5pbqxrj3fw3kz5xrio23bwmwe6sv4arb2kgff4r7cgoakgnyd2a6zixiphbry72fl6ulxivjhvbty",
	"exitcode": 0,
	"results": {}
}
```
