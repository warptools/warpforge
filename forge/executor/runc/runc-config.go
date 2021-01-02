package runc

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	"github.com/ipld/go-ipld-prime/fluent"
	"github.com/ipld/go-ipld-prime/fluent/quip"
	"github.com/ipld/go-ipld-prime/node/basic"
	"github.com/ipld/go-ipld-prime/traversal"
)

// ExecutorSpec is something we process a Formula into.  It's a little more low-level.
// It knows what mounts are and it *does not* know what Wares are.
type ExecutorSpec struct {
	Name   string
	Cmd    []string
	Mounts []ExecutorMount
	Cwd    string
}

// ExecutorMount is the spec struct for what mounts we want.
// It's a bit lower-level than Formula instructions (it's literally just mounts;
// some of them are mounts to ware storage areas (but that's not marked at this level)),
// but also a bit higher level than a runc mount spec
// (we talk about if it should be readonly, overlay, or writethrough; that's it).
//
// For overlays, we'll make the upperdir and workdir and all that other stuff
// in a temp dir somewhere.
//
// If you want yet more detailed control of mounts, you have to write some executor specific stuff.
// We put this out of the scope of warpforge, because as a general rule, whatever that is,
// it goes well outside of anything that'll be reproducible nor easily mechanically auditable.
type ExecutorMount struct {
	Source string
	Dest   string
	Mode   int
}

const (
	MountMode_ReadOnly = 0
	MountMode_Overlay  = 1
	MountMode_Writable = 2
	MountMode_Tmp      = 3
)

type ExecutorConfig struct {
	OverlayDir string // where to put all the overlay work dirs.  May need to contain large amounts of data.  We'll make dirs with the job name and then dirs per mount in here.
}

