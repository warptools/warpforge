package formulaexec

import (
	"bytes"
	"context"
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
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/tracing"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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

func getMountDirSymlinks(start string) []string {
	paths := []string{}
	path := start

	fi, _ := os.Lstat(path)
	for fi != nil && fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		pointee, _ := os.Readlink(path)
		if !filepath.IsAbs(pointee) {
			// the symlink points to a relative path
			//add the symlink's path to create an absolute path
			pointee = filepath.Join(filepath.Dir(path), pointee)
		}
		path = pointee
		paths = append(paths, path)
		fi, _ = os.Lstat(path)
	}

	return paths
}

func getNetworkMounts(wsPath string) []specs.Mount {
	// some operations require network access, which requires some configuration
	// we provide a resolv.conf for DNS configuration and /etc/ssl/certs
	// for trusted CAs from the host system

	mounts := []specs.Mount{}
	resolvMount := specs.Mount{
		Source:      "/etc/resolv.conf",
		Destination: "/etc/resolv.conf",
		Type:        "none",
		Options:     []string{"rbind"},
	}
	mounts = append(mounts, resolvMount)

	caMount := specs.Mount{
		Source:      "/etc/ssl/certs",
		Destination: "/etc/ssl/certs",
		Type:        "none",
		Options:     []string{"rbind", "ro"},
	}
	mounts = append(mounts, caMount)

	// some distros use symlinks in /etc/ssl/certificates
	// if this directory exists, mount and follow all symlinks
	// so that they will resolve.
	if certFiles, err := ioutil.ReadDir("/etc/ssl/certs"); err == nil {
		for _, file := range certFiles {
			path := filepath.Join("/etc/ssl/certs", file.Name())

			for _, p := range getMountDirSymlinks(path) {
				dir := filepath.Dir(p)

				// ignore duplicate mounts
				duplicate := false
				for _, m := range mounts {
					if m.Source == dir {
						duplicate = true
						break
					}
				}
				if duplicate {
					continue
				}

				caSymlinkMount := specs.Mount{
					Source:      dir,
					Destination: dir,
					Type:        "none",
					Options:     []string{"rbind", "ro"},
				}
				mounts = append(mounts, caSymlinkMount)
			}
		}
	}

	return mounts
}

