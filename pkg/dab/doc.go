/*
	Package dab -- short for Data Access Broker -- contains functions that help save and load data,
	mostly to a local filesystem (but sometimes to a blind content-addressed objectstore, as well).

	Most dab functions return objects from the wfapi package.
	Some return a dab type, in which case that object is to help manage further access --
	but eventually you should still reach wfapi data types.

	Functions that deal with the filesystem may expect to be dealing with either
	a workspace filesystem (e.g., conmingled with other user files),
	or a catalog filesystem projection (a somewhat stricter situation).
	Sometimes these are the same.
	The function name should provide a hint about which situations it handles.

	Sometimes, search features are provided for workspace filesystems,
	since there is no other index of those contents aside from the filesystem itself.

	Most of these functions return the "latest" version of their relevant API type.
	At the moment, that's not saying much, because we haven't grown in such a way
	that we support major varations of API object reversions -- but in the future,
	this means these functions may do "migrational" transforms to the data on the fly.
*/
package dab