func (cfg ExecutorConfig) Asdf(spec ExecutorSpec) {
	var err error
	n := baseConfig

	// Apply exec command.
	//  If we've been given a "script" style action, we process this into smsh shell instructions.
	//  If we're in one of the other "quick" run modes, we'll have been given a specific command (determined earlier by something closer to the CLI and the user).
	n, err = traversal.FocusedTransform(n, ipld.ParsePath("process/args"), func(_ traversal.Progress, prev ipld.Node) (ipld.Node, error) {
		return fluent.MustReflect(basicnode.Prototype.List, spec.Cmd), nil
	}, false)
	if err != nil {
		panic(err)
	}

	// Apply mounts.
	//  A lot of the interesting work is here.
	n, err = traversal.FocusedTransform(n, ipld.ParsePath("mounts"), func(_ traversal.Progress, prev ipld.Node) (n ipld.Node, err error) {
		// Mounts are very fiddly.
		// - We do *something* special at the root -- what, exactly, varies based on the major usage mode.  But generally some kind of overlayfs.
		// - Then we stack on a bunch of defaults for /dev, /tmp, /proc, and /sys.  These are normal in runc.  We do these unconditionally (many things flat out won't work without them, or will work "strangely").
		// - Then we keep adding the other user-specified mounts after that.
		//
		// *The order matters*.  To keep this from getting too wild, we do a couple things:
		// - More mounts under /dev, /proc, and /sys are just flat out forbidden.
		// - Everything else... is sorted by destination path.
		// - We clean paths before doing any of this.
		// - More than one mount targeting the same path is illegal.
		//
		// Runc will make parent dirs for mount targets automatically... as long as it can, within the rootfs.
		// This is why we generally make the root an overlayfs -- then, even if we make it readonly within the container,
		// the overlay still gives runc a place to work, in a way that might otherwise be problematic if we had mounted something right at the root.
		//
		nb := basicnode.Prototype.List.NewBuilder()
		quip.BuildList(&err, nb, -1, func(la ipld.ListAssembler) {
			// First: the rootfs itself is a special case.
			//  We want to default things to being read-only.  Runc has a flag which will do that... *as long as* you don't have a mount targeting here yourself.
			//  If we do have a mount targeting here: in many cases *must* be an overlay, even if the end result is going to be read-only, because dirs often need to be made for other mounts to land on, and those have to go somewhere!
			// Exhaustively:
			//  - If your formula asked for nothing at '/': we do nothing special with mounts, the data ends up in the runc rootfs dir, and we set readonly=true.
			//  - If your formula asked for a rw dir at '/': same as above but readonly=false.  Data ends up in the runc rootfs dir.
			//  - If your formula asked for a ware a '/': we will do an overlay mount, any dirs that are targets for other mounts ends up in the overlay layer dir (but defacto no real data will, because ro will be slapped atop this), and we set readonly=true.  (Nothing ends up in the runc idea of a rootfs dir!)
			//  - If your formula asked for a ware at '/' with rw: same as above but readonly=false.
			//  - If your formula asked for a host mount at '/': same as the story for a ware.
			//  - If your formula asked for a host mount at '/' with direct rw: first of all, this is really questioning the point of containers and you probably shouldn't do it, but if you do: okay: we actually do a mount and leave it writable and that's it.  (Beware that even if you do *nothing* in the container, runc will still have sideeffects on your host: the mkdirs for parents of mounts will still happen!)
			//  - If your formula asked for a host mount at '/' with overlay: same as the story for a ware that's rw.
			// How this relates to user-facing interfaces:
			//  - Quickrun mode hits the "host mount at '/'" case all the time: so, it pretty much always requires at least one overlay mount to work.
			//    - Although you can ask for "host mount at '/' and rw", too (in which case there's no overlay needed here).
			//  - Quickbox mode mounts its default contents in subdirectories, so it's usually "asked for nothing at '/'".
			//    - Unless your cwd is '/', in which case we get one of the "host mount at '/'" cases.
			//  - Default run and hermetic run are going to hit either "nothing at '/'" or "ware at '/'" most of the time, depending on your formula.
			//    - Default run can also have any of the "host mounts at '/'" variations.
			//    - Either default run or hermetic run can also have "rw dir at '/'".
			mnt := spec.Mounts[0]
			if mnt.Dest == "/" {
				switch mnt.Mode {
				case MountMode_ReadOnly:
					// STILL do an overlay.  It's needed so any other mount (including the essential ones) can proceed.
					//  We'll mark it readonly with a separate mechanism later.
					fallthrough
				case MountMode_Overlay:
					assembleOciMountInfo(la.AssembleValue(),
						mnt.Dest,
						"overlay",
						"none",
						[]string{
							"lowerdir=" + mnt.Source,
							"upperdir=" + cfg.DirForOverlay(spec.Name, mnt.Dest) + "/layer",
							"workdir=" + cfg.DirForOverlay(spec.Name, mnt.Dest) + "/work",
						},
					)
				case MountMode_Writable:
					assembleOciMountInfo(la.AssembleValue(),
						mnt.Dest,
						"none",
						mnt.Source,
						[]string{
							"rbind",
						},
					)
				}
			}

			// Next: all the magics.
			quip.CopyRange(&err, la, essentialMounts, 0, -1)

			// Last: The Rest.
			for _, mnt = range spec.Mounts {
				if mnt.Dest == "/" {
					continue // already did that one!
				}
				switch mnt.Mode {
				case MountMode_ReadOnly:
					assembleOciMountInfo(la.AssembleValue(),
						mnt.Dest,
						"none",
						mnt.Source,
						[]string{
							"rbind",
							"ro",
						},
					)
				case MountMode_Overlay:
					assembleOciMountInfo(la.AssembleValue(),
						mnt.Dest,
						"overlay",
						"none",
						[]string{
							"lowerdir=" + mnt.Source,
							"upperdir=" + cfg.DirForOverlay(spec.Name, mnt.Dest) + "/layer",
							"workdir=" + cfg.DirForOverlay(spec.Name, mnt.Dest) + "/work",
						},
					)
				case MountMode_Writable:
					assembleOciMountInfo(la.AssembleValue(),
						mnt.Dest,
						"none",
						mnt.Source,
						[]string{
							"rbind",
						},
					)
				}
			}
		})
		if err != nil {
			return nil, err
		}
		return nb.Build(), nil
	}, false)
	if err != nil {
		panic(err)
	}

	// Final bit of attending to filesystem setup: unless someone explicitly told us to leave the root writable... make it not.
	n, err = traversal.FocusedTransform(n, ipld.ParsePath("root/readonly"), func(_ traversal.Progress, prev ipld.Node) (ipld.Node, error) {
		if spec.Mounts[0].Dest == "/" {
			switch spec.Mounts[0].Mode {
			case MountMode_Writable:
				return basicnode.NewBool(false), nil
			default:
				return basicnode.NewBool(true), nil
			}
		}
		return basicnode.NewBool(true), nil
	}, false)
	if err != nil {
		panic(err)
	}

	// Make all those dirs.
	// todo: this is uncomfortable because it's computing stuff repeatedly.  the paths I don't mind; the conditions, I'm starting to.
	//   what if we generalize this to a bool flag per mount spec that says "more things coming underneath this"?  that would work for wares too?
	//     don't think so, though.  root is still magic because runc itself does that, and we don't have to mkdir in the layers ourselves there either.
	//       although... we could do the mkdirs inside the layer dir for the root case, too.  wouldn't hurt a thing.
	// todo: maybe this should just return a list of dirs that need making?
	//   that would separate concerns a little better, and simplify the cleanup question, as well.

	// Patch in hostname.
	// todo

	// Patch in env.
	// todo

	// Patch in cwd.
	//  Where this comes from varies by mode: for hermetic modes, it's from the formula; for quick modes, it might be determined by the CLI or simply inferred.
	n, err = traversal.FocusedTransform(n, ipld.ParsePath("process/cwd"), func(_ traversal.Progress, prev ipld.Node) (ipld.Node, error) {
		return basicnode.NewString(spec.Cwd), nil
	}, false)
	if err != nil {
		panic(err)
	}

	// Emit.  (To stderr, for the moment.  Placeholder.)
	err = dagjson.Encoder(n, os.Stderr)
	if err != nil {
		panic(err)
	}
}

