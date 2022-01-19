package formulaexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/printer"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

const LOG_TAG_START = "│ ┌─ formula"
const LOG_TAG = "│ │  formula"
const LOG_TAG_OUTPUT_START = "│ │ ┌─ output "
const LOG_TAG_OUTPUT = "│ │ │  output "
const LOG_TAG_OUTPUT_END = "│ │ └─ output "
const LOG_TAG_END = "│ └─ formula"

type RioResult struct {
	WareId string `json:"wareID"`
}
type RioOutput struct {
	Result RioResult `json:"result"`
}

type runConfig struct {
	spec    specs.Spec
	runPath string // path used to store temporary files used for formula run
	wsPath  string // path to of the workspace to run in
	binPath string // path containing required binaries to run (rio, runc)
}

// base directory for warpforge related files within the container
const CONTAINER_BASE_PATH string = "/.warpforge.container"

// bin directory within the container, provides `rio`
func containerBinPath() string {
	return filepath.Join(CONTAINER_BASE_PATH, "bin")
}

// home workspace directory location within the container
func containerWorkspacePath() string {
	return filepath.Join(CONTAINER_BASE_PATH, "workspace")
}

// rio warehouse directory location within the container
func containerWarehousePath() string {
	return filepath.Join(containerWorkspacePath(), "warehouse")
}

// rio cache directory location within the container
func containerCachePath() string {
	return filepath.Join(containerWorkspacePath(), "cache")
}

// base directory for `script` Action files within the container
func containerScriptPath() string {
	return filepath.Join(CONTAINER_BASE_PATH, "script")
}

func getNetworkMounts(wsPath string) []specs.Mount {
	// some operations require network access, which requires some configuration
	// we provide a resolv.conf for DNS configuration and /etc/ssl/certs
	// for trusted CAs from the host system
	etcMount := specs.Mount{
		Source:      "/etc/resolv.conf",
		Destination: "/etc/resolv.conf",
		Type:        "none",
		Options:     []string{"rbind"},
	}
	caMount := specs.Mount{
		Source:      "/etc/ssl/certs",
		Destination: "/etc/ssl/certs",
		Type:        "none",
		Options:     []string{"rbind", "ro"},
	}
	return []specs.Mount{etcMount, caMount}
}

// Creates a base configuration for runc, which is later modified before running``
//
// Errors:
//
//    - warpforge-error-io -- when file reads, writes, and dir creation fails
//    - warpforge-error-executor-failed -- when generation of the base spec by invoking runc fails
//    - warpforge-error-serialization -- when deserialization of the runc generated config fails
func getBaseConfig(wsPath, runPath, binPath string) (runConfig, error) {
	rc := runConfig{
		runPath: runPath,
		wsPath:  wsPath,
		binPath: binPath,
	}

	// generate a runc rootless config, then read the resulting config
	configFile := filepath.Join(runPath, "config.json")
	err := os.RemoveAll(configFile)
	if err != nil {
		return rc, wfapi.ErrorIo("failed to remove config.json", err)
	}
	var cmd *exec.Cmd
	if os.Getuid() == 0 {
		cmd = exec.Command(filepath.Join(binPath, "runc"),
			"spec",
			"-b", runPath)
	} else {
		cmd = exec.Command(filepath.Join(binPath, "runc"),
			"spec",
			"--rootless",
			"-b", runPath)
	}
	err = cmd.Run()
	if err != nil {
		return rc, wfapi.ErrorExecutorFailed("failed to generate runc config", err)
	}
	configFileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return rc, wfapi.ErrorIo("failed to read runc config", err)
	}
	err = json.Unmarshal(configFileBytes, &rc.spec)
	if err != nil {
		return rc, wfapi.ErrorSerialization("failed to parse runc config", err)
	}

	// set up root -- this is not actually used since it is replaced with an overlayfs
	rootPath := filepath.Join(runPath, "root")
	err = os.MkdirAll(rootPath, 0755)
	if err != nil {
		return rc, wfapi.ErrorIo("failed to create root directory", err)
	}

	root := specs.Root{
		Path: rootPath,
		// if root is readonly, the overlayfs mount at '/' will also be readonly
		// force as rw for now
		Readonly: false,
	}
	rc.spec.Root = &root

	// mount warpforge directories into the container
	// TODO: only needed for pack/unpack, but currently applied to all containers
	wfMount := specs.Mount{
		Source:      wsPath,
		Destination: containerWorkspacePath(),
		Type:        "none",
		Options:     []string{"rbind"},
	}
	rc.spec.Mounts = append(rc.spec.Mounts, wfMount)
	wfBinMount := specs.Mount{
		Source:      binPath,
		Destination: containerBinPath(),
		Type:        "none",
		Options:     []string{"rbind", "ro"},
	}
	rc.spec.Mounts = append(rc.spec.Mounts, wfBinMount)

	// required for executing on systems without a tty (e.g., github actions)
	rc.spec.Process.Terminal = false

	// the rootless spec will omit the "network" namespace by default, and the
	// rootful config will have an empty namespace.
	// normalize to no network namespace in the base configuration.
	newNamespaces := []specs.LinuxNamespace{}
	for _, ns := range rc.spec.Linux.Namespaces {
		if ns.Type != "network" {
			newNamespaces = append(newNamespaces, ns)
		}
	}
	rc.spec.Linux.Namespaces = newNamespaces

	return rc, nil
}

