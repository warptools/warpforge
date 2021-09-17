warpforge
=========
[![tests](https://github.com/warpfork/warpforge/actions/workflows/tests.yml/badge.svg)](https://github.com/warpfork/warpforge/actions/workflows/tests.yml)

Putting things together.  Consistently.

Install
-------

Check that you have Go 1.16+ and that `$GOPATH/bin` (usually `$HOME/go/bin`) is in your `PATH`.

To install `warpforge` to your `GOPATH`, run:
```
go install ./...
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
