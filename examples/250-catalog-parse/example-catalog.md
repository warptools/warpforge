Example Catalogs
================

Okay -- no examples here.  You'll have to deal with the live, real content instead.

Check out https://github.com/warpsys/catalog !

See the [Catalogs page on Notion](https://warpforge.notion.site/Catalogs-f53f2c9a2f0b4a8ba4d55d5f52997555)
for a primer on what Catalogs are, what they're for, etc.

Key highlights you can expect to see in a catalog filesystem:

- the directory structure matches the module name -- which is the same name you'll use to import it.
- the `module.json` files identify what releases there are for a thing;
- the `mirrors.json` files right next to it contain a bunch of URLs (warehouse addresses, to be specific) where we expect to be able to get the bulk data;
- the `releases/*` directory contains more json files...
- one file for each release version...
- ... and each one contains one or more WareID (the hash) of released data, each of which can be named.
- There may also be a `replays/*` directory!
- If so, the `replays/*` files contain snapshots of the plots that were used to build the releases.
  (You can, well, "replay" them -- which should reproduce the released content!)

---

TODO :) We should probably still have inline and focused examples here.

// We'll probably want to use testmark's "directory" feature heavily in this one, because there will be multiple files to form a complete catalog.