// Creates a mount for a ware
// This function performs several steps to create and configure a ware mount
//   1. Determine which warehouse to use to fetch the ware
//   2. Configure a runc execution for unpacking the mount
//   3. Invoke `rio unpack` using runc to unpack the ware
//   4. Use result of `rio unpack` to create a ware mount for execution
//
// This function also checks to see if a cached ware exists in the warehouse.
// If so, no unpack operation is performed.
//
// Errors:
//
//     - warpforge-error-io -- when IO error occurs during setup
//     - warpforge-error-executor-failed -- when runc execution of `rio unpack` fails
//     - warpforge-error-serialization -- when rio output cannot be deserialized
//     - warpforge-error-ware-unpack -- when rio output cannot be deserialized
func makeWareMount(config runConfig,
	wareId wfapi.WareID,
	dest string,
	context *wfapi.FormulaContext,
	filters wfapi.FilterMap,
	logger logging.Logger) (specs.Mount, error) {
	// default warehouse to unpack from
	src := "ca+file://" + containerWarehousePath()

	// check to see if this ware should be fetched from a different warehouse
	for k, v := range context.Warehouses.Values {
		if k.String() == wareId.String() {
			wareAddr := string(v)

			// check if we need to create a mount for this warehouse
			proto := strings.Split(wareAddr, ":")[0]
			hostPath := strings.Split(wareAddr, "://")[1]
			if proto == "file" || proto == "file+ca" {
				// this is a local file or directory, we will need to mount it to the container for unpacking
				// we will mount it at CONTAINER_BASE_PATH/tmp
				src = filepath.Join(CONTAINER_BASE_PATH, "tmp")
				mnt, err := makeBindPathMount(config, hostPath, src)
				if err != nil {
					return mnt, err
				}
				config.spec.Mounts = append(config.spec.Mounts, mnt)

				// finally, add the protocol back on to the src string for rio
				src = fmt.Sprintf("%s://%s", proto, src)
			} else {
				// this is a network address, pass it to rio as is
				src = string(v)
			}
		}
	}

	// unpacking may require fetching from a remote source, which may
	// require network access. since we do this in an empty container,
	// we need a resolv.conf for DNS configuration and /etc/ssl/certs
	// for trusted CAs
	config.spec.Mounts = append(config.spec.Mounts, getNetworkMounts(config.wsPath)...)

	// convert FilterMap to rio string
	var filterStr string
	for name, value := range filters.Values {
		filterStr = fmt.Sprintf(",%s%s=%s", filterStr, name, value)
	}

	// perform a rio unpack with no placer. this will unpack the contents
	// to the RIO_CACHE dir and stop. we will then overlay mount the cache
	// dir when executing the formula.
	config.spec.Process.Env = []string{"RIO_CACHE=" + containerCachePath()}
	config.spec.Process.Args = []string{
		filepath.Join(containerBinPath(), "rio"),
		"unpack",
		fmt.Sprintf("--source=%s", src),
		// force uid and gid to zero since these are the values in the container
		// note that the resulting hash used for placing this in the cache dir
		// will end up being different if a tar doesn't only use uid/gid 0!
		// these *must* be zero due to runc issue 1800, otherwise we would
		// choose a more sane value
		"--filters=uid=0,gid=0,mtime=follow" + filterStr,
		"--placer=none",
		"--format=json",
		wareId.String(),
		"/null",
	}

	var wareType string
	var cacheWareId string
	// check if the cached ware already exists
	expectCachePath := fmt.Sprintf("cache/%s/fileset/%s/%s/%s",
		strings.Split(wareId.String(), ":")[0],
		wareId.String()[4:7], wareId.String()[7:10], wareId.String()[4:])
	if _, err := os.Stat(filepath.Join(config.wsPath, expectCachePath)); os.IsNotExist(err) {
		// no cached ware, run the unpack
		outStr, err := invokeRunc(config, nil)
		if err != nil {
			// TODO: currently, this will return an ExecutorFailed error
			// it would be better to determine if runc or rio failed, and return
			// ErrorWareUnpack if it was rio's fault
			return specs.Mount{}, err
		}
		out := RioOutput{}
		for _, line := range strings.Split(outStr, "\n") {
			err := json.Unmarshal([]byte(line), &out)
			if err != nil {
				wfapi.ErrorSerialization("deserializing rio output", err)
			}
			if out.Result.WareId != "" {
				// found wareId
				break
			}
		}
		if out.Result.WareId == "" {
			return specs.Mount{}, wfapi.ErrorWareUnpack(wareId, err)
		}
		wareType = strings.SplitN(out.Result.WareId, ":", 2)[0]
		cacheWareId = strings.SplitN(out.Result.WareId, ":", 2)[1]
	} else {
		// use cached ware
		wareType = strings.SplitN(wareId.String(), ":", 2)[0]
		cacheWareId = strings.SplitN(wareId.String(), ":", 2)[1]
	}

	cachePath := filepath.Join(config.wsPath, "cache", wareType, "fileset", cacheWareId[0:3], cacheWareId[3:6], cacheWareId)
	upperdirPath := filepath.Join(config.runPath, "overlays", fmt.Sprintf("upper-%s", cacheWareId))
	workdirPath := filepath.Join(config.runPath, "overlays", fmt.Sprintf("work-%s", cacheWareId))

	// create upper and work dirs
	err := os.MkdirAll(upperdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of upperdir failed", err)
	}
	err = os.MkdirAll(workdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of workdir failed", err)
	}

	return specs.Mount{
		Destination: dest,
		Source:      "none",
		Type:        "overlay",
		Options: []string{
			"lowerdir=" + cachePath,
			"upperdir=" + upperdirPath,
			"workdir=" + workdirPath,
		},
	}, nil
}

