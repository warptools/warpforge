/*
	fscomp is a package of tools for filesystem composition.

	It uses a series of bind and overlayfs mounts to create composed filesystems quickly,
	as well as without modifying the source filesystems.

	This code makes it possible to do things like:
	"mount the host filesystem at what will be '/' in a container,
	and also mount something at '/foo'...
	without creating a 'foo' directory on the host filesystem",
	and do this with only those two instructions.
	To make such a thing real, many individual instructions to kernel are required
	(this example requires making several directories, making overlay filesystems,
	then bind mounting several things together, etc -- and doing all this in the right order),
	so using this package can save you a lot of work.

	This is not the package for handling packing and unpacking; it's just the mounts.
*/
package fscomp

import (
	"fmt"
	"os"
	"sort"
	"syscall"

	securejoin "github.com/cyphar/filepath-securejoin"
)

func main() {
	// This should roughly work, though it requires a fair number of mkdirs before it will.
	//  It'll also require root.  Even, apparently, on a kernel that I thought was configured to chill about user bind mounts.
	decomposeFn, err := Compose("/tmp/out",
		CompEntry{SrcPath: "/tmp/a", DstPath: "/a"},
		CompEntry{SrcPath: "/tmp/b", DstPath: "/b"},
	)
	if err != nil {
		panic(err)
	}
	if err := decomposeFn(); err != nil {
		panic(err)
	}
}

type CompEntry struct {
	SrcPath string // absolute path from anywhere on your host.
	DstPath string // target path, which should also be expressed as absolute, but will be relativized to the overall destination.
	Mode    CompMode
}

// CompMode describes the effect we want for the destination filesystem.
type CompMode uint8

const (
	CompMode_Invalid  CompMode = iota
	CompMode_ReadOnly          // mind, you still might end up needing the CompCfg.OverlayDir -- if another mount goes *within* this one's path.
	CompMode_Overlay           // result is writable, and source is effectively readonly; changes are accrued in other directories.
	CompMode_Writable          // mount straight through.
)

type CompCfg struct {
	OverlayDir string
}

func Compose(dstPath string, entries ...CompEntry) (Decomposer, error) {
	sort.Sort(compEntryByDstPath(entries))
	var dec Decomposer

	for _, entry := range entries {
		decFn, err := mountBind(entry.SrcPath, entry.DstPath, dstPath, true)
		dec = combineDecomposers(dec, decFn)
		if err != nil {
			return dec, err
		}
	}
	return dec, nil
}

type Decomposer func() error

// compEntryByDstPath is for sorting by destination path, which is effectively mount order.
type compEntryByDstPath []CompEntry

func (a compEntryByDstPath) Len() int           { return len(a) }
func (a compEntryByDstPath) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a compEntryByDstPath) Less(i, j int) bool { return a[i].DstPath < a[j].DstPath }

// mountBind does the work of creating a bind mount.
// If directories or files need to be created at the destination, this function will do so -- but it does not create parents.
// dstRoot is where we expect a chroot in the future -- so we won't allow any part of dst to refer out of that by symlink.
func mountBind(src, dst, dstRoot string, readonly bool) (Decomposer, error) {
	// src must exist.
	srcStat, err := os.Lstat(src)
	if err != nil {
		return nil, fmt.Errorf("fscomp: cannot bind mount with src=%q: %w", src, err)
	}

	// Resolve dst into the absolute thing.
	// The resolved destination must exist... and not be trying to break out.
	dstAbs, err := securejoin.SecureJoin(dstRoot, dst)
	if err != nil {
		return nil, fmt.Errorf("fscomp: cannot bind mount with dst=%q in dstRoot=%q: %w", dst, dstRoot, err)
	}

	// It is interesting to node that runc also checks for mounts that peek into proc at about this stage in their mounting code.
	//  We don't need to do this, because we're preparing a rootfs filesystem over which we expect the container system will apply its mounts (such as proc) *atop*.

	// Make the destination path exist and be the right type to mount over.
	if err := mkDest(dstAbs, srcStat.IsDir()); err != nil {
		return nil, err
	}

	// Make mount syscall to bind, and optionally then push it to readonly.
	//  Works the same for dirs or files.
	flags := syscall.MS_BIND | syscall.MS_REC
	if err := syscall.Mount(src, dstAbs, "bind", uintptr(flags), ""); err != nil {
		return nil, fmt.Errorf("fscomp: error creating bind mount with src=%q, dst=%q, dstRoot=%q: %w", src, dst, dstRoot, err)
	}
	if readonly {
		flags |= syscall.MS_RDONLY | syscall.MS_REMOUNT
		if err := syscall.Mount(src, dstAbs, "bind", uintptr(flags), ""); err != nil {
			return unmountJanitor(dstAbs), fmt.Errorf("fscomp: error creating bind mount (step 2) with src=%q, dst=%q, dstRoot=%q: %w", src, dst, dstRoot, err)
		}
	}
	return unmountJanitor(dstAbs), nil
}

func mkDest(pth string, dir bool) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("fscomp: cannot prepare mount destination %q: %w", pth, err)
		}
	}()
	_, err = os.Lstat(pth)
	switch {
	case err == nil:
		return nil
	case os.IsNotExist(err):
		// carry on, we'll create it
	default:
		return err
	}
	if dir {
		return os.Mkdir(pth, 0644)
	} else {
		var f *os.File
		f, err = os.OpenFile(pth, os.O_CREATE, 0644)
		f.Close()
		return err
	}
}

func combineDecomposers(fns ...Decomposer) Decomposer {
	return func() error {
		for _, fn := range fns {
			if fn == nil {
				continue
			}
			if err := fn(); err != nil {
				return err
			}
		}
		return nil
	}
}

func unmountJanitor(pth string) Decomposer {
	return func() error {
		if err := syscall.Unmount(pth, 0); err != nil {
			return fmt.Errorf("fscomp: error unmounting %q: %w", pth, err)
		}
		return nil
	}
}
