Example of executing single processes with a formula
====================================================

This document contains examples of formulas that execute and produce a result.

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
		"formula.v1": {
			"inputs": {
				"/": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
			},
			"action": {
				"exec": {
					"command": ["/bin/sh", "-c", "echo hello from warpforge!"]
				}
			},
			"outputs": {
			}
		}
	},
	"context": {
		"context.v1": {
			"warehouses": {}
		}
	}
}
```

### RunRecord

[testmark]:# (echo/runrecord)
```json
{
	"guid": "4f1c8e65-9875-4482-bdd5-f1fa78625e88",
	"time": 1631717098,
	"formulaID": "zM5K3Zz8R3ioVVWZ6o6GocxPKvubAJfv4iQmDH3GCq9UjtDjHtRWrry4DRoEBPvfUEYFx1D",
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
		"formula.v1": {
			"inputs": {
				"/": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"
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
		}
	},
	"context": {
		"context.v1": {
			"warehouses": {}
		}
	}
}
```

### RunRecord
[testmark]:# (pack/runrecord)
```json
{
	"guid": "f63741da-e6e9-4d34-95ab-2342fe000a1c",
	"time": 1631717580,
	"formulaID": "zM5K3Su1Nwvz8MsCVct3FWzuJN922CFxoLB4KCXNQJPDC8NCmWaB1Ao32DwiyDjzRLhEiep",
	"exitcode": 0,
	"results": {
		"test": "ware:tar:7wjdwS2Bn5faUcq6t156Je8KY9CoSejC4vMbvTQNeKzdeNLzt4sEtzKQ6H56x6KuD7"
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
		"formula.v1": {
			"inputs": {
				"/": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9",
				"/work": "mount:overlay:."
			},
			"action": {
				"exec": {
					"command": ["/bin/sh", "-c", "ls -al /work"]
				}
			},
			"outputs": {
			}
		}
	},
	"context": {
		"context.v1": {
			"warehouses": {}
		}
	}
}
```

### RunRecord
[testmark]:# (dirmount/runrecord)
```json
{
	"guid": "dee7c993-d653-45d2-b299-3b1cdec4e28d",
	"time": 1633531181,
	"formulaID": "zM5K3WPjAehei8Z2gZaknSfvkF9bhDTnLuozSjj3uoBUyYGrkjkckLNyTMU2xaKZwn6vkAB",
	"exitcode": 0,
	"results": {}
}
```

## Example: Complex Input

This formula uses an input with filters.

### Formula

[testmark]:# (complexinput/formula)
```json
{
	"formula": {
		"formula.v1": {
			"inputs": {
				"/": {
					"basis": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9",
					"filters": {
						"setid": "ignore"
					}
				}
			},
			"action": {
				"exec": {
					"command": ["/bin/sh", "-c", "echo hello from warpforge!"]
				}
			},
			"outputs": {
			}
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

### RunRecord

[testmark]:# (complexinput/runrecord)
```json
{
	"guid": "2355eefb-2e93-4183-bf3b-e04b0150b86a",
	"time": 1633531905,
	"formulaID": "zM5K3TrdW6z58mTzH5b8DGqMkm4Bxnx9YtMomAJpvFYdmkz1mcpyY5fw2cxvjzCoNTRFH5c",
	"exitcode": 0,
	"results": {}
}
```

## Mount Types
[testmark]:# (mounttypes/formula)
```json
{
	"formula": {
		"formula.v1": {
			"inputs": {
				"/": "ware:tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9",
				"/mnt/overlay": "mount:overlay:.",
				"/mnt/ro": "mount:ro:.",
				"/mnt/rw": "mount:rw:."
			},
			"action": {
				"exec": {
					"command": ["/bin/sh", "-c", "ls -al /mnt"]
				}
			},
			"outputs": {
			}
		},
	}
	"context": {
		"context.v1": {
			"warehouses": {
			}
		}
	}
}
```