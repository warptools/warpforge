warpforge
=========

Putting things together. Consistently.

Many of the docs and much of coordination work is in [Notion](https://www.notion.so/warpforge/Welcome-6653d3362db84ad8a2b0d2a0046748b7) --
check out the content there.
Especially check out the [API Specs](https://www.notion.so/warpforge/API-Specs-41830e3da58646d2927ef6ae5b2902e4),
because those are going to tell you a lot about _exactly_ what this tool will do for you.

Here are some terminal movies of what you can do with warpforge:

- Using the quickstart and writing a module:

    [Warpforge Quickstart Demo ![](https://asciinema.org/a/ax3iU4aRu17Cx4CG1OYBNCPb6.png?t=38)](https://asciinema.org/a/ax3iU4aRu17Cx4CG1OYBNCPb6)

- Adding content to a catalog from the web:

    [Plot Demo ![](https://asciinema.org/a/XL03vvethmuqnA1iNJx2xDsRD.png)](https://asciinema.org/a/XL03vvethmuqnA1iNJx2xDsRD)

- Compute using hashes — this is the low-level API on display:

    [Formula Demo ![](https://asciinema.org/a/FY4iYhlEi5m0h78oFYqqvIZYc.png)](https://asciinema.org/a/FY4iYhlEi5m0h78oFYqqvIZYc)

- Awk in a box!  This is what it looks like to combine a tool and a dataset (think: reproducible scientific compute, or distributable bigdata jobs):

    [awk in a box ![](https://asciinema.org/a/CqifX73Z2JwDwLOi7DLm5El1h.png)](https://asciinema.org/a/CqifX73Z2JwDwLOi7DLm5El1h)


Getting Started
---------------

- [Install](#install)
- Try the [Quickstart](#quickstart)
- Check out the [Examples](./examples/) dir for more examples


Install
-------

Check that you have Go 1.16+ and that `$GOPATH/bin` (by default, `$HOME/go/bin`) is in your `PATH`.

To install `warpforge` and the required plugins to your `GOPATH`, run:

```
git clone https://github.com/warpfork/warpforge
cd warpforge
make install
```

Warpforge will now be installed, probably in `$HOME/go/bin`.

(We also often symlink it to `wf` for short: `cd $HOME/go/bin && ln -s warpforge wf` .)


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
Successfully created module.wf and plot.wf for module "hello-world".
Ensure your catalogs are up to date by running `warpforge catalog update.`.
You can check status of this module with `warpforge status`.
You can run this module with `warpforge run`.
Once you've run the Hello World example, edit the 'script' section of plot.wf to customize what happens.
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
        /home/user/warpforge-quickstart (pwd, module)
        /home/user (root workspace, home workspace)

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
│  plot                 ref = catalog:alpinelinux.org/alpine:v3.15.0:x86_64
│  plot                 wareId = tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN
│  plot                 wareAddr = https://dl-cdn.alpinelinux.org/alpine/v3.15/releases/x86_64/alpine-minirootfs-3.15.0-x86_64.tar.gz
├─ plot  (hello-world) evaluating protoformula
│ ┌─ formula  
│ │  formula  ware mount:       wareId = tar:57j2Ee9HEtDxRLE6uHA1xvmNB2LgqL3HeT5pCXr7EcXkjcoYiGHSBkFyKqQuHFyGPN destPath = /
│ │  formula  executing script  interpreter = /bin/sh
│ │ ┌─ output   
│ │ │  output   hello world
│ │ └─ output   
│ │  formula  packed "out":     path = /output  wareId=tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
│ │  formula  RunRecord:
│ │  formula    GUID = 4f09fa04-cf02-4ad6-a2db-c0cb8b1aaf1d
│ │  formula    FormulaID = bafyrgqb55m6gejlpmbta3wyfvdqtxco3k7ajzyjrnoytqgxhxsj4gvgz33b6bqrhv6f4mbejnsdhekx6wvus6rrv4hmblu5r2nrt5jtlygvg6
│ │  formula    Exitcode = 0
│ │  formula    Time = 1642008026
│ │  formula    Results:
│ │  formula            out: tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
│ └─ formula  
│  plot  (hello-world) collected output hello-world:out
├─ plot  (hello-world) complete
│  plot  
│  plot  outputs:
│  plot         output -> tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
└─ plot  
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
