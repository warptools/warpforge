HACKME for the "cmd/warpforge/app" packages
===========================================

Why this package?
-----------------

### Why any package at all?

The CLI args object needs to be accessable by other packages.

- if it's stuck in one package and unexported, it's much harder to write large-scale testing.
- if it's stuck in one package and unexported, it's much harder to write other features like docs generation that uses it.

### Why not just put this in the `main` package?

We can't use the `main` package and just export it the CLI args object,
because `main` packages cannot be imported in golang, due to rules enforced by the compiler.

### Why so many sub-packages?

One subpackage per top-level subcommand.

(This is a lot of subpackages, but helps development in two ways:
for the human, it makes it easy to related groups of commands together when developing, without having to test "everything";
and for the compiler and test harness, it allows package-level parallelism.)

There's also:

- one sub-package for `base` (which everything else imports).
	- This contains the root CLI object.  (It's a global.  Other packages shovel their wiring into it when loaded.)
	- This also also contains global config stuff.  Environment vars, etc.
- one sub-package for `testutil` (which everything else imports in their tests),
	- This is mostly just for quickly wiring up testmark harnesses (because every subcommand writes tests in a similar fashion).
- and this package itself, which brings everything together (the real CLI, many tests, and the docs generation tools import this, because they all want to see everything at once).
	- There's not much here other than the imports that tie everything together.



Why are package-scoped vars used?
---------------------------------

Fair question.  Generally, that _is_ a bad code smell.

### The `urfave/cli` library forced our hand.

There's some pieces of customization in the library we use for handling the command line that are already package-scope (aka global) vars,
up in _that_ package.

Unfortunate though this is, it's kind of a dam-busting scenario.
It's hard to see a point in refraining from just doubling-down on package-scope vars once we're already stuck with more than none of them.

### Doesn't this bone test parallelization?

It totally does.  For tests that use the full CLI, anyway.

But it's not all bad news.
We still have package-level parallelization,
since golang compiles a separate binary and runs a separate process for each package's tests.
So in practice, when running `go test ./...` over the whole repo, that means things are still pretty fine.
(The use of sub-packages per command group helps a lot here, too.)



How much "business logic" is supposed to be in here?
----------------------------------------------------

Drawing the line is hard.

Here's a couple rules-of-thumb that usually help decide where code goes:

- If it refers to any `cli` packages or variables: **it goes in here**.
- If one of these packages would call into one of its neighbors: that's bad; **shared code should be factored out to `pkg/*`**.
- If the above two are contradictory: refactor until you have functions that *can* be out in `pkg/*` and are *not* refering to `cli` wiring directly.

If you *can* move some code out into `pkg/*` without too much fuss... it's probably a good target-of-opportunity.
But don't add complexity for no reason.
If that code has zero reuse, maybe it doesn't justify the creation of another package.

Not all existing code is a good example of the goalposts.



TODOs
-----

### Add serum annotations on every 'action' function.

Serum analysis in the code that's the very nearest to the user's eyeballs is the most important place!
Every one of the "action" functions that are wired into the CLI should be annotated for analysis.

A more aggressive variation of this might also be to create an interface type and use it to declare _all_ error codes valid to see returned from the CLI,
and then make all the "action" functions conform to that interface, so the analyzer makes sure each "action" is returning a subset of the total.