// Creates an overlay mount for a path on the host filesystem
//
// Errors:
//
//     - warpforge-error-io -- when creation of dirs fails
func makeOverlayPathMount(config runConfig, path string, dest string) (specs.Mount, error) {
	mountId := strings.Replace(path, "/", "-", -1)
	mountId = strings.Replace(mountId, ".", "-", -1)
	upperdirPath := filepath.Join(config.runPath, "overlays/upper-", mountId)
	workdirPath := filepath.Join(config.runPath, "overlays/work-", mountId)

	// create upper and work dirs
	err := os.MkdirAll(upperdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of upperdir failed", err)
	}
	err = os.MkdirAll(workdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of workdir failed", err)
	}

	return specs.Mount{
		Destination: dest,
		Source:      "none",
		Type:        "overlay",
		Options: []string{
			"lowerdir=" + path,
			"upperdir=" + upperdirPath,
			"workdir=" + workdirPath,
		},
	}, nil
}

// Creates an overlay mount for a path on the host filesystem
//
// Errors: none -- this function only adds an entry to the runc config and cannot fail
func makeBindPathMount(config runConfig, path string, dest string) (specs.Mount, error) {
	return specs.Mount{
		Source:      path,
		Destination: dest,
		Type:        "none",
		Options:     []string{"rbind", "ro"},
	}, nil
}

