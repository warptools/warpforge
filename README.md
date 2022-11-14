warpforge
=========

Putting things together. Consistently.

Many of the docs and much of coordination work is in [Notion](https://warpforge.notion.site/Welcome-6653d3362db84ad8a2b0d2a0046748b7) on http://warpforge.io/ --
check out the content there.
Especially check out the [API Specs](https://warpforge.notion.site/API-Specs-41830e3da58646d2927ef6ae5b2902e4),
because those are going to tell you a lot about _exactly_ what this tool will do for you.

Warpforge is solving three problems:

- **One**: we need computation that's "hashes go in, hashes come out".  We need this as sheer foundation.
	- Warpforge runs processes based on instructions in a JSON API for saying what data hashes to use as inputs, what script to run on them, and what outputs you want collected.  The outputs will be hashed, and Warpforge tells you the result (in more JSON).
	- Warpforge does this with containers and filesystems: for heremeticism and reproducibility, and because we care about working with the vast wealth of software that humanity has already produced.
- **Two**: we need some human-readable naming system to label the hashes, so humans can use this system: say what they want, build update conventions, etc.  (People won't copy and paste hashes manually: we need tools for communcating about data.)
	- Warpforge solves this with an API layer called "catalogs".  Catalogs are still content-addressable data: you can easily snapshot them and refer to them by hashes (so you can compose secure, reliable, and decentralized systems with them).
- **Three**: we want packages and executables that are isolated and work under a wide range of conditions, with minimal dependencies.  Things should be simple; simple things work better, and are easier to collaborate on.
	- Warpforge itself doesn't do anything about this...
	- Warpsys, however, is a suite of packages made *with* Warpforge that's all about this.

By targetting all three of these problems at once, we hope to make more reliable computers, and more productive environments.


Demos
-----

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
git clone https://github.com/warptools/warpforge
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
│  plot                 ref = catalog:warpsys.org/busybox:v1.35.0:amd64-static
│  plot                 wareId = tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9
│  plot                 wareAddr = https://warpsys.s3.amazonaws.com/warehouse/4z9/DCT/4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9
├─ plot  (hello-world) evaluating protoformula
│ ┌─ formula  
│ │  formula  ware mount:       wareId = tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9 destPath = /
│ │  formula  executing script  interpreter = /bin/sh
│ │ ┌─ output   
│ │ │  output   hello world
│ │ └─ output   
│ │  formula  packed "out":     path = /output  wareId=tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
│ │  formula  RunRecord:
│ │  formula    GUID = 67439882-1411-4d4d-8510-9d73cd72b38e
│ │  formula    FormulaID = zM5K3ZMzLiBwQB93yZ4nFUsVSSgVtNPjpY72hKHxDjc9FRk9KnJSoCvkHFEPWfxARdjaguZ
│ │  formula    Exitcode = 0
│ │  formula    Time = 1662490569
│ │  formula    Results:
│ │  formula            out: tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
│ └─ formula  
│  plot  (hello-world) collected output hello-world:out
├─ plot  (hello-world) complete
│  plot  
│  plot  outputs:
│  plot         output: tar:6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA
└─ plot
```


Running Tests
-------------

All tests can be executed using:

```
make test
```

To skip tests that require network access, use the `-offline` flag:

```
go test ./... -offline
```

License
-------

SPDX-License-Identifier: Apache-2.0 OR MIT
