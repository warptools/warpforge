package main

import (
	"os"

	"github.com/warpfork/warpforge/forge/executor/runc"
)

/*
	At least three major modes of operation:

	- default run:
		- takes a formula and executes it in a clean room.
		- if the action is a kind that supports it (e.g. "script" mode), will drop into interactive mode when any contained commands exit in error.
		- cleanroom start, but does accept inputs that are non-reproducible (host mounts, live ingests, etc)
		- will happily supply default values for the rootfs if you didn't provide one.

	- hermetic run:
		- takes a formula and executes it in a clean room.
		- mostly identical to default run mode... it's just going to halt more.  More suitable for headless use.
		- will *not* provide default values for the rootfs or anything else: halts instead.
		- will *not* drop into interactive debug mode, ever: halts instead.
		- will *not* evaluate a formula where any of the inputs are non-reproducible.
			- can still permit some of these things but only with explicit flags (e.g. network (not technically an input, anyway)); default no-args use is all strict mode.

	- quickrun:
		- basically supposed to let you run things on your host that you have questions about the sanitization of.  for quick science, not production.  like a "dry-run" condom for any program.
		- turns the args after "--" into the command in the container
		- interactive mode fully on
		- mounts your whole host fs... readonly
		- copies your current $PATH from the host environment
		- keeps your uid the same inside the container
		- network on by default (no namespace at all)
		- remounts your working directory read-write (only this)
			- or maybe we do an overlayfs on this by default.  "fuck around and find out" followed rapidly by "cancel" is good.
		- makes a ramdisk for /tmp
		- flags for creating a temporary user home dir that's writable

	- quickbox:
		- the interactivity of quickrun, but with the clean room setup of default run.
		- cwd initialized to `/quicktask` which is a ramdisk.
		- host mounted read-only in `/host` by default.
		- original host cwd mounted writeable in `/hostcwd` by default.
			- or maybe we do an overlayfs on this by default.  "fuck around and find out" followed rapidly by "cancel" is good.

	interesting to note that we might make use of default run mode and a bunch of impure imports for bootstrapping purposes.
	the real hermetic release build will then simply be the same, but re-templated with inputs values to be content-addressed stuff.
	very elegant; and the use of mounts in the bootstrap will make development much more rapid as well as bootstrap more maintainable.
*/

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	runc.ExecutorConfig{
		OverlayDir: "/tmp/overlay",
	}.Asdf(runc.ExecutorSpec{
		Name: "fwee",
		Cmd:  []string{"/bin/bash"},
		// The following is angling in the direction of "quickrun":
		Mounts: []runc.ExecutorMount{
			{
				Dest:   "/",
				Source: "/",
				Mode:   runc.MountMode_ReadOnly,
			},
			{
				Dest: "/tmp",
				Mode: runc.MountMode_Tmp,
			},
			{
				Dest:   cwd,
				Source: cwd,
				Mode:   runc.MountMode_Overlay,
			},
		},
		Cwd: cwd,
	})
}
