CLI
===

## check

### Check a Formula is Valid

[testmark]:# (checkformula/sequence)
```
warpforge --verbose check formula.json
```

[testmark]:# (checkformula/output)
```
ok: struct<FormulaAndContext>{
	formula: struct<Formula>{
		inputs: map<Map__SandboxPort__FormulaInput>{
			union<SandboxPort>{string<SandboxPath>{""}}: union<FormulaInput>{union<FormulaInputSimple>{struct<WareID>{
				packtype: string<Packtype>{"tar"}
				hash: string<String>{"7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS"}
			}}}
		}
		action: union<Action>{struct<Action_Exec>{
			command: list<List__String>{
				0: string<String>{"/bin/sh"}
				1: string<String>{"-c"}
				2: string<String>{"echo hello from warpforge!"}
			}
		}}
		outputs: map<Map__OutputName__GatherDirective>{
		}
	}
	context: struct<FormulaContext>{
		warehouses: map<Map__WareID__WarehouseAddr>{
		}
	}
}
```

### Check a Plot is Valid

[testmark]:# (checkplot/sequence)
```
warpforge check plot.json
```

## run

### Execute a Formula

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

### Execute a Module

[testmark]:# (runmodule/sequence)
```
warpforge --json run module.json
```

[testmark]:# (runmodule/output)
```
{
	"test": "tar:4mjq8TRFaprkK3pae5ZbjrJkWetGrEYszVW2WbcELd8vfpnwHpqkLzo4Q6wkfVRCGp"
}
```
