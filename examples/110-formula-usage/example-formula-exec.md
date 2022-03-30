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
		"inputs": {
			"/": "ware:tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN"
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
		"warehouses": {
			"tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN": "https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/x86_64/alpine-minirootfs-3.15.0-x86_64.tar.gz"
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
	"formulaID": "bafyrgqhdywzmzh4bocoldc3ncwwbucuzzjkiadib5khpvhgi33qx7k63kg5i53rmszd6t2a7agdue6vuihjwx7whvciiktkwytczeronejynk",
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
			"/": "ware:tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN"
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
		"warehouses": {
			"tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN": "https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/x86_64/alpine-minirootfs-3.15.0-x86_64.tar.gz"
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
	"formulaID": "bafyrgqfmj3qxe4lb5liyr7o6aumzwrscsxpgfccpqoomecicdvzvinqpfb55txxfwxmfed7knonxaki6qwhu7q23kiwzeqtahgcohs63b2ste",
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
		"inputs": {
			"/": "ware:tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN",
			"/work": "mount:overlay:."
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
		"warehouses": {
			"tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN": "https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/x86_64/alpine-minirootfs-3.15.0-x86_64.tar.gz"
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
	"formulaID": "bafyrgqaqkncrcgdpkglphvc66btkxp26ldgsiz2rk7xdbqygalzta4z4ugz65f6mlwja5lsazrckbrklqarmjgndcmdlk5p3yov6kgg2bgs2i",
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
		"inputs": {
			"/": {
				"basis": "ware:tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN",
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
	},
	"context": {
		"warehouses": {
			"tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN": "https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/x86_64/alpine-minirootfs-3.15.0-x86_64.tar.gz"
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
	"formulaID": "bafyrgqd7ym6wxhvmdbcqc5kuh7763uqb7tktid5f7azblfibc2ipetpighyk2peurqzdtqff7sfk3j44hzk3vpbthrogg36jsyrswwlznr5k4",
	"exitcode": 0,
	"results": {}
}
```
