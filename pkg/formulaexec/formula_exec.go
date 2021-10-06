package formulaexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	runPath string
	wsPath  string
}

func getBaseConfig(wsPath string, runPath string) (runConfig, error) {
	rc := runConfig{
		runPath: runPath,
		wsPath:  wsPath,
	}

	// generate a runc rootless config, then read the resulting config
	configFile := filepath.Join(runPath, "config.json")
	err := os.RemoveAll(configFile)
	if err != nil {
		return rc, fmt.Errorf("failed to remove config.json")
	}
	cmd := exec.Command(filepath.Join(wsPath, "bin/runc"),
		"spec",
		"--rootless",
		"-b", runPath)
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

	// mount warpforge directory into the container
	// TODO: this should probably be removed for the exec step,
	// only needed for pack/unpack
	wfMount := specs.Mount{
		Source:      wsPath,
		Destination: "/warpforge",
		Type:        "none",
		Options:     []string{"rbind"},
	}
	rc.spec.Mounts = append(rc.spec.Mounts, wfMount)

	// required for executing on systems without a tty (e.g., github actions)
	rc.spec.Process.Terminal = false

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
			log.Printf("using warehouse %s for ware %s", v, wareId)
			src = string(v)
		}
	}

	// unpacking may require fetching from a remote source, which may
	// require network access. since we do this in an empty container,
	// we need a resolv.conf for DNS configuration and /etc/ssl/certs
	// for trusted CAs

	// copy host's resolv.conf so we can mount it
	err := os.MkdirAll(filepath.Join(config.wsPath, "/etc"), 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to create etc directory: %s", err)
	}
	destResolv, err := os.Create(filepath.Join(config.wsPath, "etc/resolv.conf"))
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to create resolv.conf file: %s", err)
	}
	defer destResolv.Close()
	srcResolv, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to open host resolv.conf: %s", err)
	}
	defer srcResolv.Close()
	_, err = io.Copy(destResolv, srcResolv)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to copy resolv.conf: %s", err)
	}
	err = destResolv.Sync()
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to sync resolv.conf: %s", err)
	}

	// add mounts for resolv.conf and ssl certificates
	etcMount := specs.Mount{
		Source:      filepath.Join(config.wsPath, "/etc"),
		Destination: "/etc",
		Type:        "none",
		Options:     []string{"rbind"},
	}
	caMount := specs.Mount{
		Source:      "/etc/ssl/certs",
		Destination: "/etc/ssl/certs",
		Type:        "none",
		Options:     []string{"rbind", "readonly"},
	}
	config.spec.Mounts = append(config.spec.Mounts, etcMount)
	config.spec.Mounts = append(config.spec.Mounts, caMount)

	// convert FilterMap to rio string
	var filterStr string
	for name, value := range filters.Values {
		filterStr = fmt.Sprintf(",%s%s=%s", filterStr, name, value)
	}

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
	wareType := strings.SplitN(out.Result.WareId, ":", 2)[0]
	cacheWareId := strings.SplitN(out.Result.WareId, ":", 2)[1]

	cachePath := filepath.Join(config.wsPath, "cache", wareType, "fileset", cacheWareId[0:3], cacheWareId[3:6], cacheWareId)
	upperdirPath := filepath.Join(config.runPath, "overlays", fmt.Sprintf("upper-%s", cacheWareId))
	workdirPath := filepath.Join(config.runPath, "overlays", fmt.Sprintf("work-%s", cacheWareId))

	// create upper and work dirs
	err = os.MkdirAll(upperdirPath, 0755)
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

func makePathMount(config runConfig, path string, dest string) (specs.Mount, error) {
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

	cmd := exec.Command(filepath.Join(config.wsPath, "bin", "runc"),
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

	// get the workspace location
	_, wsPath := ws.Path()
	wsPath = filepath.Join("/", wsPath, ".warpforge")

	// each formula execution gets a unique run directory
	// this is used to store working files and is destroyed upon completion
	runPath, err := ioutil.TempDir(os.TempDir(), "run-")
	if err != nil {
		return rr, fmt.Errorf("failed to create temp run directory: %s", err)
	}

	_, keep := os.LookupEnv("WARPFORGE_KEEP_RUNDIR")
	if !keep {
		defer os.RemoveAll(runPath)
	}

	// get our configuration for the exec step
	// this config will collect the various input mounts as each is set up
	execConfig, err := getBaseConfig(wsPath, runPath)
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
			// TODO deal with complex input fields
			log.Println("WARNING: ignoring complex input (not supported)")
			filters = src.FormulaInputComplex.Filters
		} else {
			return rr, fmt.Errorf("invalid formula input for %s", *dest.SandboxPath)
		}

		var mnt specs.Mount
		// create a temporary config for setting up each mount
		config, err := getBaseConfig(wsPath, runPath)
		if err != nil {
			return rr, err
		}
		if input.Mount != nil {
			switch input.Mount.Mode {
			case "overlay":
				// mount uses relative path
				mnt, err = makePathMount(config, filepath.Join(pwd, input.Mount.HostPath), filepath.Join("/", string(*dest.SandboxPath)))
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

	// set up the runrecord result
	rr.Guid = uuid.New().String()
	rr.Time = time.Now().Unix()
	nFormula, err := bindnode.Wrap(&fc, wfapi.TypeSystem.TypeByName("FormulaAndContext")).LookupByString("formula")
	if err != nil {
		return rr, err
	}
	lsys := cidlink.DefaultLinkSystem()
	lnk, err := lsys.ComputeLink(cidlink.LinkPrototype{cid.Prefix{
		Version:  1,    // Usually '1'.
		Codec:    0x71, // 0x71 means "dag-cbor" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhType:   0x13, // 0x13 means "sha2-512" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhLength: 64,   // sha2-512 hash has a 64-byte sum.
	}}, nFormula.(schema.TypedNode).Representation())
	if err != nil {
		return rr, err
	}
	rr.FormulaID = lnk.String()

	// run the exec action
	switch {
	case formula.Action.Exec != nil:
		execConfig.spec.Process.Args = formula.Action.Exec.Command
		execConfig.spec.Process.Cwd = "/"
		out, err := invokeRunc(execConfig)
		if err != nil {
			return rr, fmt.Errorf("invoke runc for exec failed: %s", err)
		}
		// TODO exit code?
		rr.Exitcode = 0
		fmt.Printf("%s\n", out)
	default:
		// TODO handle other actions
		log.Fatal("unsupported action")
	}

	// collect outputs
	rr.Results.Values = make(map[wfapi.OutputName]wfapi.FormulaInputSimple)
	for name, gather := range formula.Outputs.Values {
		switch {
		case gather.From.SandboxPath != nil:
			path := string(*gather.From.SandboxPath)
			wareId, err := rioPack(execConfig, path)
			if err != nil {
				return rr, err
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

	return rr, nil
}
