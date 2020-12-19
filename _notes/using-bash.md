notes on bash
=============

... specifically, how to use it as a job manager.

Or how not to, as the case may be.

desires
-------

- I want to feed a sequence of script actions.
- I want to know which of those, if any, failed.
- And not continue executing subsequent ones.
- I'd love to be able to get an interactive mode at that time.

can we do it
------------

tl;dr no

### can we feed commands in stepwise

As far as I can tell... no.

- There are roughly two ways to get commands into bash: feeding them into stdin, or, concatenating them with linebreaks and/or ";" and handing them as an arg to "-c".
	- When feeding them into stdin, there's no feedback nor backpressure mechanism.  Just have to push the whole thing and hope.
	- When concatenating them with ";" and handing off one blob to "-c", of course there's no feedback nor backpressure mechanism.
- You can get a couple extra separated steps by working with `--init-file` and `--rcfile`, but neither of these produces feedback nor backpressure mechanisms either.

Without a way to control input and output flow 

### can we get stepwise info out with traps

As far as I can tell... no.

- trap on RETURN doesn't happen often enough.
- trap on ERR doesn't happen often enough.
- trap on DEBUG happens too often, and at the wrong time (before each command).
	- maybe if you could have this stream out control information on a magic fd, this would be possible to accomplish _something_ with.
	- doesn't have exit codes and can't get them.  maybe in combination with other traps, _something_ could be done.
	- try it: `bash -Tc "function bark { echo :: $@;}  ; trap bark DEBUG; echo hello ; uh ; wow ; "`
- all of these are overridable, which makes them a bit fragile.
- you might have to dance with `set -o functrace` aka `-T` to get many of these traps to persist in all places.
	- which may also affect any user scripts that set them.  no compartmentalization here.
- most of these don't actually receive much information, either.

### can we get stepwise info out with '-x' mode

As far as I can tell... no.  Not reliably.

- it's a mode that's easily unset in scripts.  Fragile.
- it's something that users sometimes set intentionally and want to see (and again, it's ambiguous to us _when_ since that can be done _in_ a script), so we certainly can't slurp and hide this data.
- it mangles the data and control plane, so it's a fundamentally broken system.  (Try `bash -xc 'echo + uh oh'` and you'll see what I mean.)

### can we make clever compositions

- Solving for input backpressure: we could try to use strace from the outside.  We could see child processes exiting that way, and insert the next step at that time.
	- This seems like really getting into the deep end though, doesn't it?
	- Nope: this tactic only works if every step of your script is exactly one subprocess.  This is an invalid assumption: much scripting isn't.  Loops; invocations of other builtins like export; etc.

### haven't other people done this???

Yes, I've seen some CI platforms pull it off.

It appears to work by injecting a ton of preamble script and parsing things in shady ways that continue to suffer from mixture of control plane and data plane.

Predictably, it goes very, very badly when you "set -x".

Or at least it did the last time I encountered one of these and poked at its edges.

If someone out there has a solution that addresses all of these things *reliably*, I'd love to hear about it.


what's the alternative
----------------------

I think we'll be implementing a small shell with a command-and-control mode specifically to support these features.
