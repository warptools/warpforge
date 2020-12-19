notes on u-root
===============

- has most of coreutils implemented in pure go
- pretty heckin cool
- builds fast, works good
- emits a busybox-style all-in-one binary that you activate through symlink farm


rough incantation
-----------------

`u-root -base=/dev/null -build=bb -format=dir`

- may be worth it to whitelist emitted commands, some are... not useful in a container context, i suspect.
	- also not a high priority to minimize this.  the output from the default complete set is 12Mb.  livable.
- also has a feature for building other golang commands into the monobinary.  haven't tested this yet, but may be useful.

more completely, if you're gonna whitelist commands (and you've done dir layouts as I do):

`gof - bin/u-root -base=/dev/null -build=bb -format=dir --initcmd="" --defaultsh="" .gopath/src/github.com/u-root/u-root/cmds/core/{cat,ls}`

- another size datapoint, just for interest: the `{cat,ls}` output is 2.1Mb.


how to fit this in
------------------

- build the thing and put it in.
	- for now i'll just shove it into `/bin` as-is
	- later we'll probably want to make a normative radix-convention package out of it, and invoke a symlink farmer on that as normal.
- will i bundle any container init processes into this?  probably not.
	- don't see strong a reason to.
	- this implementation of coreutils should be a standard (not-particularly-blessed) input, when the system is more fully baked.  so any init magic shouldn't be tied to it.
	- might be trying to have the container init binary be out of sight of the final rootfs.  whereas this stuff will of course be visible in the container rootfs.


bits that sorta don't fit good (...for this exact usecase)
----------------------------------------------------------

(I don't have a lot of complaints; this is pretty cool stuff.  A couple things strike me as worth noting as I play around, though.)

### elvish

- is... just too clever for me, essentially.  it's cool, but it's too much.
	- it's being very active as i type.  recoloring things, trying to give lots of hints... it's cool but, eh.
	- i'm very nervous around overly clever interactive things and passing it through effectively and without flapping noises in containment contexts.  (even if it works, if we want to replay the logs of it, it's... noise.)
- **it's not doing $PATH in any way that i recognize**, which is a showshopper.
	- "Compilation error: variable $PATH not found" excuse me you what

will probably replace this with something based on `github.com/mvdan/sh`.

### ash

- isn't on in the default set.  maybe we should include it, though?  worth a shot?
- i'm a little unclear on what "ash" means in various contexts.  it's also a busybox shell name, but surely this one is just aiming for rough compat with that?  sharing a name and not being the thing is potentially confusing, and "potentially confusing" around basic shell things is not good.  do people have an _expectation_ that "ash" means "whatever, a shell of some kind, no promises"?

### rush

- there's *another* prototype shell in here, in the exp tree.  not even sure how i feel anymore.  experimentation is good I guess but I'll probably ignore this for now.

### pogosh

- *another* another shell.

### sha aliases

- there aren't as many of them as i might expect.  there's a 'shasum' but not 'sha256sum' or friends.  I see these used in scripts a fair amount so this is unfortunate.
	- otoh, since we're working on a project that's trying to shift the hashing out into something you rely on the toolchain to consistently do for you, surely the impact of this should be minimal.

### unshare

- it's cool that they made a small gadget for this
- it hardcodes the default shell to elvish, heh




i should file a couple issues
-----------------------------

- it's way too hard to find the format arg options.  i had to search source to find out "tar" isn't a magic word, but "dir" is.
- some of the helptext on flags talks about an initramfs more than i think is necessary.  if I *just* want the `bb` effects... the tool does that, but the helptext doesn't really lead you directly to what you need to incant (and also don't) to do that.
