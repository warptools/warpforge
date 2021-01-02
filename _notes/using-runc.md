using runc
==========

- all of the containment you wanted; none of the centralized and questionably-designed storage systems that you didn't.


unfortunate sharp edges
-----------------------

### initial process missing

The error handling for the command binary being any of:

- missing
- wrong permissions
- not executable in context (due to dynamic linking, wrong executable format, etc)

... is still atrocious:

> standard_init_linux.go:219: exec user process caused: no such file or directory

Part of this is to blame on the linux kernel itself being sketchily vague in its error codes in this vicinity.
But this is also not a very pleasing string to have to pattern match on if you want to handle it.

This can be compensated for mostly by looking for the problems ahead of time.
Technically that's a TOCTOU violation, but around containers, "it's fine", c'est la vie.


rootless
--------

... is possible.  But with oh-so-many caveats.

- You *must* have a UID 0 in the container.
	- There's a hardcoded check for this in runc when using any uid mappings.
		- I'm not sure if this is really required, or just a lazily implemented check.
- If you don't map your host UID to 0 in the container, stuff will break.
	- E.g.: `ERRO[0000] chown /run/user/{your-host-uid}/runc/{container-name}/exec.fifo: operation not permitted`
- You need a `newuidmap` and `newgidmap` command on your host.
	- _This is only the case when you have more than one UID or GID mapping range._
	- These tend to be setuid root binaries.  Which really makes questionable whether you can call this "rootless".
	- At present, the error you'll get from runc if these are missing is very unclear: something about "`nsenter: mapping tool not present`". 
	- I wonder if this requirement (the external commands; the need for them to be host-root; ideally both) could be avoided with more development effort in runc.  At the time of writing this note, I'm not familiar enough to be sure.
- Due to the conjunction of the above... if you want any non-zero UIDs to be possible in the container... you'll need those `newuidmap` commands.
	- Yes: even if you wanted *only* uid 1000 in the container: you still have to have a uid 0, and that means in total you need at least two uids, and that means this whole set of dependencies comes crashing down on you.
- There must be entries in the `/etc/sub{uid,gid}` files for your user, and their magic numbers must align with those in the runc config.
	- _This is only the case when you want to map a UID or GID that is not your own._  (So, again, when you want more than one uid or gid to be available within the container, or any non-zero uid/gid.)
	- It seems many linux distros will automatically fill out these files with some default ranges for human users, nowaday.  But if they don't: this is another place you'll need root (one-time only, though, fortunately) to get "rootless" mode working.
	- The "magic numbers must align" thing significantly complicates trying to make this a no-input-needed-DTRT system.
		- Bonus fun: these files use user *name* (not uid), so we can't even look up the magic number ranges without reimplementing an /etc/passwd parser and the name lookup stuff.
			- No, I'm not even willing to discuss the other various ways that nsswitch can work.

... so, it seems that if we want maximally portable zero-config "rootless" operations,
we should do that by always having commands in the container launched as uid=0,
and make no other uids and gids possible in the container at all.

Additionally:

- bind mounts are possible during container setup without additional privileges on "most" linux distros by default now (afaik) -- but it is a configuration flag.
	- this configuration flag is now one you can adjust by pushing magic numbers into the sys psuedofs, iiuc, but it's another thing that make require sudo to set up the first time.
- overlay mounts -- which are also extremely critical for practical usage -- are possible on *some* distros (namely, Ubuntu) but this is not something that can be assumed to work everywhere.
	- this configuration flag is one at kernel compile time, iiuc, which makes it extremely nontrivial to ask an end user to do.

These last two issues aren't things that can be easily avoided by changing or limiting the scope of the workload.

There are overlay implementations in fuse, I understand, but this adds more dependencies, and also has a performance overhead.
Perhaps we can use that when necessary; hopefully it's fairly drop-in and transparent in the occasions when its needed.

### user namespaces

... are disabled by default on most distros.  Which means no joy for rootless containers, full stop.
First let's talk about detecting that; they talk about fixing it.

Many host environments will have an `unshare` command which we can use to quickly test this:
Try running `unshare --user sh`.
If that errors with something about "operation not permitted",
then your kernel doesn't have user namespaces enabled, and we won't be able to run rootless containers.

There are several ways to address this, but they may vary based on your distro/kernel/etc, so these are things you can try:

- `sudo sysctl -w kernel.unprivileged_userns_clone=1`
- ?
- Try this forum for more information and options:
  https://superuser.com/questions/1094597/enable-user-namespaces-in-debian-kernel
