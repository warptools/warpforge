warpforge
=========
Putting things together. Consistently.

Many of the docs and much of coordination work is in [Notion](https://www.notion.so/warpforge/Welcome-6653d3362db84ad8a2b0d2a0046748b7) --
check out the content there.
Especially check out the [API Specs](https://www.notion.so/warpforge/API-Specs-41830e3da58646d2927ef6ae5b2902e4),
because those are going to tell you a lot about _exactly_ what this tool will do for you.

Here are some terminal movies of what you can do with warpforge:

- Describing computations using filesystems named by catalogs: https://asciinema.org/a/XL03vvethmuqnA1iNJx2xDsRD (This is the "L2" API -- e.g. the thing that has human-readable names.)
- Describing computations using all hashes: https://asciinema.org/a/FY4iYhlEi5m0h78oFYqqvIZYc (This is the "L1" API -- it's purely content-addressable, cryptographic hashes -- no fun to write by hand, but extremely reproducible!  The "L2" API generates this for you.)


Install
-------

Check that you have Go 1.16+ and that `$GOPATH/bin` (by default, `$HOME/go/bin`) is in your `PATH`.

To install `warpforge` and the required plugins to your `GOPATH`, run:

```
make install
```

Quickstart
----------

After installing, create a new directory and run the quickstart subcommand.
In this example, we will create a module named "hello-world".

```
mkdir warpforge-quickstart
cd warpforge-quickstart
warpforge quickstart hello-world
```

```
Successfully created module.json and plot.json for module "hello-world".
Ensure your catalogs are up to date by running `warpforge catalog update.`.
You can check status of this module with `warpforge status`.
You can run this module with `warpforge run`.
Once you've run the Hello World example, edit the 'script' section of plot.json to customize what happens.
```

Next, update the catalog:

```
warpforge catalog update
```

```
installing default catalog to /home/eric/.warpforge/catalogs/default-remote... done.
default-remote: already up to date
```

We can check the status of our module and configuration with the status subcommand:

```
warpforge status
```

```
Workspace:
        /tmp/warpforge-quickstart (pwd, module)
        /home/eric (root workspace, home workspace)

You can evaluate this module with the `warpforge run` command.
```

If everything looks good, we can run the module:

```
warpforge run
```

```
┌─ plot  
│  plot  inputs:
│  plot         type = catalog
│  plot                 ref = catalog:alpinelinux.org/alpine:v3.14.2:x86_64
│  plot                 wareId = tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS
│  plot                 wareAddr = https://dl-cdn.alpinelinux.org/alpine/v3.14/releases/x86_64/alpine-minirootfs-3.14.2-x86_64.tar.gz
├─ plot  (hello-world) evaluating protoformula
 ┌─ formula  
 │  formula  ware mount:        wareId = tar:7P8nq1YY361BSEvgsSU3gu4ot1U5ieiFey2XyvMoTM7Mhwg3mo8aV2KyGwwrKRLtxS destPath = /
 │  formula  executing script   interpreter = /bin/sh
 │  formula  ->  hello world
 │  formula  packed "out":      path = /output  wareId=tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
 │  formula  RunRecord:
 │  formula     GUID = c728996f-2951-478f-bd60-dc662183fde3
 │  formula     FormulaID = bafyrgqb55m6gejlpmbta3wyfvdqtxco3k7ajzyjrnoytqgxhxsj4gvgz33b6bqrhv6f4mbejnsdhekx6wvus6rrv4hmblu5r2nrt5jtlygvg6
 │  formula     Exitcode = 0
 │  formula     Time = 1641936529
 │  formula     Results:
 │  formula             out: tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
 └─ formula  
│  plot  (hello-world) collected output hello-world:out
├─ plot  (hello-world) complete
│  plot  
│  plot  outputs:
│  plot         output -> tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
└─ plot  
ok: map<PlotResults>{
        string<LocalLabel>{"output"}: struct<WareID>{
                packtype: string<Packtype>{"tar"}
                hash: string<String>{"6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA"}
        }
}
```


Running Tests
-------------

All tests can be executed using:

```
make test
```

License
-------

SPDX-License-Identifier: Apache-2.0 OR MIT
