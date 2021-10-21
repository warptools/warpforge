Script Formula Actions
==========================================================

The `script` Action can be used to execute a series of commands.

This formula uses a script action to perform multiple commands. The `script` Action takes an intrepreter
to use (`/bin/sh` in this example) and a list of `contents` strings. The contents are each executed sequentially
in the same process, allowing variables to be used between them. This should work with any POSIX compliant shell
interpreter (`sh`, `bash`, `zsh`, etc...).

### Formula

[testmark]:# (script/formula)
```json
{
	"formula": {
		"inputs": {
			"/": "ware:tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"
		},
		"action": {
			"script": {
				"interpreter": "/bin/sh",
				"contents": [
					"MESSAGE='hello, this is a script action'",
					"echo $MESSAGE",
					"echo done!"
				]
			}
		},
		"outputs": {
		}
	},
	"context": {
		"warehouses": {
			"tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS": "https://dl-cdn.alpinelinux.org/alpine/v3.14/releases/x86_64/alpine-minirootfs-3.14.2-x86_64.tar.gz"
		}
	}
}
```

### RunRecord

[testmark]:# (script/runrecord)
```json
{
	"guid": "cb351d5f-9b85-4404-aec9-b54cb71d249c",
	"time": 1634850353,
	"formulaID": "bafyrgqczb4b4iy52qpyl4gpnxcabop4mmjk722y72wen3qko2wzoso4qoycesetst5zro3p6y7xa5sbkawdmnd6cpnqjumfzo4xdoezpbu2d2",
	"exitcode": 0,
	"results": {}
}
```
