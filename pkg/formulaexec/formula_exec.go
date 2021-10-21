package formulaexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

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

func getNetworkMounts(wsPath string) []specs.Mount {
	// some operations require network access, which requires some configuration
	// we provide a resolv.conf for DNS configuration and /etc/ssl/certs
	// for trusted CAs from the host system
	etcMount := specs.Mount{
		Source:      "/etc/resolv.conf",
		Destination: "/etc/resolv.conf",
		Type:        "none",
		Options:     []string{"rbind", "readonly"},
	}
	caMount := specs.Mount{
		Source:      "/etc/ssl/certs",
		Destination: "/etc/ssl/certs",
		Type:        "none",
		Options:     []string{"rbind", "readonly"},
	}
	return []specs.Mount{etcMount, caMount}
}

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
		return rc, fmt.Errorf("failed to remove config.json")
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
		return rc, fmt.Errorf("failed to generate runc config: %s", err)
	}
	configFileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return rc, fmt.Errorf("failed to read runc config: %s", err)
	}
	err = json.Unmarshal(configFileBytes, &rc.spec)
	if err != nil {
		return rc, fmt.Errorf("failed to parse runc config: %s", err)
	}

	// set up root -- this is not actually used since it is replaced with an overlayfs
	rootPath := filepath.Join(runPath, "root")
	err = os.MkdirAll(rootPath, 0755)
	if err != nil {
		return rc, fmt.Errorf("failed to create root directory: %s", err)
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
		Destination: "/warpforge",
		Type:        "none",
		Options:     []string{"rbind"},
	}
	rc.spec.Mounts = append(rc.spec.Mounts, wfMount)
	wfBinMount := specs.Mount{
		Source:      binPath,
		Destination: "/warpforge/bin",
		Type:        "none",
		Options:     []string{"rbind"},
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

func makeWareMount(config runConfig,
	wareId string,
	dest string,
	context *wfapi.FormulaContext,
	filters wfapi.FilterMap) (specs.Mount, error) {
	// default warehouse to unpack from
	src := "ca+file:///warpforge/warehouse"
	// check to see if this ware should be fetched from a different warehouse
	for k, v := range context.Warehouses.Values {
		if k.String() == wareId {
			fmt.Printf("using warehouse %q for ware %q\n", v, wareId)
			src = string(v)
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
	config.spec.Process.Env = []string{"RIO_CACHE=/warpforge/cache"}
	config.spec.Process.Args = []string{
		"/warpforge/bin/rio",
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
		wareId,
		"/null",
	}

	var wareType string
	var cacheWareId string
	// check if the cached ware already exists
	expectCachePath := fmt.Sprintf("cache/%s/fileset/%s/%s/%s", "tar", wareId[4:7], wareId[7:10], wareId[4:])
	if _, err := os.Stat(filepath.Join(config.wsPath, expectCachePath)); os.IsNotExist(err) {
		// no cached ware, run the unpack
		outStr, err := invokeRunc(config)
		if err != nil {
			return specs.Mount{}, fmt.Errorf("invoke runc for rio unpack of %s failed: %s", wareId, err)
		}
		out := RioOutput{}
		for _, line := range strings.Split(outStr, "\n") {
			err := json.Unmarshal([]byte(line), &out)
			if err != nil {
				log.Fatal(err)
			}
			if out.Result.WareId != "" {
				// found wareId
				break
			}
		}
		if out.Result.WareId == "" {
			return specs.Mount{}, fmt.Errorf("no wareId output from rio when unpacking %s", wareId)
		}
		wareType = strings.SplitN(out.Result.WareId, ":", 2)[0]
		cacheWareId = strings.SplitN(out.Result.WareId, ":", 2)[1]
	} else {
		// use cached ware
		fmt.Printf("ware %q already in cache\n", wareId)
		wareType = strings.SplitN(wareId, ":", 2)[0]
		cacheWareId = strings.SplitN(wareId, ":", 2)[1]
	}

	cachePath := filepath.Join(config.wsPath, "cache", wareType, "fileset", cacheWareId[0:3], cacheWareId[3:6], cacheWareId)
	upperdirPath := filepath.Join(config.runPath, "overlays", fmt.Sprintf("upper-%s", cacheWareId))
	workdirPath := filepath.Join(config.runPath, "overlays", fmt.Sprintf("work-%s", cacheWareId))

	// create upper and work dirs
	err := os.MkdirAll(upperdirPath, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of upperdir failed: %s", err)
	}
	err = os.MkdirAll(workdirPath, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of workdir failed: %s", err)
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

func makeOverlayPathMount(config runConfig, path string, dest string) (specs.Mount, error) {
	mountId := strings.Replace(path, "/", "-", -1)
	mountId = strings.Replace(mountId, ".", "-", -1)
	upperdirPath := filepath.Join(config.runPath, "overlays/upper-", mountId)
	workdirPath := filepath.Join(config.runPath, "overlays/work-", mountId)

	// create upper and work dirs
	err := os.MkdirAll(upperdirPath, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of upperdir failed: %s", err)
	}
	err = os.MkdirAll(workdirPath, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of workdir failed: %s", err)
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

func makeBindPathMount(config runConfig, path string, dest string) (specs.Mount, error) {
	return specs.Mount{
		Source:      path,
		Destination: dest,
		Type:        "none",
		Options:     []string{"rbind", "readonly"},
	}, nil
}

func invokeRunc(config runConfig) (string, error) {
	configBytes, err := json.Marshal(config.spec)
	if err != nil {
		return "", err
	}
	bundlePath, err := ioutil.TempDir(config.runPath, "bundle-")
	if err != nil {
		return "", fmt.Errorf("failed to create bundle directory: %s", err)
	}
	err = ioutil.WriteFile(filepath.Join(bundlePath, "config.json"), configBytes, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write config.json: %s", err)
	}

	cmd := exec.Command(filepath.Join(config.binPath, "runc"),
		"--root", filepath.Join(config.wsPath, "runc-root"),
		"run",
		"-b", bundlePath, // bundle path
		fmt.Sprintf("warpforge-%d", time.Now().UTC().UnixNano()), // container id
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s: %s %s", err, stdout.String(), stderr.String())
	}
	// TODO what exitcode do we care about?
	return stdout.String(), nil
}

func rioPack(config runConfig, path string) (wfapi.WareID, error) {
	config.spec.Process.Args = []string{
		"/warpforge/bin/rio",
		"pack",
		"--format=json",
		"--target=ca+file:///warpforge/warehouse",
		"tar",
		path,
	}

	outStr, err := invokeRunc(config)
	if err != nil {
		return wfapi.WareID{}, fmt.Errorf("invoke runc for rio pack of %s failed: %s", path, err)
	}

	out := RioOutput{}
	for _, line := range strings.Split(outStr, "\n") {
		err := json.Unmarshal([]byte(line), &out)
		if err != nil {
			log.Fatal(err)
		}
		if out.Result.WareId != "" {
			// found wareId
			break
		}
	}
	if out.Result.WareId == "" {
		log.Fatal("no wareId output from rio!")
	}

	wareId := wfapi.WareID{}
	wareId.Packtype = wfapi.Packtype(strings.Split(out.Result.WareId, ":")[0])
	wareId.Hash = strings.Split(out.Result.WareId, ":")[1]
	return wareId, nil
}

func Exec(ws *workspace.Workspace, fc wfapi.FormulaAndContext) (wfapi.RunRecord, error) {
	formula := fc.Formula
	context := fc.Context
	rr := wfapi.RunRecord{}

	pwd, err := os.Getwd()
	if err != nil {
		return rr, fmt.Errorf("failed to get working dir: %s", err)
	}

	// get the home workspace location
	var wsPath string
	path, override := os.LookupEnv("WARPFORGE_HOME")
	if override {
		wsPath = path
	} else if ws != nil {
		_, wsPath = ws.Path()
	} else {
		return rr, fmt.Errorf("no home workspace was provided")
	}
	wsPath = filepath.Join("/", wsPath, ".warpforge")

	// ensure a warehouse dir exists within the home workspace
	err = os.MkdirAll(filepath.Join(wsPath, "warehouse"), 0755)
	if err != nil {
		return rr, fmt.Errorf("failed to create warehouse: %s", err)
	}

	// determine the path of the running executable
	// other binaries (runc, rio) will be located here as well
	var binPath string
	path, override = os.LookupEnv("WARPFORGE_PATH")
	if override {
		binPath = path
	} else {
		binPath, err = os.Executable()
		if err != nil {
			return rr, err
		}
		binPath = filepath.Dir(binPath)
	}

	// each formula execution gets a unique run directory
	// this is used to store working files and is destroyed upon completion
	runPath, err := ioutil.TempDir(os.TempDir(), "warpforge-run-")
	if err != nil {
		return rr, fmt.Errorf("failed to create temp run directory: %s", err)
	}

	_, keep := os.LookupEnv("WARPFORGE_KEEP_RUNDIR")
	if keep {
		fmt.Printf("using rundir %q\n", runPath)
	} else {
		defer os.RemoveAll(runPath)
	}

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
			return rr, fmt.Errorf("invalid formula input for %s", *dest.SandboxPath)
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
				mnt, err = makeOverlayPathMount(config, hostPath, destPath)
				if err != nil {
					return rr, err
				}
			case "bind":
				mnt, err = makeBindPathMount(config, hostPath, destPath)
				if err != nil {
					return rr, err
				}
			default:
				return rr, fmt.Errorf("unsupported mount mode %q", input.Mount.Mode)
			}
		} else if input.WareID != nil {
			mnt, err = makeWareMount(config, input.WareID.String(), filepath.Join("/", string(*dest.SandboxPath)), context, filters)
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
	} else {
		// create empty network namespace to disable network
		execConfig.spec.Linux.Namespaces = append(execConfig.spec.Linux.Namespaces,
			specs.LinuxNamespace{
				Type: "network",
				Path: "",
			})
	}

	// set up the runrecord result
	rr.Guid = uuid.New().String()
	rr.Time = time.Now().Unix()
	nFormula, err := bindnode.Wrap(&fc, wfapi.TypeSystem.TypeByName("FormulaAndContext")).LookupByString("formula")
	if err != nil {
		return rr, fmt.Errorf("failed to wrap formula: %s", err)
	}
	lsys := cidlink.DefaultLinkSystem()
	lnk, err := lsys.ComputeLink(cidlink.LinkPrototype{cid.Prefix{
		Version:  1,    // Usually '1'.
		Codec:    0x71, // 0x71 means "dag-cbor" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhType:   0x13, // 0x13 means "sha2-512" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhLength: 64,   // sha2-512 hash has a 64-byte sum.
	}}, nFormula.(schema.TypedNode).Representation())
	if err != nil {
		return rr, fmt.Errorf("failed to compute formula ID: %s", err)
	}
	rr.FormulaID = lnk.String()

	// configure the action
	switch {
	case formula.Action.Exec != nil:
		execConfig.spec.Process.Args = formula.Action.Exec.Command
		execConfig.spec.Process.Cwd = "/"
	case formula.Action.Script != nil:
		// the script action creates a seperate "entry" file for each element in the script contents
		// and creates a "run" file which executes these in order within the the current shell process.

		// create the script directory
		scriptPath := filepath.Join(runPath, "script")
		err = os.MkdirAll(scriptPath, 0755)
		if err != nil {
			return rr, fmt.Errorf("failed to create script dir %q: %s", scriptPath, err)
		}

		// open the script file
		scriptFilePath := filepath.Join(scriptPath, "run")
		scriptFile, err := os.OpenFile(scriptFilePath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return rr, fmt.Errorf("failed to open script file for writing: %s", scriptFilePath)
		}
		defer scriptFile.Close()

		// iterate over each item in script contents
		for n, entry := range formula.Action.Script.Contents {
			// open the entry file (entry-#)
			entryFilePath := filepath.Join(scriptPath, fmt.Sprintf("entry-%d", n))
			entryFile, err := os.OpenFile(entryFilePath, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return rr, fmt.Errorf("failed to open entry file %q for writing: %s", entryFilePath, scriptFilePath)
			}
			defer entryFile.Close()

			// write the entry file
			_, err = entryFile.WriteString(entry + "\n")
			if err != nil {
				return rr, fmt.Errorf("error writing entry file %q: %s", entryFilePath, err)
			}

			// write a line to execute this entry into the main script file
			// we use the POSIX standard `. filename` to cause the entry file to be executed
			// within the current shell process (also known as `source` in bash)
			_, err = scriptFile.WriteString(fmt.Sprintf(". /script/entry-%d", n) + "\n")
			if err != nil {
				return rr, fmt.Errorf("error writing script file: %s", err)
			}
		}

		// create a mount for the script file
		scriptMount, err := makeBindPathMount(execConfig, scriptPath, "/script")
		if err != nil {
			return rr, fmt.Errorf("failed to create script mount: %s", err)
		}
		execConfig.spec.Mounts = append(execConfig.spec.Mounts, scriptMount)

		// configure the process
		execConfig.spec.Process.Args = []string{formula.Action.Script.Interpreter, "/script/run"}
		execConfig.spec.Process.Cwd = "/"
	default:
		// TODO handle other actions
		log.Fatal("unsupported action")
	}

	// run the action
	out, err := invokeRunc(execConfig)
	if err != nil {
		return rr, fmt.Errorf("invoke runc for exec failed: %s", err)
	}
	// TODO exit code?
	rr.Exitcode = 0
	fmt.Printf("%s\n", out)

	// collect outputs
	rr.Results.Values = make(map[wfapi.OutputName]wfapi.FormulaInputSimple)
	for name, gather := range formula.Outputs.Values {
		switch {
		case gather.From.SandboxPath != nil:
			path := string(*gather.From.SandboxPath)
			wareId, err := rioPack(execConfig, path)
			if err != nil {
				return rr, fmt.Errorf("rio pack failed: %s", err)
			}
			rr.Results.Keys = append(rr.Results.Keys, name)
			rr.Results.Values[name] = wfapi.FormulaInputSimple{WareID: &wareId}
			log.Println("packed", name, "(", path, "->", wareId, ")")
		case gather.From.SandboxVar != nil:
			log.Fatal("unsupported output type")
		default:
			log.Fatal("invalid output spec")
		}
	}

	fmt.Println(rr)
	return rr, nil
}
