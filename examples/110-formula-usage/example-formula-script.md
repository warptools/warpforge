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
			"/": "ware:tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN"
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
			"tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN": "https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/x86_64/alpine-minirootfs-3.15.0-x86_64.tar.gz"
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
	"formulaID": "bafyrgqf27e6rwqjv7zrya6jaxsf3z3fbp3czzn4skllljq3lvdrv7ednfi77yczrphups7y6h3mc7p4dtnjsvrxxlgbz7utgsjt2cubbqzf4i",
	"exitcode": 0,
	"results": {}
}
```
