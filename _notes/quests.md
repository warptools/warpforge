quests for warpforge
====================

- good containment
- good interactivity and debuggability
- good integrations with content-addressable supply chains
- reproducibility strict mode (no stdin, no network, only content-addressed inputs)


implementing interactivity
--------------------------

### why

The single-command model of actions used by repeatr (and other container systems, I suppose) is flawed:
theoretically pure, but infuriating to use in practice.
Moreover, terminating the container -- and losing all shell state and variables -- is unhelpful during development and debugging.

- End users almost invariably want to run a short list of commands.
  It's better UX to offer them that explicitly, rather than leave it up to them
  to shove their work haphazardly into a shell script string concatenation.
- If we can drop to an interactive shell on error, it's very helpful for debugging.
	- Seeing the files later is one thing, but not always sufficient.
	- Environment variables present in the shell can be relevant information to inspect.
	- Being able to continue to act to inspect the situation while still having all the same shell environment
	  (not just environment variables, but any other interpreter state such as functions as well)
	  is a valuable swiss army knife. 
	- Open files, everything else: see "COMMAND EXECUTION ENVIRONMENT" in `man bash` for a long list of details.
	- Being able to interactively experience any permissions errors or filesystem mount interactions
	  firsthand can be a massive time-saver when understanding thorny situations involving them.
- Start-with-script->become-interactive is simply a great authoring flow.
  It's easy and humanely ergonomic to work by iteratively increasing the amount of actions that are scripted,
  while forging ahead playfully with interactive steps that follow those steps already known and scripted.

### how

There are two major paths that could be taken:

1. Spawn a series of containers, in sequence, atop the same filesystem.
	- Works if the only state you care about between steps of your script is the filesystem.
		- This is rarely true.  People expect to "export FOO=bar" and accumulate effects in environment, too.
	- Could we extract env vars from the zombie of the shell process after it exits?
		- Maybe.  But this is fairly complicated.
	- Could we persist any other shell state this way?  Say, "set -euo pipefail"?  Any shell functions created?  Etc?
		- No.
	- Even the working dir?
		- Maybe.  But this is fairly complicated.  I think you'd have to go looking for the tombstones from the child process and inspecting them.  This is inconvenient at best, and not even necessarily possible if some other process reaps them (and I think there are such racing reapers in play around here).
2. Run one shell process, feeding commands to it (carefully).
	- The "feed carefully" part is still tricky.  If we want per-script-step error detection and halt with reporting, we have to go one step at a time.
		- Not all shells are good at this.  For example: bash doesn't make this possible at all, as far as I can tell -- the entire [using-bash.md] file for more details on this.
	- Getting feedback about exactly what command failed is tricky.
		- For example: bash's "-x" flag (and mode, equivalently) is useless, because it mixes data plane and control plane.
			- See the entire [using-bash.md] file for more details on this and other problems.

So what're we going to do?

Option 2... plus... make our own darn shell.

It's an aggressive tactic, but it's the only one that meets the goals with any reliability.

We could also pursue spawning a new container with the same filesystem setup as a debug fallback.
This would work even when someone has a job that doesn't want to launch with our shell.
However, given the relative ease with which we've prototyped our shell approach,
we might actually shunt this into "PR's welcome" territory.
