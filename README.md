warpforge
=========
[![tests](https://github.com/warpfork/warpforge/actions/workflows/tests.yml/badge.svg)](https://github.com/warpfork/warpforge/actions/workflows/tests.yml)

Putting things together. Consistently.

Many of the docs and much of coordination work is in [Notion](https://www.notion.so/warpforge/Welcome-6653d3362db84ad8a2b0d2a0046748b7) --
check out the content there.
Especially check out the [API Specs](https://www.notion.so/warpforge/API-Specs-41830e3da58646d2927ef6ae5b2902e4),
because those are going to tell you a lot about _exactly_ what this tool will do for you.

Here are some terminal movies of what you can do with warpforge:

- Describing computations using filesystems named by catalogs: https://asciinema.org/a/jXl9OmTs6xlFeaeXo36BmFtjB (This is the "L2" API -- e.g. the thing that has human-readable names.)
- Describing computations using all hashes: https://asciinema.org/a/FY4iYhlEi5m0h78oFYqqvIZYc (This is the "L1" API -- it's purely content-addressable, cryptographic hashes -- no fun to write by hand, but extremely reproducible!  The "L2" API generates this for you.)


Install
-------

Check that you have Go 1.16+ and that `$GOPATH/bin` (usually `$HOME/go/bin`) is in your `PATH`.

To install `warpforge` to your `GOPATH`, run:

```
go install ./...
```

You will also need to copy the binaries in the `plugins` directory to the same path as the
`warpforge` binary.

```
cp plugins/* ~/go/bin
```

You should now be able to run the `warpforge` command:

```
$ warpforge --version
warpforge version 0.0.1
```

Running Tests
-------------

Tests require a workspace to be set up with the required binaries and wares.
For now, this can be done by running the `setup-test-workspace.sh` script.

With the workspace setup, all tests can be run with:

```
go test ./...
```

License
-------

SPDX-License-Identifier: Apache-2.0 OR MIT
