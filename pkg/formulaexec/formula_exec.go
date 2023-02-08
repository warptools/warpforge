package formulaexec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
	"github.com/serum-errors/go-serum"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

const (
	LOG_TAG_START        = "│ ┌─ formula"
	LOG_TAG              = "│ │  formula"
	LOG_TAG_OUTPUT_START = "│ │ ┌─ output "
	LOG_TAG_OUTPUT       = "│ │ │  output "
	LOG_TAG_OUTPUT_END   = "│ │ └─ output "
	LOG_TAG_END          = "│ └─ formula"
)

type RioResult struct {
	WareId string `json:"wareID"`
}
type RioOutput struct {
	Result RioResult `json:"result"`
}

// DefaultRunPathPrefix will be the prefix used to create a temporary execution directory
const DefaultRunPathPrefix = "warpforge-run-"

// runConfig is the minimal set of things to execute invokeRunc
type runcConfig struct {
	binPath     string     // path containing required binaries to run (rio, runc)
	interactive bool       // flag to determine if stdin should be wired to containier for interactivity
	rootPath    string     // rootPath is the root directory for storage of container state
	runPath     string     // path used to store temporary files used for formula run
	spec        specs.Spec // OCI config spec
	cachePath   string     // directory where wares will be cached
}

func (rc runcConfig) debug(ctx context.Context) {
	logger := logging.Ctx(ctx)
	logger.Debug(LOG_TAG+" runc-config", "binpath: %s", rc.binPath)
	logger.Debug(LOG_TAG+" runc-config", "interactive: %t", rc.interactive)
	logger.Debug(LOG_TAG+" runc-config", "rootPath: %s", rc.rootPath)
	logger.Debug(LOG_TAG+" runc-config", "runPath: %s", rc.runPath)
	logger.Debug(LOG_TAG+" runc-config", "cachePath: %s", rc.cachePath)
	spec, _ := json.Marshal(rc.spec)
	logger.Debug(LOG_TAG+" runc-config", "spec: %s", string(spec))
}

// ExecConfig is an interface that may be used to configure behavior of formula execution.
// This simplifies some of the OS interactions and environment variable lookups by
// definining what
type ExecConfig struct {
	// BinPath returns the path to required binaries (rio, runc)
	BinPath string
	// KeepRunDir returns whether or not to delete the run directory
	KeepRunDir bool
	// runPathBase will be generated on init if not provided
	RunPathBase string
	// WarehousePathOverride overrides the directory where outputs are stored.
	WhPathOverride *string
	// WorkingDirectory is the directory we are running warpforge from
	WorkingDirectory string
	// FormulaDirectory is the location of the formula (or module) being run
	// Relative mount paths are relative to this path
	FormulaDirectory string
}

func (cfg *ExecConfig) debug(ctx context.Context) {
	logger := logging.Ctx(ctx)
	logger.Debug(LOG_TAG, "bin path: %q", cfg.BinPath)
	logger.Debug(LOG_TAG, "run path base: %q", cfg.RunPathBase)
	logger.Debug(LOG_TAG, "keep run dir: %t", cfg.KeepRunDir)
	logger.Debug(LOG_TAG, "warehouse override path: %v", cfg.WhPathOverride)
}

type internalConfig struct {
	ExecConfig
	RootWs *workspace.Workspace
	wfapi.FormulaExecConfig
	wfapi.FormulaAndContext
}

func (cfg *internalConfig) loadMemo(ctx context.Context, fid string) (*wfapi.RunRecord, error) {
	// check if a memoized RunRecord already exists
	if !cfg.FormulaExecConfig.DisableMemoization && cfg.RootWs != nil {
		memo, err := cfg.RootWs.LoadMemo(fid)
		if err != nil {
			return nil, err
		}
		return memo, nil
	}
	return nil, nil
}

func (cfg *internalConfig) storeMemo(ctx context.Context, rr wfapi.RunRecord) error {
	if cfg.RootWs != nil {
		if err := cfg.RootWs.StoreMemo(rr); err != nil {
			return err
		}
		return nil
	}
	logger := logging.Ctx(ctx)
	logger.Info("", "unable to store memo of run record")
	return nil
}

