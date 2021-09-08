package plotexec

import (
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/warpfork/warpforge/wfapi"
)

func TestExecSingleStepPlot(t *testing.T) {
	serial := `{
	"inputs": {
		"rootfs": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB"
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
		}
	},
	"outputs": {
		"test": "pipe:one:test"
	}
}
`

	p := wfapi.Plot{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &p, wfapi.TypeSystem.TypeByName("Plot"))
	qt.Assert(t, err, qt.IsNil)

	_, err = Exec(p)
	qt.Assert(t, err, qt.IsNil)
}

func TestExecMultiStepPlot(t *testing.T) {
	serial := `{
	"inputs": {
		"rootfs": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB"
	},
	"steps": {
		"zero": {
			"protoformula": {
				"inputs": {
					"/": "pipe::rootfs"
				},
				"action": {
					"exec": {
						"command": [
							"/bin/sh",
							"-c",
							"mkdir /test; echo 'hello from step zero' > /test/file"
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
					"exec": {
						"command": [
							"/bin/sh",
							"-c",
							"echo 'in step one'; cat /test/file"
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
`

	p := wfapi.Plot{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &p, wfapi.TypeSystem.TypeByName("Plot"))
	qt.Assert(t, err, qt.IsNil)

	order, err := OrderSteps(p)
	qt.Assert(t, err, qt.IsNil)
	fmt.Println(order)

	_, err = Exec(p)
	qt.Assert(t, err, qt.IsNil)
}

func TestNestedPlot(t *testing.T) {
	serial := `{
	"inputs": {
		"rootfs": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB"
	},
	"steps": {
		"zero-outer": {
			"plot": {
				"inputs": {
					"rootfs": "ware:tar:47Yg1Sdq21rPyDw9X9sCmRubQUADhFKe9G7qZCJRe61RhWPCxcQysCFzyCHffBKRjB"
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
					}
					"one-inner": {
						"protoformula": {
							"inputs": {
								"/": "pipe::rootfs"
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
`

	p := wfapi.Plot{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &p, wfapi.TypeSystem.TypeByName("Plot"))
	qt.Assert(t, err, qt.IsNil)

	order, err := OrderSteps(p)
	qt.Assert(t, err, qt.IsNil)
	fmt.Println(order)

	r, err := Exec(p)
	qt.Assert(t, err, qt.IsNil)

	nResults := bindnode.Wrap(&r, wfapi.TypeSystem.TypeByName("PlotResults"))
	fmt.Println(printer.Sprint(nResults))

}

func TestCycleFails(t *testing.T) {
	serial := `{
	"inputs": {},
	"steps": {
		"zero": {
			"protoformula": {
				"inputs": {
					"/": "pipe:one:out"
				},
				"action": {
					"exec": {
						"command": [
							"/bin/echo"
						]
					}
				},
				"outputs": {
					"out": {
						"from": "/",
						"packtype": "tar"
					}
				}
			}
		},
		"one": {
			"protoformula": {
				"inputs": {
					"/": "pipe:zero:out",
				},
				"action": {
					"exec": {
						"command": [
							"/bin/echo"
						]
					}
				},
				"outputs": {
					"out": {
						"from": "/",
						"packtype": "tar"
					}
				}
			}
		}

	},
	"outputs": {}
}
`

	p := wfapi.Plot{}
	_, err := ipld.Unmarshal([]byte(serial), json.Decode, &p, wfapi.TypeSystem.TypeByName("Plot"))
	qt.Assert(t, err, qt.IsNil)

	// this will fail due to a dependency cycle between steps zero and one
	_, err = OrderSteps(p)
	qt.Assert(t, err, qt.IsNotNil)
}