// Performs runc invocation and collects results.
//
// Errors:
//
//    - warpforge-error-executor-failed -- invocation of runc caused an error
//    - warpforge-error-io -- i/o error occurred during setup of runc invocation
//    - warpforge-error-serialization -- serialization of runc config failed
func invokeRunc(config runConfig, logWriter io.Writer) (string, error) {
	configBytes, err := json.Marshal(config.spec)
	if err != nil {
		return "", wfapi.ErrorSerialization("serializing runc config", err)
	}
	bundlePath, err := ioutil.TempDir(config.runPath, "bundle-")
	if err != nil {
		return "", wfapi.ErrorIo("creating bundle tmpdir", err)
	}
	err = ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), configBytes, 0644)
	if err != nil {
		return "", wfapi.ErrorIo("writing config.json", err)
	}

	cmd := exec.Command(filepath.Join(config.binPath, "runc"),
		"--root", filepath.Join(config.wsPath, "runc-root"),
		"run",
		"-b", bundlePath, // bundle path
		fmt.Sprintf("warpforge-%d", time.Now().UTC().UnixNano()), // container id
	)

	// if a logWriter was provided, write output to it
	// otherwise, capture stderr and stdout to buffers
	var stderrBuf bytes.Buffer
	var stdoutBuf bytes.Buffer
	if logWriter != nil {
		cmd.Stderr = io.MultiWriter(&stderrBuf, logWriter)
		cmd.Stdout = io.MultiWriter(&stdoutBuf, logWriter)
	} else {
		cmd.Stderr = &stderrBuf
		cmd.Stdout = &stdoutBuf
	}
	err = cmd.Run()
	if err != nil {
		return "", wfapi.ErrorExecutorFailed("runc", err)
	}
	// TODO what exitcode do we care about?
	return stdoutBuf.String(), nil
}

// Packs a given path within a container as a ware in the host system's warehouse
//
// Errors:
//
//    - warpforge-error-executor-failed -- if runc execution fails
//    - warpforge-error-serialization -- if deserialiation of rio output fails
//    - warpforge-error-ware-pack -- if rio pack of ware fails
func rioPack(config runConfig, path string) (wfapi.WareID, error) {
	config.spec.Process.Args = []string{
		filepath.Join(containerBinPath(), "rio"),
		"pack",
		"--format=json",
		"--filters=uid=0,gid=0",
		"--target=ca+file://" + containerWarehousePath(),
		"tar",
		path,
	}

	outStr, err := invokeRunc(config, nil)
	if err != nil {
		// TODO: this should figure out of the rio op failed, or if runc failed and throw different errors
		return wfapi.WareID{}, wfapi.ErrorExecutorFailed(fmt.Sprintf("invoke runc for rio pack of %s failed", path), err)
	}

	out := RioOutput{}
	for _, line := range strings.Split(outStr, "\n") {
		err := json.Unmarshal([]byte(line), &out)
		if err != nil {
			return wfapi.WareID{}, wfapi.ErrorSerialization("deserializing rio pack output", err)
		}
		if out.Result.WareId != "" {
			// found wareId
			break
		}
	}
	if out.Result.WareId == "" {
		return wfapi.WareID{}, wfapi.ErrorWarePack(path, fmt.Errorf("empty WareID value from rio pack"))
	}

	wareId := wfapi.WareID{}
	wareId.Packtype = wfapi.Packtype(strings.Split(out.Result.WareId, ":")[0])
	wareId.Hash = strings.Split(out.Result.WareId, ":")[1]
	return wareId, nil
}

// Get the binary path of warpforge.
// This path is also used for plugins (rio, runc).
//
// Errors:
//
//     - warpforge-error-io -- when locating path of this executable fails
func GetBinPath() (string, error) {
	// determine the path of the running executable
	// other binaries (runc, rio) will be located here as well
	path, override := os.LookupEnv("WARPFORGE_PATH")
	if override {
		return path, nil
	} else {
		binPath, err := os.Executable()
		if err != nil {
			return "", wfapi.ErrorIo("locating binary path", err)
		}
		return filepath.Dir(binPath), nil
	}
}