func (cfg *ExecConfig) warehousePathOverride() (string, bool) {
	if cfg.WhPathOverride == nil {
		return "", false
	}
	if filepath.IsAbs(*cfg.WhPathOverride) {
		return *cfg.WhPathOverride, true
	}
	return filepath.Join(cfg.WorkingDirectory, *cfg.WhPathOverride), true
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
	//FIXME: This function is not implemented in an easily testable way.
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

func getNetworkMounts() []specs.Mount {
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

				if isDuplicateMount(mounts, dir, dir) {
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

// isDuplicateMount returns true if the source or destination is in mounts
func isDuplicateMount(mounts []specs.Mount, src string, dst string) bool {
	for _, m := range mounts {
		if m.Source == src {
			return true
		}
		if m.Destination == dst {
			return true
		}
	}
	return false
}

func isSymbolicLink(m fs.FileMode) bool {
	return m&fs.ModeSymlink == fs.ModeSymlink
}

func getBinMounts(ctx context.Context, binPath string) []specs.Mount {
	wfBinMount := specs.Mount{
		Source:      binPath,
		Destination: containerBinPath(),
		Type:        "none",
		Options:     []string{"rbind", "ro"},
	}
	log := logging.Ctx(ctx)
	result := []specs.Mount{wfBinMount}
	rioPath := filepath.Join(binPath, "rio")
	fi, err := os.Lstat(rioPath)
	if err != nil {
		log.Info(LOG_TAG, "rio not found: %v", err)
		return result
	}
	if !isSymbolicLink(fi.Mode()) {
		return result
	}
	for isSymbolicLink(fi.Mode()) {
		p, err := os.Readlink(rioPath)
		if err != nil {
			log.Info(LOG_TAG, "could not read symbolic link for rio %q: %v", rioPath, err)
			return result
		}
		rioPath = p
		fi, err = os.Lstat(rioPath)
		if err != nil {
			log.Info(LOG_TAG, "could not find rio via link %q: %v", rioPath, err)
			return result
		}
	}
	dst := filepath.Join(containerBinPath(), "rio")
	log.Debug(LOG_TAG, "mounting rio via symbolic link %q->%q", rioPath, dst)
	symMount := specs.Mount{
		Source:      rioPath,
		Destination: dst,
		Type:        "none",
		Options:     []string{"bind", "ro"},
	}
	result = append(result, symMount)
	return result
}

// newRuncSpec executes "runc spec" for the given parameters and parses the result
//
// Errors:
//
//    - warpforge-error-executor-failed -- when generation of the base spec by runc fails
//    - warpforge-error-io  -- when the generated spec file cannot be read
func newRuncSpec(ctx context.Context, runPath string, binPath string) (specs.Spec, error) {
	var result specs.Spec
	configFile := filepath.Join(runPath, "config.json")
	// generate a runc rootless config, then read the resulting config
	if err := os.RemoveAll(configFile); err != nil {
		return result, wfapi.ErrorIo("failed to remove config.json", configFile, err)
	}

	var cmd *exec.Cmd
	cmdCtx, cmdSpan := tracing.Start(ctx, "runc config", trace.WithAttributes(tracing.AttrFullExecNameRunc))
	defer cmdSpan.End()
	if os.Getuid() == 0 {
		cmd = exec.CommandContext(cmdCtx, filepath.Join(binPath, "runc"),
			"spec",
			"-b", runPath)
	} else {
		cmd = exec.CommandContext(cmdCtx, filepath.Join(binPath, "runc"),
			"spec",
			"--rootless",
			"-b", runPath)
	}
	err := cmd.Run()
	tracing.EndWithStatus(cmdSpan, err)
	if err != nil {
		return result, wfapi.ErrorExecutorFailed("failed to generate runc config", err)
	}

	configFileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return result, wfapi.ErrorIo("failed to read runc config", configFile, err)
	}

	err = json.Unmarshal(configFileBytes, &result)
	if err != nil {
		return result, wfapi.ErrorExecutorFailed("runc",
			wfapi.ErrorSerialization("failed to parse runc config", err))
	}
	return result, nil
}

// copySpec returns a copy of the spec
// Internally, copySpec serializes and deserializes the data
// Presumably this can round trip, but an error is returned just in case.
//
// Errors:
//
//    - warpforge-error-internal -- when copying the spec fails
func copySpec(spec specs.Spec) (specs.Spec, error) {
	var result specs.Spec
	raw, err := json.Marshal(spec)
	if err != nil {
		return result, serum.Error(wfapi.ECodeInternal,
			serum.WithCause(wfapi.ErrorSerialization("failed to serialize OCI spec during copy", err)),
		)
	}
	err = json.Unmarshal(raw, &result)
	if err != nil {
		return result, serum.Error(wfapi.ECodeInternal,
			serum.WithCause(wfapi.ErrorSerialization("failed to parse OCI spec during copy", err)),
		)
	}
	return result, nil
}

// Creates a configuration for runc based on the runConfig
//
// Errors:
//
//    - warpforge-error-io -- when file reads, writes, and dir creation fails
//    - warpforge-error-internal -- copying the base spec fails
func (cfg internalConfig) newRuncConfig(ctx context.Context, runPath string, baseSpec specs.Spec) (runcConfig, error) {
	rootWsIntPath := "/" + cfg.RootWs.InternalPath()
	rc := runcConfig{
		binPath:     cfg.ExecConfig.BinPath,
		runPath:     runPath,
		rootPath:    filepath.Join(rootWsIntPath, "runc-root"),
		cachePath:   filepath.Join(rootWsIntPath, "cache"),
		interactive: false,
	}
	_spec, err := copySpec(baseSpec)
	if err != nil {
		return rc, err
	}
	rc.spec = _spec

	// set up root -- this is not actually used since it is replaced with an overlayfs
	rootPath := filepath.Join(rc.runPath, "root")

	if err := os.MkdirAll(rootPath, 0755); err != nil {
		return rc, wfapi.ErrorIo("failed to create root directory", rootPath, err)
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
		Source:      rootWsIntPath,
		Destination: containerWorkspacePath(),
		Type:        "none",
		Options:     []string{"rbind"},
	}
	rc.spec.Mounts = append(rc.spec.Mounts, wfMount)

	// check if the rio warehouse location has been overridden
	// if so, mount it to the expected warehouse path
	if warehousePath, ok := cfg.warehousePathOverride(); ok {
		wfWarehouseMount := specs.Mount{
			Source:      warehousePath,
			Destination: containerWarehousePath(),
			Type:        "none",
			Options:     []string{"rbind"},
		}
		rc.spec.Mounts = append(rc.spec.Mounts, wfWarehouseMount)
	}

	wfBinMounts := getBinMounts(ctx, rc.binPath)
	rc.spec.Mounts = append(rc.spec.Mounts, wfBinMounts...)

	// check if /dev/tty exists
	rc.spec.Process.Terminal = false // default to disabled where no tty exists (e.g. github actions)
	if _, err := os.Open("/dev/tty"); err == nil {
		// enable a normal interactive terminal
		rc.spec.Process.Terminal = true
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

func wareCachePath(base string, wareId wfapi.WareID) string {
	packType := string(wareId.Packtype)
	hash := wareId.Hash
	return filepath.Join(base, packType, "fileset", hash[0:3], hash[3:6], hash)
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
func (rc *runcConfig) makeWareMount(ctx context.Context,
	wareId wfapi.WareID,
	dest string,
	context *wfapi.FormulaContext,
	filters wfapi.FilterMap,
) (specs.Mount, error) {
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
				mnt, err := rc.makeBindPathMount(ctx, hostPath, src, true)
				if err != nil {
					return mnt, err
				}
				rc.spec.Mounts = append(rc.spec.Mounts, mnt)

				// finally, add the protocol back on to the src string for rio
				src = fmt.Sprintf("%s://%s", proto, src)
			} else {
				// this is a network address, pass it to rio as is
				src = string(v)
				// HACK
				src = strings.Replace(src, "ca+s3", "ca+https", 1)
			}
		}
	}

	// unpacking may require fetching from a remote source, which may
	// require network access. since we do this in an empty container,
	// we need a resolv.conf for DNS configuration and /etc/ssl/certs
	// for trusted CAs
	rc.spec.Mounts = append(rc.spec.Mounts, getNetworkMounts()...)

	// convert FilterMap to rio string
	var filterStr string
	for name, value := range filters.Values {
		filterStr = fmt.Sprintf(",%s%s=%s", filterStr, name, value)
	}

	// perform a rio unpack with no placer. this will unpack the contents
	// to the RIO_CACHE dir and stop. we will then overlay mount the cache
	// dir when executing the formula.
	rc.spec.Process.Env = []string{"RIO_CACHE=" + containerCachePath()}
	rc.spec.Process.Args = []string{
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

	cacheWareId := wareId
	// check if the cached ware already exists
	expectCachePath := wareCachePath(rc.cachePath, wareId)
	if _, errRaw := os.Stat(expectCachePath); os.IsNotExist(errRaw) {
		// no cached ware, run the unpack
		outStr, err := rc.invokeRunc(ctx, nil)
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
		// UID/GID filters can mean the ware ID changes.
		wareIdSplit := strings.SplitN(out.Result.WareId, ":", 2)
		cacheWareId = wfapi.WareID{Packtype: wfapi.Packtype(wareIdSplit[0]), Hash: wareIdSplit[1]}
	}

	lowerdirPath := wareCachePath(rc.cachePath, cacheWareId)
	upperdirPath := filepath.Join(rc.runPath, "overlays", fmt.Sprintf("upper-%s", cacheWareId))
	workdirPath := filepath.Join(rc.runPath, "overlays", fmt.Sprintf("work-%s", cacheWareId))

	// create upper and work dirs
	errRaw := os.MkdirAll(upperdirPath, 0755)
	if errRaw != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of upperdir failed", upperdirPath, errRaw)
	}
	errRaw = os.MkdirAll(workdirPath, 0755)
	if errRaw != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of workdir failed", workdirPath, errRaw)
	}

	return specs.Mount{
		Destination: dest,
		Source:      "none",
		Type:        "overlay",
		Options: []string{
			"lowerdir=" + lowerdirPath,
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
func (rc *runcConfig) makeOverlayPathMount(ctx context.Context, path string, dest string) (specs.Mount, error) {
	mountId := strings.Replace(path, "/", "-", -1)
	mountId = strings.Replace(mountId, ".", "-", -1)
	upperdirPath := filepath.Join(rc.runPath, "overlays/upper-", mountId)
	workdirPath := filepath.Join(rc.runPath, "overlays/work-", mountId)

	// create upper and work dirs
	err := os.MkdirAll(upperdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of upperdir failed", upperdirPath, err)
	}
	err = os.MkdirAll(workdirPath, 0755)
	if err != nil {
		return specs.Mount{}, wfapi.ErrorIo("creation of workdir failed", workdirPath, err)
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
func (rc *runcConfig) makeBindPathMount(ctx context.Context, path string, dest string, readOnly bool) (specs.Mount, error) {
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
func (rc *runcConfig) invokeRunc(ctx context.Context, logWriter io.Writer) (string, error) {
	ctx, span := tracing.Start(ctx, "invokeRunc")
	defer span.End()
	rc.debug(ctx)
	configBytes, err := json.Marshal(rc.spec)
	if err != nil {
		return "", wfapi.ErrorExecutorFailed("runc", wfapi.ErrorSerialization("failed to serialize runc config", err))
	}

	bundlePath, err := ioutil.TempDir(rc.runPath, "bundle-")
	if err != nil {
		return "", wfapi.ErrorIo("creating bundle tmpdir", bundlePath, err)
	}
	configPath := filepath.Join(bundlePath, "config.json")
	err = ioutil.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		return "", wfapi.ErrorIo("writing config.json", configPath, err)
	}

	cmdCtx, cmdSpan := tracing.Start(ctx, "exec bundle", trace.WithAttributes(tracing.AttrFullExecNameRunc))
	defer cmdSpan.End()
	cmd := exec.CommandContext(cmdCtx, filepath.Join(rc.binPath, "runc"),
		"--root", rc.rootPath,
		"run",
		"-b", bundlePath, // bundle path
		fmt.Sprintf("warpforge-%d", time.Now().UTC().UnixNano()), // container id
	)

	// if the config has terminal enabled, and interactivity is requested,
	// wire stdin to the contaniner
	if rc.spec.Process.Terminal && rc.interactive {
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
	tracing.EndWithStatus(cmdSpan, err)
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
func (rc *runcConfig) rioPack(ctx context.Context, path string) (wfapi.WareID, error) {
	ctx, span := tracing.Start(ctx, "rioPack")
	defer span.End()
	rc.spec.Process.Args = []string{
		filepath.Join(containerBinPath(), "rio"),
		"pack",
		"--format=json",
		"--filters=uid=0,gid=0",
		"--target=ca+file://" + containerWarehousePath(),
		"tar",
		path,
	}

	outStr, err := rc.invokeRunc(ctx, nil)
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
			span.AddEvent("Found ware ID", trace.WithAttributes(attribute.String(tracing.AttrKeyWarpforgeWareId, out.Result.WareId)))
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

// Internal function for executing a formula
//
// Errors:
//
// - warpforge-error-io -- when an IO operation fails
// - warpforge-error-executor-failed -- when the execution step of the formula fails
// - warpforge-error-ware-unpack -- when a ware unpack operation fails for a formula input
// - warpforge-error-ware-pack -- when a ware pack operation fails for a formula output
// - warpforge-error-formula-invalid -- when an invalid formula is provided
// - warpforge-error-serialization -- when serialization or deserialization of a memo fails
// - warpforge-error-internal -- when copying the runc spec fails
func execFormula(ctx context.Context, cfg internalConfig) (wfapi.RunRecord, error) {
	logger := logging.Ctx(ctx)
	ctx, span := tracing.Start(ctx, "execFormula")
	defer span.End()
	rr := wfapi.RunRecord{}

	if cfg.FormulaAndContext.Formula.Formula == nil {
		return rr, wfapi.ErrorFormulaInvalid("no v1 Formula in FormulaCapsule")
	}
	formula := cfg.FormulaAndContext.Formula.Formula

	context := wfapi.FormulaContext{}
	if cfg.FormulaAndContext.Context != nil && cfg.FormulaAndContext.Context.FormulaContext != nil {
		context = *cfg.FormulaAndContext.Context.FormulaContext
	}

	// convert formula to node
	nFormula := bindnode.Wrap(cfg.FormulaAndContext.Formula.Formula, wfapi.TypeSystem.TypeByName("Formula"))

	// set up the runrecord result
	rr.Guid = uuid.New().String()
	rr.Time = time.Now().Unix()
	lsys := cidlink.DefaultLinkSystem()
	lnk, errRaw := lsys.ComputeLink(cidlink.LinkPrototype{Prefix: cid.Prefix{
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
	span.SetAttributes(attribute.String(tracing.AttrKeyWarpforgeFormulaId, fid))
	logger.Info(LOG_TAG_START, "")

	if memo, err := cfg.loadMemo(ctx, fid); err != nil {
		return rr, err
	} else if memo != nil {
		logger.PrintRunRecord(LOG_TAG, *memo, true)
		logger.Info(LOG_TAG_END, "")
		return *memo, nil
	}

	formulaSerial, errRaw := ipld.Marshal(ipldjson.Encode, formula, wfapi.TypeSystem.TypeByName("Formula"))
	if errRaw != nil {
		return rr, wfapi.ErrorFormulaInvalid(fmt.Sprintf("failed to re-serialize formula: %s", errRaw))
	}
	logger.Debug(LOG_TAG, "resolved formula:")
	logger.Debug(LOG_TAG, string(formulaSerial))

	// ensure a warehouse dir exists within the root workspace
	warehousePath := filepath.Join("/", cfg.RootWs.WarehousePath())
	errRaw = os.MkdirAll(warehousePath, 0755)
	if errRaw != nil {
		return rr, wfapi.ErrorIo("failed to create warehouse", warehousePath, errRaw)
	}

	// each formula execution gets a unique run directory
	// this is used to store working files and is destroyed upon completion
	runPath, errRaw := ioutil.TempDir(cfg.RunPathBase, DefaultRunPathPrefix)
	if errRaw != nil {
		return rr, wfapi.ErrorIo("failed to create temp run directory", cfg.RunPathBase, errRaw)
	}
	if !cfg.KeepRunDir {
		defer os.RemoveAll(runPath)
		logger.Debug(LOG_TAG, "using rundir %q", runPath)
	} else {
		logger.Info(LOG_TAG, "using rundir %q", runPath)
	}
	cfg.debug(ctx)

	baseSpec, err := newRuncSpec(ctx, runPath, cfg.BinPath)
	if err != nil {
		return rr, err
	}

	// get our configuration for the exec step
	// this config will collect the various inputs (mounts and vars) as each is set up
	execConfig, err := cfg.newRuncConfig(ctx, runPath, baseSpec)
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
			tmpConfig, err := cfg.newRuncConfig(ctx, runPath, baseSpec)
			if err != nil {
				return rr, err
			}

			// determine the host path for mount types
			var hostPath string
			if inputSimple.Mount != nil {
				if filepath.IsAbs(inputSimple.Mount.HostPath) {
					hostPath = inputSimple.Mount.HostPath
				} else {
					// otherwise, use relative path
					hostPath = filepath.Join(cfg.FormulaDirectory, inputSimple.Mount.HostPath)
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
				mnt, err = tmpConfig.makeOverlayPathMount(ctx, hostPath, destPath)
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
				mnt, err = tmpConfig.makeBindPathMount(ctx, hostPath, destPath, true)
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
				mnt, err = tmpConfig.makeBindPathMount(ctx, hostPath, destPath, false)
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
				mnt, err = tmpConfig.makeWareMount(ctx, *inputSimple.WareID, destPath, &context, filters)
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
		execConfig.spec.Mounts = append(execConfig.spec.Mounts, getNetworkMounts()...)
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
			return rr, wfapi.ErrorIo("failed to create script dir", scriptPath, errRaw)
		}

		// open the script file
		scriptFilePath := filepath.Join(scriptPath, "run")
		scriptFile, errRaw := os.OpenFile(scriptFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if errRaw != nil {
			return rr, wfapi.ErrorIo("failed to open script file for writing", scriptFilePath, errRaw)
		}
		defer scriptFile.Close()

		// iterate over each item in script contents
		for n, entry := range formula.Action.Script.Contents {
			// open the entry file (entry-#)
			entryFilePath := filepath.Join(scriptPath, fmt.Sprintf("entry-%d", n))
			entryFile, err := os.OpenFile(entryFilePath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return rr, wfapi.ErrorIo("failed to open entry file for writing", entryFilePath, err)
			}
			defer entryFile.Close()

			// write the entry file
			_, err = entryFile.WriteString(entry + "\n")
			if err != nil {
				return rr, wfapi.ErrorIo("error writing entry file", entryFilePath, err)
			}

			// write a line to execute this entry into the main script file
			// we use the POSIX standard `. filename` to cause the entry file to be executed
			// within the current shell process (also known as `source` in bash)
			entrySrc := fmt.Sprintf(". %s\n",
				filepath.Join(containerScriptPath(), fmt.Sprintf("entry-%d", n)))
			_, err = scriptFile.WriteString(entrySrc)
			if err != nil {
				return rr, wfapi.ErrorIo("error writing entry to script file", scriptFilePath, err)
			}
		}

		// create a mount for the script file
		scriptMount, err := execConfig.makeBindPathMount(ctx, scriptPath, containerScriptPath(), false)
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

	// determine initeractivity output formatting.
	// if interactive, do not apply any special formatting and wire stdin to container
	// otherwiise, pretty-format the output and do not wire stdin
	var runcWriter io.Writer
	if cfg.FormulaExecConfig.Interactive {
		runcWriter = logger.RawWriter()
		execConfig.interactive = true
	} else {
		runcWriter = logger.OutputWriter(LOG_TAG_OUTPUT)
		execConfig.interactive = false
	}

	// run the action
	logger.Output(LOG_TAG_OUTPUT_START, "")
	_, err = execConfig.invokeRunc(ctx, runcWriter)
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
			wareId, err := execConfig.rioPack(ctx, path)
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

	if err := cfg.storeMemo(ctx, rr); err != nil {
		return rr, err
	}

	return rr, nil
}

// Execute a Formula using the provided root Workspace
//
// Errors:
//
//     - warpforge-error-executor-failed -- when the execution step of the formula fails
//     - warpforge-error-formula-execution-failed -- when an error occurs during formula execution
//     - warpforge-error-formula-invalid -- when an invalid formula is provided
//     - warpforge-error-serialization -- when serialization or deserialization of a memo fails
//     - warpforge-error-ware-pack -- when a ware pack operation fails for a formula output
//     - warpforge-error-ware-unpack -- when a ware unpack operation fails for a formula input
func Exec(ctx context.Context, cfg ExecConfig, root *workspace.Workspace, frmCtx wfapi.FormulaAndContext, frmCfg wfapi.FormulaExecConfig) (result wfapi.RunRecord, err error) {
	ctx, span := tracing.StartFn(ctx, "Exec")
	defer func() { tracing.EndWithStatus(span, err) }()
	icfg := internalConfig{
		ExecConfig:        cfg,
		RootWs:            root,
		FormulaAndContext: frmCtx,
		FormulaExecConfig: frmCfg,
	}
	rr, err := execFormula(ctx, icfg)
	if err != nil {
		switch serum.Code(err) {
		case "warpforge-error-io", "warpforge-error-internal":
			err := wfapi.ErrorFormulaExecutionFailed(err)
			return rr, err
		default:
			// Error Codes -= warpforge-error-io, warpforge-error-internal
			return rr, err
		}
	}
	return rr, nil
}
