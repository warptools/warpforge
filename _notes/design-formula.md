designing formulas
==================

Formulas are one of the main API surfaces of warpforge.

:::info
Seealso the "docs-formulas.md" file, which is a little more declarative about where we're at.

This file is notes about the thinking behind the scenes and contains considerations moreso than decisions.
:::

- Formulas describe inputs to a process, the process, and the outputs to collect from the process.
- Formulas are designed primarily to describe hermetic processes...
	- but also have a couple points where they can also describe non-hermetic data interactions (mainly for debugging and development convenience).
		- These are small, enumerable, and any parts of the tool that evaluate formulas will flag them and halt unless they're explicitly allowed by the user.

generally
---------

### consent

Generally, formulas are meant to be a file you can put in a repo somewhere,
and reasonably ask people to evaluate it using warpforge,
and they should be able to do so without fear
(as long as they trust the containment mechanism not to break, and as long as they're not worried about resource budgets).

Any of the statements that can be put into a formula file that might describe exposing something from the host
(e.g., any host mounts, any request for network permissions, etc) are more like "asks":
the formula can say it wants them, and if we don't give a container those things we can't expect it to work,
but we also won't actually *give* it those things by default: that choice ultimately needs to be raised to the user.


### posixishness

Formulas happen to describe something that looks a lot like a heap of posix(ish) filesystems and posix(ish) processes.

That *doesn't* mean we're interested in proxying every single possible option expressable in those systems up to the user, though.
(If we were, we wouldn't be a tool at all.)

Broadly, we're aiming to:

- Be able to describe enough stuff to make a "workable" environment.
	- This is entirely a question of practicalities, and a pareto principle will apply: we're going for "most" workloads; not "all imaginable" workloads.
	- We're mostly interested in computational workloads (such as doing data analysis, or building software).
	- We're relatively less interested in hosting workloads, but, they should work as much as possible in a "best effort" sense.
		- It's also tenable for these kind of workloads to work conditionally on more host integration demands (e.g., I'm more concerned that someone should be able to run a build-a-foobar demo with minimal host setup than I'm concerned about their ability to run an application with an X window connection or something esoteric like that in zero config way).
- We're *not* interested in any of the stuff relating to the concept of good sysadmin principles or security that you might be familiar with from a posixish world.
	- We're living in a container world, here.  The rules are different.  We don't need uids and gids for isolation: we use the containers for that.


in and out
----------

### inputs

- ware
	- "ware:{wareKind}:{hash}"
	- this should be the bread and butter: these are the hermetic things.
	- //review: should this actually be "ware:(ro|rw):{wareKind}:{hash}"?
		- one of the few things I think we might've done wrong in the last generation of repeatr is rw overlays by default.  I'd rather make ro the default.
			- yes, there are plenty of projects (especially old C things) that will whimsically attempt to write in their source dir.  I still think that should be discouraged and require a terse acknowledgement.
		- if we shove it in one linear stringjoin like this, then it'll require "ro" or "rw" explicitly.  no default or implication; that makes parsing too irregular.
		- if we put this in a map, *then* we could do implication and make "ro" the default.
		- possible concern with doing ro mode at all: then putting more inputs under that path becomes problematic again (can't mkdir).
			- we could address this with More Overlays, but this is getting hectic.  we'd have to compute needed dirs ahead of time and make them in the overlay layer dir from the outside.
- dir literals
	- "dir:(ro|rw)"
		- optional: posix bits // why would anyone want these if we're stuck with one uid+gid ?  even if we have several, in a container environment,... who cares?
	- technically also hermetic
	- this could be something where someone wants to express a tmpfs preference... but, I think that should probably be adjunct config somehow; how much one is willing to spend RAM for speed is a host choice, not a correctness thing.
- file literals
	- "file:{literal text}"
		- optional: posix bits // why would anyone want these if we're stuck with one uid+gid ?  even if we have several, in a container environment,... who cares?
		- //review: this would be a *very* good case for a map as the value instead of a string; we don't really want to parse these literals.  and i'd like to be able to use a multiline heredoc for them, in formats that allow it.
	- technically also hermetic
	- will need a size limit of some kind.  10kb?
	- can this be (or ever need to be) readonly like any other mount?  i suppose it can; whether it should, idk.
- mounts
	- "mount:(ro|rw|direct):{hostPath}"
	- **not** hermetic
	- note that "rw" still means an overlay.  "direct" means writable straight to host.

(Almost) all of the above may also want to include "filters", which could handle posix rwx bits and ownership and additional bits like "sticky", etc.
(Mounts are probably an exception, because they can't trivially support such filters.)

#### complex objects or simple strings?

The major design question here is: can we cram all these inputs into simple strings (using lots of stringprefix unions and stringjoin structs)?
Or should we make all of them full maps/structs right from the start?

The main reason to worry about this would be out of interest for terse formulas and easy writing by hand.
It's unclear if this is a major concern, though.  Ideally, users don't write formulas by hand.  So optimizing for their terseness is foolish.
On the other hand: "ideally" is doing a lot of work there; we don't have a ton of higher level templating helper systems ready to go and shipped with a bow on them.

The second reason to consider this is that it affects how we design higher-level systems that pass information around between graphs of formula outlines (sometimes aka "modules").
Single strings are easier and clearer to pass around than objects.
(There's a lot of room for nuance and argument in this vicinity, though.)

Notably, IPLD Schemas can give us a lot of nicely described features for stringprefix unions and etc.
But one thing it doesn't give is: the ability to have a string-or-map based on if all-but-one fields are absent/implicit.
So, this could result in something of a normalization issue.

Something we could do is: do a kindred union between SimpleInput and DetailedInput.
The latter is a struct with map repr and has one field that's just SimpleInput again... and the rest of its fields can handle filters, mount modes, etc.
This would not the most graceful to process... and it doesn't normalize itself if none of the optional fields are present!
But it would be survivable to do that small amount of normalization as application logic.
(And I think it may turn out that most of the optional fields we might put in DetailedInput would turn out the same for almost all inputs; enough so to call it a day, schematically.)

On the higher-level perspective, a major thing to note on this is:
when passing references to wares around at L3, you tend to pass around wareIDs... and **not** any of this other fiddly stuff,
or especially whether an input is read-only versus read+write (because that's the consuming formula's problem|choice!).
This is probably a pretty strong argument in favor of a complex object for inputs, wherein only *part* of that object is subject to substitution by an L3 tool.

But okay: resolution on this for now then:
- do the larger structure now
- the joy of a kinded union like this is you can *always* stick in the more compact representation later and it's not a breaking change.  So we can defer a lot of this decision.

### outputs

- ware
	- "ware:{wareKind}:{pathToPack}"
	- 99.9% of the time.
	- since it's handling files, could contain filters for posix attributes, like discussed for inputs.
- literals
	- could come in the form of "read this file and make the content string itself the output value"
	- ...? is there any use to this?
		- short messages that you might actually want to see or print is my main thought.
		- things you could feed into env vars in later processes?  questionable if this is something we'd want to encourage, though.
- exitCode
	- ...? is there any use to this?
		- if you're building a pipeline that wants to build reports on its own possibly-partial successes, you should have it make report files and pass them around.
			- you'll have a better time with this in general, because there's no control-plane-and-data-plane-mixing problems, as exit codes *always* fall prey to due to their limited range.
			- use a non-zero exit from your container only for cataclysmic failings that mean future computations shouldn't even try.  think of it like panicking vs error returns.
- env vars
	- ...? is there much practical use to this?
- objects
	- if we hypergeneralized this computation concept enough, we could say just full on IPLD objects are a valid output.
	- that's getting awfully, well, hypergeneralized, though.

#### complex objects or simple strings?

This is a similar question to the one for inputs.

Its unclear if the answers need to be the same.
It's likely they should be at least somewhat symmetrical, considering that L3 systems will be passing around values between the two.


### compared to modules

Modules can describe all the same stuff, but to turn an input into an import,
they become one level more wrapped, and imports have other options as well:

- the prefix "literal:" can be applied to anything already usable as an input.
- the prefix "catalog:" can be used to do catalog lookups (a reasonably replay-friendly system).
- the prefix "ingest:" can be used to scrape info live from the host system (not at all replay-friendly).


action
------

Action needs a union in it somewhere.

Question is, where, what are its members,
and how much content is moved inside the members vs can stay constant outside the union.

### members of the action union

- "exec"
	- a list of strings which will head directly into an exec syscall.
	- simple and direct.
	- also only thing previously supported.
	- tends to degenerate to `["/bin/bash", "-c", "oneverylonglonglongstring"]` and thus be a terribly painful UX in practice.
- "script"
	- a list of strings which are scriptlets that will be fed one at a time to our (custom) shell.
		- nature of these as scriptlets fixes the `"oneverylonglonglongstring"` UX problem that the exec form has.
	- means that we need our custom shell process in there.
		- maybe an option should be available for overriding what this shell is called.  (still assuming a smsh-like API, though.)
- ...other?
	- We probably want to leave the door open here to future extensions.

### where are we homing concepts like env vars?

Any kind of action that's looking vaguely like a posix process in a container
is going to have concepts like env vars and current working directories.
Right now, we're even throwing things like _hostname_ in there.

Even more exotic things like network mode (e.g. denied or transparent) are currently
stuffed in as properties of `action`.

And how about `uid` and `gid`?

Do these things belong there?  Or should they be fields one step deeper, inside the union members they apply to?

At the moment, they'd apply to every union member we have
(since they're all variations on ways to launch posixish processes).
But this may not always be the case.
