CLI Help Output
===============

This document shows output of help for various commands.
This allows us to see and check diffs for changes to our CLI configuration.


[testmark]:# (ware_unpack/sequence)
```
warpforge ware unpack --help
```

[testmark]:# (ware_unpack/output)
```
NAME:
   warpforge ware unpack - Places the contents of a ware onto the filesystem

USAGE:
   warpforge ware unpack [command options] [WareID]

DESCRIPTION:
   [WareID]: a ware ID such as [packtype]:[hash]. e.g. "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"

OPTIONS:
   --path value  Location to place the ware contents. Defaults to current directory.
   --force       Allow overwriting the given path. Any contents of an existing directory may be lost. (default: false)
   --help, -h    show help
   
```