func assembleOciMountInfo(na ipld.NodeAssembler, destination string, typ string, source string, options []string) (err error) {
	quip.BuildMap(&err, na, 4, func(ma ipld.MapAssembler) {
		quip.MapEntry(&err, ma, "destination", func(va ipld.NodeAssembler) {
			quip.AbsorbError(&err, va.AssignString(destination))
		})
		quip.MapEntry(&err, ma, "type", func(va ipld.NodeAssembler) {
			quip.AbsorbError(&err, va.AssignString(typ))
		})
		quip.MapEntry(&err, ma, "source", func(va ipld.NodeAssembler) {
			quip.AbsorbError(&err, va.AssignString(source))
		})
		quip.MapEntry(&err, ma, "options", func(va ipld.NodeAssembler) {
			quip.BuildList(&err, va, int64(len(options)), func(la ipld.ListAssembler) {
				for i := range options {
					quip.ListEntry(&err, la, func(va ipld.NodeAssembler) {
						quip.AbsorbError(&err, va.AssignString(options[i]))
					})
				}
			})
		})
	})
	return
}

func (cfg ExecutorConfig) DirForOverlay(jobName, mountName string) string {
	return filepath.Join(
		cfg.OverlayDir,
		url.PathEscape(jobName), // url style path escaping escapes a lot of perfectly readable unicode, which i think is a bit excessive, but it works and fits everything into one path segment safely.
		url.PathEscape(mountName),
	)
}

func mustParse(json string) ipld.Node {
	nb := basicnode.Prototype.Any.NewBuilder()
	err := dagjson.Decoder(nb, strings.NewReader(json))
	if err != nil {
		panic(err)
	}
	return nb.Build()
}

var baseConfig = mustParse(`{
	"ociVersion": "1.0.2-dev",
	"process": {
		"terminal": true,
		"user": {
			"uid": 0,
			"gid": 0
		},
		"args": ["@@args@@"],
		"env": [
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		],
		"cwd": "/",
		"capabilities": {
			"bounding":    ["CAP_AUDIT_WRITE","CAP_KILL","CAP_NET_BIND_SERVICE"],
			"effective":   ["CAP_AUDIT_WRITE","CAP_KILL","CAP_NET_BIND_SERVICE"],
			"inheritable": ["CAP_AUDIT_WRITE","CAP_KILL","CAP_NET_BIND_SERVICE"],
			"permitted":   ["CAP_AUDIT_WRITE","CAP_KILL","CAP_NET_BIND_SERVICE"],
			"ambient":     ["CAP_AUDIT_WRITE","CAP_KILL","CAP_NET_BIND_SERVICE"]
		},
		"rlimits": [
			{
				"type": "RLIMIT_NOFILE",
				"hard": 1024,
				"soft": 1024
			}
		],
		"noNewPrivileges": true
	},
	"root": {
		"path": "rootfs",
		"readonly": true
	},
	"hostname": "runc",
	"mounts": [],
	"linux": {
		"uidMappings": [
			{
				"containerID": 0,
				"hostID": 1000,
				"size": 1
			}
		],
		"gidMappings": [
			{
				"containerID": 0,
				"hostID": 1000,
				"size": 1
			}
		],
		"namespaces": [
			{"type": "pid"},
			{"type": "ipc"},
			{"type": "uts"},
			{"type": "mount"},
			{"type": "user"}
		],
		"maskedPaths": [
			"/proc/acpi",
			"/proc/asound",
			"/proc/kcore",
			"/proc/keys",
			"/proc/latency_stats",
			"/proc/timer_list",
			"/proc/timer_stats",
			"/proc/sched_debug",
			"/sys/firmware",
			"/proc/scsi"
		],
		"readonlyPaths": [
			"/proc/bus",
			"/proc/fs",
			"/proc/irq",
			"/proc/sys",
			"/proc/sysrq-trigger"
		]
	}
}`)

var essentialMounts = mustParse(`[
		{
			"destination": "/proc",
			"type": "proc",
			"source": "proc"
		},{
			"destination": "/dev",
			"type": "tmpfs",
			"source": "tmpfs",
			"options": ["nosuid","strictatime","mode=755","size=65536k"]
		},{
			"destination": "/dev/pts",
			"type": "devpts",
			"source": "devpts",
			"options": ["nosuid","noexec","newinstance","ptmxmode=0666","mode=0620"]
		},{
			"destination": "/dev/shm",
			"type": "tmpfs",
			"source": "shm",
			"options": ["nosuid","noexec","nodev","mode=1777","size=65536k"]
		},{
			"destination": "/dev/mqueue",
			"type": "mqueue",
			"source": "mqueue",
			"options": ["nosuid","noexec","nodev"]
		},{
			"destination": "/sys",
			"type": "none",
			"source": "/sys",
			"options": ["rbind","nosuid","noexec","nodev","ro"]
		}
]`)
