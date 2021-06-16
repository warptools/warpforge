Subsystems of Warpforge
=======================

:::info
This is a mostly internal document about code architecture.
Some of its effects may also map to the user experience, but it's not written to be an end-user guide.
:::

### executor

Responsibilities:

- meant to do one job (which usually means: container exec) at a time.
- does *not* have responsibility for data transfer
	- expects the ware for of its inputs to already be in the local warehouse -- is free to error if not there.
	- should unpack wares to cache as necessary, and should pack outputs to warehouse.
		- debatable.  We could break this out further.  Seems simple enough to say this for now though.
		- mostly this involves shelling out to the packer.
	- just generally, should feel like an "offline" process.  Execution time of this subsystem should never, ever vary based on your network latency.
	- practical meaning?  The arguments to this subsystem don't even include the "formula context".  No URLs needed.  That should've already been handled before this subsystem is invoked.

### packer

Responsibilities:

- map packed wares into unpacked filesets, and vice versa.

Implementation notes:

- the current implementation is wrapping the `rio` process (which is still implemented in golang).
- the containerization of this could've been done several different ways:
	- options:
		- do rio bare on host, and launch just the job in a container
		- do rio in a container, and launch the job in a sibling container
		- do a wrapper container, do rio inside it, and launch the job in a deeper container
	- the choice made: option 2.
		- critical reason: to get consistent container uid mapping, if you're using userspace containers and uidmapping.
			- Which is exceptionally important, because that mode is our new default.
		- bonus: arguably, slightly more "security", because the rio process can't accidentally work outside of the warehouse and cache areas given to its container.  (The rio process isn't generally considered untrusted anyway, but defense in depth doesn't hurt.)