// Creates a base configuration for runc, which is later modified before running``
//
// Errors:
//
//    - warpforge-error-io -- when file reads, writes, and dir creation fails
//    - warpforge-error-executor-failed -- when generation of the base spec by runc fails
func getBaseConfig(wsPath, runPath, binPath string) (runConfig, wfapi.Error) {
	rc := runConfig{
		runPath: runPath,
		wsPath:  wsPath,
		binPath: binPath,
	}

	// generate a runc rootless config, then read the resulting config
	configFile := filepath.Join(runPath, "config.json")
	err := os.RemoveAll(configFile)
	if err != nil {
		return rc, wfapi.ErrorIo("failed to remove config.json", &configFile, err)
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
		return rc, wfapi.ErrorIo("failed to read runc config", &configFile, err)
	}
	err = json.Unmarshal(configFileBytes, &rc.spec)
	if err != nil {
		return rc, wfapi.ErrorExecutorFailed("runc",
			wfapi.ErrorSerialization("failed to parse runc config", err))
	}

	// set up root -- this is not actually used since it is replaced with an overlayfs
	rootPath := filepath.Join(runPath, "root")
	err = os.MkdirAll(rootPath, 0755)
	if err != nil {
		return rc, wfapi.ErrorIo("failed to create root directory", &rootPath, err)
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

	// check if /dev/tty exists
	_, err = os.Open("/dev/tty")
	if err == nil {
		// enable a normal interactive terminal
		rc.spec.Process.Terminal = true
	} else {
		// disable terminal when executing on systems without a tty (e.g., github actions)
		rc.spec.Process.Terminal = false
	}

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
//     - warpforge-error-ware-unpack -- when `rio unpack` operation fails
func makeWareMount(ctx context.Context,
	config runConfig,
	wareId wfapi.WareID,
	dest string,
	context *wfapi.FormulaContext,
	filters wfapi.FilterMap,
	logger logging.Logger) (specs.Mount, wfapi.Error) {
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
				mnt, err := makeBindPathMount(ctx, config, hostPath, src, true)
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
	if _, errRaw := os.Stat(filepath.Join(config.wsPath, expectCachePath)); os.IsNotExist(errRaw) {
		// no cached ware, run the unpack
		outStr, err := invokeRunc(ctx, config, nil)
		if err != nil {
			// TODO: currently, this will return an ExecutorFailed error
			// it would be better to determine if runc or rio failed, and return
			// ErrorWareUnpack if it was rio's fault
			return specs.Mount{}, err
		}
		out := RioOutput{}
		for _, line := range strings.Split(outStr, "\n") {
			errRaw := json.Unmarshal([]byte(line), &out)
			if err != nil {
				return specs.Mount{}, wfapi.ErrorWareUnpack(wareId, wfapi.ErrorSerialization("deserializing rio output", errRaw))
			}
			if out.Result.WareId != "" {
				// found wareId
				break
			}
		}
		if out.Result.WareId == "" {
			return specs.Mount{}, wfapi.ErrorWareUnpack(wareId, fmt.Errorf("rio unpack resulted in empty WareID output"))
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
	errRaw := os.MkdirAll(upperdirPath, 0755)
	if errRaw != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of upperdir failed", &upperdirPath, errRaw)
	}
	errRaw = os.MkdirAll(workdirPath, 0755)
	if errRaw != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of workdir failed", &workdirPath, errRaw)
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
func makeOverlayPathMount(ctx context.Context, config runConfig, path string, dest string) (specs.Mount, wfapi.Error) {
	mountId := strings.Replace(path, "/", "-", -1)
	mountId = strings.Replace(mountId, ".", "-", -1)
	upperdirPath := filepath.Join(config.runPath, "overlays/upper-", mountId)
	workdirPath := filepath.Join(config.runPath, "overlays/work-", mountId)

	// create upper and work dirs
	err := os.MkdirAll(upperdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of upperdir failed", &upperdirPath, err)
	}
	err = os.MkdirAll(workdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of workdir failed", &workdirPath, err)
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
func makeBindPathMount(ctx context.Context, config runConfig, path string, dest string, readOnly bool) (specs.Mount, wfapi.Error) {
	options := []string{"rbind"}
	if readOnly {
		options = append(options, "ro")
	}
	return specs.Mount{
		Source:      path,
		Destination: dest,
		Type:        "none",
		Options:     options,
	}, nil
}

// Performs runc invocation and collects results.
//
// Errors:
//
//    - warpforge-error-executor-failed -- invocation of runc caused an error
//    - warpforge-error-io -- i/o error occurred during setup of runc invocation
func invokeRunc(ctx context.Context, config runConfig, logWriter io.Writer) (string, wfapi.Error) {
	ctx, span := tracing.Start(ctx, "invokeRunc")
	defer span.End()

	configBytes, err := json.Marshal(config.spec)
	if err != nil {
		return "", wfapi.ErrorExecutorFailed("runc", wfapi.ErrorSerialization("failed to serialize runc config", err))
	}
	bundlePath, err := ioutil.TempDir(config.runPath, "bundle-")
	if err != nil {
		return "", wfapi.ErrorIo("creating bundle tmpdir", nil, err)
	}
	configPath := filepath.Join(bundlePath, "config.json")
	err = ioutil.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		return "", wfapi.ErrorIo("writing config.json", &configPath, err)
	}

	cmd := exec.Command(filepath.Join(config.binPath, "runc"),
		"--root", filepath.Join(config.wsPath, "runc-root"),
		"run",
		"-b", bundlePath, // bundle path
		fmt.Sprintf("warpforge-%d", time.Now().UTC().UnixNano()), // container id
	)

	if config.spec.Process.Terminal {
		cmd.Stdin = os.Stdin
	}

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
		span.SetStatus(codes.Error, err.Error())
		return "", wfapi.ErrorExecutorFailed("runc", fmt.Errorf("%s %s", stdoutBuf.String(), stderrBuf.String()))
	}
	// TODO what exitcode do we care about?
	return stdoutBuf.String(), nil
}

// Packs a given path within a container as a ware in the host system's warehouse
//
// Errors:
//
//    - warpforge-error-executor-failed -- if runc execution fails
//    - warpforge-error-ware-pack -- if rio pack of ware fails
func rioPack(ctx context.Context, config runConfig, path string) (wfapi.WareID, error) {
	ctx, span := tracing.Start(ctx, "rioPack")
	defer span.End()
	config.spec.Process.Args = []string{
		filepath.Join(containerBinPath(), "rio"),
		"pack",
		"--format=json",
		"--filters=uid=0,gid=0",
		"--target=ca+file://" + containerWarehousePath(),
		"tar",
		path,
	}

	outStr, err := invokeRunc(ctx, config, nil)
	if err != nil {
		// TODO: this should figure out of the rio op failed, or if runc failed and throw different errors
		return wfapi.WareID{}, wfapi.ErrorExecutorFailed(fmt.Sprintf("invoke runc for rio pack of %s failed", path), err)
	}

	out := RioOutput{}
	for _, line := range strings.Split(outStr, "\n") {
		err := json.Unmarshal([]byte(line), &out)
		if err != nil {
			return wfapi.WareID{}, wfapi.ErrorWarePack(path,
				wfapi.ErrorSerialization("deserializing rio pack output", err))
		}
		if out.Result.WareId != "" {
			// found wareId
			span.AddEvent("Found ware ID", trace.WithAttributes(attribute.String(tracing.SpanAttrWarpforgePackId, out.Result.WareId)))
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
func GetBinPath() (string, wfapi.Error) {
	// determine the path of the running executable
	// other binaries (runc, rio) will be located here as well
	path, override := os.LookupEnv("WARPFORGE_PATH")
	if override {
		return path, nil
	} else {
		binPath, err := os.Executable()
		if err != nil {
			return "", wfapi.ErrorIo("failed to locate binary path", nil, err)
		}
		return filepath.Dir(binPath), nil
	}
}

// Internal function for executing a formula
//
// Errors:
//
// - warpforge-error-io -- when an IO operation fails
// - warpforge-error-executor-failed -- when the execution step of the formula fails
// - warpforge-error-ware-unpack -- when a ware unpack operation fails for a formula input
// - warpforge-error-ware-pack -- when a ware pack operation fails for a formula output
// - warpforge-error-workspace -- when an invalid workspace is provided
// - warpforge-error-formula-invalid -- when an invalid formula is provided
// - warpforge-error-serialization -- when serialization or deserialization of a memo fails
func execFormula(ctx context.Context, ws *workspace.Workspace, fc wfapi.FormulaAndContext, formulaConfig wfapi.FormulaExecConfig, logger logging.Logger) (wfapi.RunRecord, wfapi.Error) {
	ctx, span := tracing.Start(ctx, "execFormula")
	defer span.End()
	rr := wfapi.RunRecord{}

	if fc.Formula.Formula == nil {
		return rr, wfapi.ErrorFormulaInvalid("no v1 Formula in FormulaCapsule")
	}
	formula := fc.Formula.Formula

	context := wfapi.FormulaContext{}
	if fc.Context != nil && fc.Context.FormulaContext != nil {
		context = *fc.Context.FormulaContext
	}

	// convert formula to node
	nFormula := bindnode.Wrap(fc.Formula.Formula, wfapi.TypeSystem.TypeByName("Formula"))

	// set up the runrecord result
	rr.Guid = uuid.New().String()
	rr.Time = time.Now().Unix()
	lsys := cidlink.DefaultLinkSystem()
	lnk, errRaw := lsys.ComputeLink(cidlink.LinkPrototype{cid.Prefix{
		Version:  1,    // Usually '1'.
		Codec:    0x71, // 0x71 means "dag-cbor" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhType:   0x20, // 0x20 means "sha2-384" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhLength: 48,   // sha2-384 hash has a 48-byte sum.
	}}, nFormula.(schema.TypedNode).Representation())
	if errRaw != nil {
		// panic! this should never fail unless IPLD is broken
		panic(fmt.Sprintf("Fatal IPLD Error: lsys.ComputeLink failed for Formula: %s", errRaw))
	}
	fid, errRaw := lnk.(cidlink.Link).StringOfBase('z')
	if errRaw != nil {
		panic(fmt.Sprintf("Fatal IPLD Error: failed to encode CID for Formula: %s", errRaw))
	}
	rr.FormulaID = fid
	span.SetAttributes(attribute.String(tracing.SpanAttrWarpforgeFormulaId, fid))
	logger.Info(LOG_TAG_START, "")

	// check if a memoized RunRecord already exists
	if !formulaConfig.DisableMemoization {
		memo, err := loadMemo(ws, fid)
		if err != nil {
			return rr, err
		}
		if memo != nil {
			logger.PrintRunRecord(LOG_TAG, *memo, true)
			logger.Info(LOG_TAG_END, "")
			return *memo, nil

		}
	}

	formulaSerial, errRaw := ipld.Marshal(ipldjson.Encode, formula, wfapi.TypeSystem.TypeByName("Formula"))
	if errRaw != nil {
		return rr, wfapi.ErrorFormulaInvalid(fmt.Sprintf("failed to re-serialize formula: %s", errRaw))
	}
	logger.Debug(LOG_TAG, "resolved formula:")
	logger.Debug(LOG_TAG, string(formulaSerial))

	pwd, errRaw := os.Getwd()
	if errRaw != nil {
		return rr, wfapi.ErrorIo("failed to get working dir", nil, errRaw)
	}

	// get the root workspace location
	var wsPath string
	path, override := os.LookupEnv("WARPFORGE_HOME")
	if override {
		wsPath = path
	} else if ws != nil {
		_, wsPath = ws.Path()
	} else {
		return rr, wfapi.ErrorWorkspace("", fmt.Errorf("no root workspace path was provided for formula exec"))
	}
	wsPath = filepath.Join("/", wsPath, ".warpforge")

	// ensure a warehouse dir exists within the root workspace
	warehousePath := filepath.Join(wsPath, "warehouse")
	errRaw = os.MkdirAll(warehousePath, 0755)
	if errRaw != nil {
		return rr, wfapi.ErrorIo("failed to create warehouse", &warehousePath, errRaw)
	}

	binPath, err := GetBinPath()
	if err != nil {
		return rr, err
	}

	// each formula execution gets a unique run directory
	// this is used to store working files and is destroyed upon completion
	base, override := os.LookupEnv("WARPFORGE_RUNPATH")
	if !override {
		base = os.TempDir()
	}
	runPath, errRaw := ioutil.TempDir(base, "warpforge-run-")
	if errRaw != nil {
		return rr, wfapi.ErrorIo("failed to create temp run directory", nil, errRaw)
	}

	_, keep := os.LookupEnv("WARPFORGE_KEEP_RUNDIR")
	if keep {
		logger.Info(LOG_TAG, "using rundir %q\n", runPath)
	} else {
		defer os.RemoveAll(runPath)
	}

	logger.Debug(LOG_TAG, "root workspace path: %s", wsPath)
	logger.Debug(LOG_TAG, "bin path: %s", binPath)
	logger.Debug(LOG_TAG, "run path: %s", runPath)

	// get our configuration for the exec step
	// this config will collect the various inputs (mounts and vars) as each is set up
	execConfig, err := getBaseConfig(wsPath, runPath, binPath)
	if err != nil {
		return rr, err
	}

	// loop over formula inputs
	for port, input := range formula.Inputs.Values {
		// get the FormulaInputSimple and FilterMap for this input
		inputSimple := input.Basis()
		var filters wfapi.FilterMap
		if input.FormulaInputComplex != nil {
			filters = input.FormulaInputComplex.Filters
		}

		if port.SandboxVar != nil {
			// construct the string for this env var
			varStr := fmt.Sprintf("%s=%s", *port.SandboxVar, *inputSimple.Literal)

			// insert the variable to the container spec, de-duplicating any existing variables
			// note that the runc default config has PATH and TERM defined, so this allows
			// for overriding those defaults
			oldVars := execConfig.spec.Process.Env
			execConfig.spec.Process.Env = []string{}
			for _, v := range oldVars {
				if strings.Split(v, "=")[0] != string(*port.SandboxVar) {
					execConfig.spec.Process.Env = append(execConfig.spec.Process.Env, v)
				}
			}
			execConfig.spec.Process.Env = append(execConfig.spec.Process.Env, varStr)
		} else if port.SandboxPath != nil {
			var mnt specs.Mount
			// create a temporary config for setting up the mount
			config, err := getBaseConfig(wsPath, runPath, binPath)
			if err != nil {
				return rr, err
			}

			// determine the host path for mount types
			var hostPath string
			if inputSimple.Mount != nil {
				if inputSimple.Mount.HostPath[0] == '/' {
					// leading slash, use absolute path
					hostPath = inputSimple.Mount.HostPath
				} else {
					// otherwise, use relative path
					hostPath = filepath.Join(pwd, inputSimple.Mount.HostPath)
				}
			}

			// add leading slash to destPath since it is removed during parsing
			destPath := filepath.Join("/", string(*port.SandboxPath))

			// create the mount based on type
			switch {
			case inputSimple.Mount != nil && inputSimple.Mount.Mode == wfapi.MountMode_Overlay:
				// overlay mount
				logger.Info(LOG_TAG,
					"overlay mount:\t%s = %s\t%s = %s",
					color.HiBlueString("hostPath"),
					color.WhiteString(hostPath),
					color.HiBlueString("destPath"),
					color.WhiteString(destPath))
				mnt, err = makeOverlayPathMount(ctx, config, hostPath, destPath)
				if err != nil {
					return rr, err
				}
			case inputSimple.Mount != nil && inputSimple.Mount.Mode == wfapi.MountMode_Readonly:
				// read only bind mount
				logger.Info(LOG_TAG,
					"bind mount (ro):\t%s = %s\t%s = %s",
					color.HiBlueString("hostPath"),
					color.WhiteString(hostPath),
					color.HiBlueString("destPath"),
					color.WhiteString(destPath))
				mnt, err = makeBindPathMount(ctx, config, hostPath, destPath, true)
				if err != nil {
					return rr, err
				}
			case inputSimple.Mount != nil && inputSimple.Mount.Mode == wfapi.MountMode_Readwrite:
				// bind mount
				logger.Info(LOG_TAG,
					"bind mount (rw):\t%s = %s\t%s = %s",
					color.HiBlueString("hostPath"),
					color.WhiteString(hostPath),
					color.HiBlueString("destPath"),
					color.WhiteString(destPath))
				mnt, err = makeBindPathMount(ctx, config, hostPath, destPath, false)
				if err != nil {
					return rr, err
				}

			case inputSimple.WareID != nil:
				// ware mount
				destPath = filepath.Join("/", string(*port.SandboxPath))
				logger.Info(LOG_TAG,
					"ware mount:\t%s = %s\t%s = %s",
					color.HiBlueString("wareId"),
					color.WhiteString(inputSimple.WareID.String()),
					color.HiBlueString("destPath"),
					color.WhiteString(destPath))
				mnt, err = makeWareMount(ctx, config, *inputSimple.WareID, destPath, &context, filters, logger)
				if err != nil {
					return rr, err
				}
			default:
				return rr, wfapi.ErrorFormulaInvalid(fmt.Sprintf("unsupported mount mode %q", inputSimple.Mount.Mode))
			}

			// root mount must come first
			// leading slash is removed during parsing, so `"/"` will result in `""`
			if *port.SandboxPath == wfapi.SandboxPath("") {
				execConfig.spec.Mounts = append([]specs.Mount{mnt}, execConfig.spec.Mounts...)
			} else {
				execConfig.spec.Mounts = append(execConfig.spec.Mounts, mnt)
			}
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
			return rr, wfapi.ErrorIo("failed to create script dir", &scriptPath, errRaw)
		}

		// open the script file
		scriptFilePath := filepath.Join(scriptPath, "run")
		scriptFile, errRaw := os.OpenFile(scriptFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if errRaw != nil {
			return rr, wfapi.ErrorIo("failed to open script file for writing", &scriptFilePath, errRaw)
		}
		defer scriptFile.Close()

		// iterate over each item in script contents
		for n, entry := range formula.Action.Script.Contents {
			// open the entry file (entry-#)
			entryFilePath := filepath.Join(scriptPath, fmt.Sprintf("entry-%d", n))
			entryFile, err := os.OpenFile(entryFilePath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return rr, wfapi.ErrorIo("failed to open entry file for writing", &entryFilePath, err)
			}
			defer entryFile.Close()

			// write the entry file
			_, err = entryFile.WriteString(entry + "\n")
			if err != nil {
				return rr, wfapi.ErrorIo("error writing entry file", &entryFilePath, err)
			}

			// write a line to execute this entry into the main script file
			// we use the POSIX standard `. filename` to cause the entry file to be executed
			// within the current shell process (also known as `source` in bash)
			entrySrc := fmt.Sprintf(". %s\n",
				filepath.Join(containerScriptPath(), fmt.Sprintf("entry-%d", n)))
			_, err = scriptFile.WriteString(entrySrc)
			if err != nil {
				return rr, wfapi.ErrorIo("error writing entry to script file", &scriptFilePath, err)
			}
		}

		// create a mount for the script file
		scriptMount, err := makeBindPathMount(ctx, execConfig, scriptPath, containerScriptPath(), false)
		if err != nil {
			return rr, err
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

	// determine output formatting. if interactive, do not apply any special formatting
	var runcWriter io.Writer
	if formulaConfig.Interactive {
		runcWriter = logger.RawWriter()
	} else {
		runcWriter = logger.OutputWriter(LOG_TAG_OUTPUT)
	}

	// run the action
	logger.Output(LOG_TAG_OUTPUT_START, "")
	_, err = invokeRunc(ctx, execConfig, runcWriter)
	logger.Output(LOG_TAG_OUTPUT_END, "")
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
			wareId, err := rioPack(ctx, execConfig, path)
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
			panic("SandboxVar output type not supported")
		default:
			return rr, wfapi.ErrorFormulaInvalid(fmt.Sprintf("invalid gather directive provided for output %q", name))
		}
	}

	logger.PrintRunRecord(LOG_TAG, rr, false)
	logger.Info(LOG_TAG_END, "")

	// memoize this run, if we have a valid workspace
	if ws != nil {
		err = memoizeRun(ws, rr)
		if err != nil {
			return rr, err
		}
	}

	return rr, nil
}

// Execute a Formula using the provided root Workspace
//
// Errors:
//
//     - warpforge-error-formula-execution-failed -- when an error occurs during formula execution
//     - warpforge-error-executor-failed -- when the execution step of the formula fails
//     - warpforge-error-ware-unpack -- when a ware unpack operation fails for a formula input
//     - warpforge-error-ware-pack -- when a ware pack operation fails for a formula output
//     - warpforge-error-workspace -- when an invalid workspace is provided
//     - warpforge-error-formula-invalid -- when an invalid formula is provided
//     - warpforge-error-serialization -- when serialization or deserialization of a memo fails
func Exec(ctx context.Context, ws *workspace.Workspace, fc wfapi.FormulaAndContext, formulaConfig wfapi.FormulaExecConfig) (wfapi.RunRecord, wfapi.Error) {
	ctx, span := tracing.Start(ctx, "Exec")
	defer span.End()
	logger := logging.Ctx(ctx)
	rr, err := execFormula(ctx, ws, fc, formulaConfig, *logger)
	if err != nil {
		switch err.(*wfapi.ErrorVal).Code() {
		case "warpforge-error-io":
			err := wfapi.ErrorFormulaExecutionFailed(err)
			tracing.SetSpanError(ctx, err)
			return rr, err
		default:
			tracing.SetSpanError(ctx, err)
			// Error Codes -= warpforge-error-io
			return rr, err
		}
	}
	return rr, nil
}
