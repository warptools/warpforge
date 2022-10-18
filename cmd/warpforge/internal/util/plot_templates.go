package util

const DefaultPlotJson = `
{
    "plot.v1": {
        "inputs": {
            "rootfs": "catalog:warpsys.org/busybox:v1.35.0:amd64-static"
        },
        "steps": {
            "hello-world": {
                "protoformula": {
                    "inputs": {
                        "/": "pipe::rootfs"
                    },
                    "action": {
                        "script": {
                            "interpreter": "/bin/sh",
                            "contents": [
                                "mkdir /output",
                                "echo 'hello world' | tee /output/file"
                            ],
                            "network": false
                        }
                    },
                    "outputs": {
                        "out": {
                            "from": "/output",
                            "packtype": "tar"
                        }
                    }
                }
            }
        },
        "outputs": {
            "output": "pipe:hello-world:out"
        }
    }
}
`

const FerkPlotTemplate = `
{
    "inputs": {
        "rootfs": "catalog:min.warpforge.io/debian/rootfs:bullseye-1646092800:amd64"
    },
    "steps": {
        "ferk": {
            "protoformula": {
                "inputs": {
                    "/": "pipe::rootfs",
                    "/pwd": "mount:overlay:."
                },
                "action": {
                    "script": {
                        "interpreter": "/bin/bash",
                        "contents": [
                            "echo 'APT::Sandbox::User \"root\";' > /etc/apt/apt.conf.d/01ferk",
                            "echo 'Dir::Log::Terminal \"\";' >> /etc/apt/apt.conf.d/01ferk",
                            "/bin/bash"
                        ],
                        "network": true
                    }
                },
                "outputs": {
                    "out": {
                        "from": "/out",
                        "packtype": "tar"
                    }
                }
            }
        }
    },
    "outputs": {
        "out": "pipe:ferk:out"
    }
}
`
