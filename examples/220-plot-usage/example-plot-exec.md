Example of executing plots
==========================

This document contains examples of plots that execute and produce a result.

---

These fixtures are executed by tests in the `pkg/plotexec` package.

These fixtures are executable by parsing them using
the [testmark](https://github.com/warpfork/go-testmark) format.
Each example should have a `example/plot` value and optionally
`example/order` and `example/plotresults` values.

---

## Example: Single Step Plot

This plot has a single protoformula step, which creates a file. This file is used as a plot output.

### Plot

[testmark]:# (singlestep/plot)
```json
{
	"inputs": {
		"rootfs": "catalog:alpinelinux.org/alpine:v3.14.2:x86_64"
	},
	"steps": {
		"one": {
			"protoformula": {
				"inputs": {
					"/": "pipe::rootfs"
				},
				"action": {
					"exec": {
						"command": [
							"/bin/sh",
							"-c",
							"echo test > /test"
						],
						"network": false
					}
				},
				"outputs": {
					"test": {
						"from": "/test",
						"packtype": "tar"
					}
				}
			}
		}
	},
	"outputs": {
		"test": "pipe:one:test"
	}
}
```

### Execution Order
[testmark]:# (singlestep/order)
```
[one]
```

### PlotResults

[testmark]:# (singlestep/plotresults)
```json
{
	"test": "tar:4mjq8TRFaprkK3pae5ZbjrJkWetGrEYszVW2WbcELd8vfpnwHpqkLzo4Q6wkfVRCGp"
}
```

## Example: Multi Step Plot

This plot has multiple steps. Step zero creates a file, which is used as an input to step one.
The execution order is automatically determined.

### Plot

[testmark]:# (multistep/plot)
```json
{
	"inputs": {
		"rootfs": "catalog:alpinelinux.org/alpine:v3.14.2:x86_64"
	},
	"steps": {
		"zero": {
			"protoformula": {
				"inputs": {
					"/": "pipe::rootfs"
				},
				"action": {
					"script": {
						"interpreter": "/bin/sh",
						"contents": [
							"mkdir /test",
							"echo 'hello from step zero' > /test/file"
						]
					}
				},
				"outputs": {
					"test": {
						"from": "/test",
						"packtype": "tar"
					}
				}
			}
		},
		"one": {
			"protoformula": {
				"inputs": {
					"/": "pipe::rootfs",
					"/test": "pipe:zero:test"
				},
				"action": {
					"script": {
						"interpreter": "/bin/sh",
						"contents": [
							"echo 'in step one'",
							"cat /test/file"
						]
					}
				},
				"outputs": {}
			}
		}

	},
	"outputs": {}
}
```

### Execution Order
[testmark]:# (multistep/order)
```
[zero one]
```

### PlotResults

This plot has no outputs.

[testmark]:# (multistep/plotresults)
```json
{}
```

## Example: Nested Plots

This example contains a nested plot step. The `zero-outer` step contains a full multistep plot,
which produces an output (`pipe:zero-outer:test`). This output is used in the `one-outer` step.
The execution order is computed automatically. 

### Plot

[testmark]:# (nested/plot)
```json
{
	"inputs": {
		"rootfs": "catalog:alpinelinux.org/alpine:v3.14.2:x86_64"
	},
	"steps": {
		"zero-outer": {
			"plot": {
				"inputs": {
					"rootfs": "catalog:alpinelinux.org/alpine:v3.14.2:x86_64"
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
					},
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
```

### Execution Order
[testmark]:# (nested/order)
```
[zero-outer zero-inner one-inner one-outer]
```

### PlotResults

[testmark]:# (nested/plotresults)
```json
{
	"test": "tar:3P7pTG7U7ezdpSJMKBHr6mVAUSC6yHsgYgXqwUkDJ9wcVeY4KT9okcZZnsvKwHhRH5"
}
```

## Example: Multiple Input Types

This plot uses the `catalog` and `mount` input types, then uses `pipe` to reference them in
the protoformula.

### Plot

[testmark]:# (input-types/plot)
```json
{
	"inputs": {
		"rootfs": "catalog:alpinelinux.org/alpine:v3.14.2:x86_64",
		"pwd": "mount:overlay:."
	},
	"steps": {
		"one": {
			"protoformula": {
				"inputs": {
					"/": "pipe::rootfs",
					"/pwd": "pipe::pwd"
				},
				"action": {
					"exec": {
						"command": [
							"/bin/sh",
							"-c",
							"ls /pwd"
						]
					}
				},
				"outputs": {}
			}
		}
	},
	"outputs": {}
}
```

### Execution Order
[testmark]:# (input-types/order)
```
[one]
```

### PlotResults

[testmark]:# (input-types/plotresults)
```json
{}
```