// Execute a formula
//
// Errors:
//
// - warpforge-error-io -- when an IO operation fails
// - warpforge-error-executor-failed -- when the execution step of the formula fails
// - warpforge-error-ware-unpack -- when a ware unpack operation fails for a formula input
// - warpforge-error-ware-pack -- when a ware pack operation fails for a formula output
// - warpforge-error-serialization -- when serialization or deserialization operations fail
// - warpforge-error-workspace -- when an invalid workspace is provided
// - warpforge-error-formula-invalid -- when an invalid formula is provided
// - warpforge-error-ipld -- when an IPLD error occurs during formula wrapping or computing formula ID
// - warpforge-error-unimplemented -- when an unimplemented feature is used
func Exec(ws *workspace.Workspace, fc wfapi.FormulaAndContext, logger logging.Logger) (wfapi.RunRecord, error) {
	formula := fc.Formula
	context := fc.Context
	rr := wfapi.RunRecord{}

	// convert formula to node
	nFormulaAndContext, errRaw := bindnode.Wrap(&fc, wfapi.TypeSystem.TypeByName("FormulaAndContext")).LookupByString("formula")
	if errRaw != nil {
		return rr, wfapi.ErrorIpld("failed to wrap formula", errRaw)
	}

	// set up the runrecord result
	rr.Guid = uuid.New().String()
	rr.Time = time.Now().Unix()
	lsys := cidlink.DefaultLinkSystem()
	lnk, errRaw := lsys.ComputeLink(cidlink.LinkPrototype{cid.Prefix{
		Version:  1,    // Usually '1'.
		Codec:    0x71, // 0x71 means "dag-cbor" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhType:   0x13, // 0x13 means "sha2-512" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhLength: 64,   // sha2-512 hash has a 64-byte sum.
	}}, nFormulaAndContext.(schema.TypedNode).Representation())
	if errRaw != nil {
		return rr, wfapi.ErrorIpld("failed to compute formula ID", errRaw)
	}
	rr.FormulaID = lnk.String()

	logger.Info(LOG_TAG_START, "")
	logger.Debug(LOG_TAG, printer.Sprint(nFormulaAndContext))

	pwd, errRaw := os.Getwd()
	if errRaw != nil {
		return rr, wfapi.ErrorIo("failed to get working dir", errRaw)
	}

	// get the home workspace location
	var wsPath string
	path, override := os.LookupEnv("WARPFORGE_HOME")
	if override {
		wsPath = path
	} else if ws != nil {
		_, wsPath = ws.Path()
	} else {
		return rr, wfapi.ErrorWorkspace("", fmt.Errorf("no home workspace path was provided for formula exec"))
	}
	wsPath = filepath.Join("/", wsPath, ".warpforge")

	// ensure a warehouse dir exists within the home workspace
	errRaw = os.MkdirAll(filepath.Join(wsPath, "warehouse"), 0755)
	if errRaw != nil {
		return rr, wfapi.ErrorIo("failed to create warehouse", errRaw)
	}

	binPath, err := GetBinPath()
	if err != nil {
		return rr, err
	}

	// each formula execution gets a unique run directory
	// this is used to store working files and is destroyed upon completion
	runPath, errRaw := ioutil.TempDir(os.TempDir(), "warpforge-run-")
	if err != nil {
		return rr, wfapi.ErrorIo("failed to create temp run directory", errRaw)
	}

	_, keep := os.LookupEnv("WARPFORGE_KEEP_RUNDIR")
	if keep {
		logger.Info(LOG_TAG, "using rundir %q\n", runPath)
	} else {
		defer os.RemoveAll(runPath)
	}

	logger.Debug(LOG_TAG, "home workspace path: %s", wsPath)
	logger.Debug(LOG_TAG, "bin path: %s", binPath)
	logger.Debug(LOG_TAG, "run path: %s", runPath)

	// get our configuration for the exec step
	// this config will collect the various input mounts as each is set up
	execConfig, err := getBaseConfig(wsPath, runPath, binPath)
	if err != nil {
		return rr, err
	}

	// loop over formula inputs
	for dest, src := range formula.Inputs.Values {
		var input *wfapi.FormulaInputSimple
		var filters wfapi.FilterMap
		if src.FormulaInputSimple != nil {
			input = src.FormulaInputSimple
		} else if src.FormulaInputComplex != nil {
			input = &src.FormulaInputComplex.Basis
			filters = src.FormulaInputComplex.Filters
		} else {
			return rr, wfapi.ErrorFormulaInvalid(fmt.Sprintf("invalid input for %s", *dest.SandboxPath))
		}

		var mnt specs.Mount
		// create a temporary config for setting up each mount
		config, err := getBaseConfig(wsPath, runPath, binPath)
		if err != nil {
			return rr, err
		}
		if input.Mount != nil {
			// determine the host path
			var hostPath string
			if input.Mount.HostPath[0] == '/' {
				// leading slash, use absolute path
				hostPath = input.Mount.HostPath
			} else {
				// otherwise, use relative path
				hostPath = filepath.Join(pwd, input.Mount.HostPath)
			}

			// add leading slash to destPath since it is removed during parsing
			destPath := filepath.Join("/", string(*dest.SandboxPath))

			// create the mount
			switch input.Mount.Mode {
			case "overlay":
				logger.Info(LOG_TAG,
					"overlay mount:\t%s = %s\t%s = %s",
					color.HiBlueString("hostPath"),
					color.WhiteString(hostPath),
					color.HiBlueString("destPath"),
					color.WhiteString(destPath))
				mnt, err = makeOverlayPathMount(config, hostPath, destPath)
				if err != nil {
					return rr, err
				}
			case "bind":
				logger.Info(LOG_TAG,
					"bind mount:\t%s = %s\t%s = %s",
					color.HiBlueString("hostPath"),
					color.WhiteString(hostPath),
					color.HiBlueString("destPath"),
					color.WhiteString(destPath))
				mnt, err = makeBindPathMount(config, hostPath, destPath)
				if err != nil {
					return rr, err
				}
			default:
				return rr, wfapi.ErrorFormulaInvalid(fmt.Sprintf("unsupported mount mode %q", input.Mount.Mode))
			}
		} else if input.WareID != nil {
			destPath := filepath.Join("/", string(*dest.SandboxPath))
			logger.Info(LOG_TAG,
				"ware mount:\t%s = %s\t%s = %s",
				color.HiBlueString("wareId"),
				color.WhiteString(input.WareID.String()),
				color.HiBlueString("destPath"),
				color.WhiteString(destPath))
			mnt, err = makeWareMount(config, *input.WareID, destPath, context, filters, logger)
			if err != nil {
				return rr, err
			}
		}

		// root mount must come first
		// leading slash is removed during parsing, so `"/"` will result in `""`
		if *dest.SandboxPath == wfapi.SandboxPath("") {
			execConfig.spec.Mounts = append([]specs.Mount{mnt}, execConfig.spec.Mounts...)
		} else {
			execConfig.spec.Mounts = append(execConfig.spec.Mounts, mnt)
		}
	}

	// add network mounts if networking is enabled, otherwise disable networking
	enableNet := false
	switch {
	case formula.Action.Exec != nil:
		if formula.Action.Exec.Network != nil {
			enableNet = *formula.Action.Exec.Network
		}
	case formula.Action.Script != nil:
		if formula.Action.Script.Network != nil {
			enableNet = *formula.Action.Script.Network
		}
	}
	if enableNet {
		execConfig.spec.Mounts = append(execConfig.spec.Mounts, getNetworkMounts(wsPath)...)
		logger.Debug(LOG_TAG, "networking enabled")
	} else {
		// create empty network namespace to disable network
		execConfig.spec.Linux.Namespaces = append(execConfig.spec.Linux.Namespaces,
			specs.LinuxNamespace{
				Type: "network",
				Path: "",
			})
		logger.Debug(LOG_TAG, "networking disabled")
	}

	// configure the action
	switch {
	case formula.Action.Exec != nil:
		logger.Info(LOG_TAG, "executing command: %q", strings.Join(formula.Action.Exec.Command, " "))
		execConfig.spec.Process.Args = formula.Action.Exec.Command
		execConfig.spec.Process.Cwd = "/"
	case formula.Action.Script != nil:
		// the script action creates a seperate "entry" file for each element in the script contents
		// and creates a "run" file which executes these in order within the the current shell process.

		logger.Info(LOG_TAG, "executing script\t%s = %s",
			color.HiBlueString("interpreter"),
			color.WhiteString(formula.Action.Script.Interpreter))

		// create the script directory
		scriptPath := filepath.Join(runPath, "script")
		errRaw = os.MkdirAll(scriptPath, 0755)
		if errRaw != nil {
			return rr, wfapi.ErrorIo(fmt.Sprintf("failed to create script dir %q", scriptPath), errRaw)
		}

		// open the script file
		scriptFilePath := filepath.Join(scriptPath, "run")
		scriptFile, err := os.OpenFile(scriptFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return rr, wfapi.ErrorIo(fmt.Sprintf("failed to open script file for writing: %s", scriptFilePath), err)
		}
		defer scriptFile.Close()

		// iterate over each item in script contents
		for n, entry := range formula.Action.Script.Contents {
			// open the entry file (entry-#)
			entryFilePath := filepath.Join(scriptPath, fmt.Sprintf("entry-%d", n))
			entryFile, err := os.OpenFile(entryFilePath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return rr, wfapi.ErrorIo(fmt.Sprintf("failed to open entry file %q for writing", entryFilePath), err)
			}
			defer entryFile.Close()

			// write the entry file
			_, err = entryFile.WriteString(entry + "\n")
			if err != nil {
				return rr, wfapi.ErrorIo(fmt.Sprintf("error writing entry file %q", entryFilePath), err)
			}

			// write a line to execute this entry into the main script file
			// we use the POSIX standard `. filename` to cause the entry file to be executed
			// within the current shell process (also known as `source` in bash)
			entrySrc := fmt.Sprintf(". %s\n",
				filepath.Join(containerScriptPath(), fmt.Sprintf("entry-%d", n)))
			_, err = scriptFile.WriteString(entrySrc)
			if err != nil {
				return rr, wfapi.ErrorIo("error writing script file", err)
			}
		}

		// create a mount for the script file
		scriptMount, err := makeBindPathMount(execConfig, scriptPath, containerScriptPath())
		if err != nil {
			return rr, wfapi.ErrorIo("failed to create script mount", err)
		}
		execConfig.spec.Mounts = append(execConfig.spec.Mounts, scriptMount)

		// configure the process
		execConfig.spec.Process.Args = []string{formula.Action.Script.Interpreter,
			filepath.Join(containerScriptPath(), "run"),
		}
		execConfig.spec.Process.Cwd = "/"
	default:
		return rr, wfapi.ErrorFormulaInvalid("unsupported action, or no action defined")
	}

	// run the action
	logger.Info(LOG_TAG_OUTPUT_START, "")
	_, err = invokeRunc(execConfig, logger.InfoWriter(LOG_TAG_OUTPUT))
	logger.Info(LOG_TAG_OUTPUT_END, "")
	if err != nil {
		return rr, err
	}
	// TODO exit code?
	rr.Exitcode = 0

	// collect outputs
	rr.Results.Values = make(map[wfapi.OutputName]wfapi.FormulaInputSimple)
	for name, gather := range formula.Outputs.Values {
		switch {
		case gather.From.SandboxPath != nil:
			path := string(*gather.From.SandboxPath)
			wareId, err := rioPack(execConfig, path)
			if err != nil {
				return rr, wfapi.ErrorWarePack(path, err)
			}
			rr.Results.Keys = append(rr.Results.Keys, name)
			rr.Results.Values[name] = wfapi.FormulaInputSimple{WareID: &wareId}
			logger.Info(LOG_TAG, "packed %q:\t%s = %s\t%s=%s",
				name,
				color.HiBlueString("path"),
				color.WhiteString("/"+path),
				color.HiBlueString("wareId"),
				color.WhiteString(wareId.String()))
		case gather.From.SandboxVar != nil:
			return rr, wfapi.ErrorUnimplemented("SandboxVar output type not supported")
		default:
			return rr, wfapi.ErrorFormulaInvalid(fmt.Sprintf("invalid gather directive provided for output %q", name))
		}
	}

	logger.Info(LOG_TAG, "RunRecord:\n\t%s = %s\n\t%s = %s\n\t%s = %s\n\t%s = %s\n\t%s:",
		color.HiBlueString("GUID"),
		color.WhiteString(rr.Guid),
		color.HiBlueString("FormulaID"),
		color.WhiteString(rr.FormulaID),
		color.HiBlueString("Exitcode"),
		color.WhiteString(fmt.Sprintf("%d", rr.Exitcode)),
		color.HiBlueString("Time"),
		color.WhiteString(fmt.Sprintf("%d", rr.Time)),
		color.HiBlueString("Results"),
	)

	for k, v := range rr.Results.Values {
		logger.Info(LOG_TAG, "\t\t%s: %s", k, v.WareID)
	}

	logger.Info(LOG_TAG_END, "")

	return rr, nil
}
