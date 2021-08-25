package formulaexec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
	"path"

	"github.com/google/uuid"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	ipldjson "github.com/ipld/go-ipld-prime/codec/json"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/bindnode"
	"github.com/ipld/go-ipld-prime/schema"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/warpfork/warpforge/wfapi"
)

type RioResult struct {
	WareId string `json:"wareID"`
}
type RioOutput struct {
	Result RioResult `json:"result"`
}

func warpforge_dir() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return path.Join(homedir, ".warpforge")
}

func warpforge_run_dir() string {
	return path.Join(warpforge_dir() + "/run")
}

func get_base_config(rootdir string) (specs.Spec, error) {
	s := specs.Spec{}

	// generate a runc rootless config, then read the resulting config
	err := os.RemoveAll("config.json")
	if err != nil {
		return s, fmt.Errorf("failed to delete existing runc config: %s", err)
	}
	cmd := exec.Command(warpforge_dir()+"/bin/runc", "spec", "--rootless")
	err = cmd.Run()
	if err != nil {
		return s, fmt.Errorf("failed to generate runc config: %s", err)
	}
	config_file, err := ioutil.ReadFile("config.json")
	if err != nil {
		return s, fmt.Errorf("failed to read runc config: %s", err)
	}
	err = json.Unmarshal([]byte(config_file), &s)
	if err != nil {
		return s, fmt.Errorf("failed to parse runc config: %s", err)
	}

	// set up root -- this is not actually used since it is replaced with an overlayfs
	root := specs.Root{
		Path: rootdir,
		// if root is readonly, the overlayfs mount at '/' will also be readonly
		// force as rw for now
		Readonly: false,
	}
	s.Root = &root

	// mount warpforge directory into the container
	// TODO: this should probably be removed for the exec step,
	// only needed for pack/unpack
	wf_mount := specs.Mount{
		Source:      warpforge_dir(),
		Destination: "/warpforge",
		Type:        "none",
		Options:     []string{"rbind"},
	}
	s.Mounts = append(s.Mounts, wf_mount)

	return s, nil
}

func make_ware_mount(ware_id string, dest string, context *wfapi.FormulaContext) (specs.Mount, error) {
	rootdir, err := ioutil.TempDir(warpforge_run_dir(), "exec")
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to create exec directory: %s", err)
	}
	defer os.RemoveAll(rootdir)

	s, err := get_base_config(rootdir)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to get base runc config: %s", err)
	}

	// default warehouse to unpack from
	src := "ca+file:///warpforge/warehouse"
	// check to see if this ware should be fetched from a different warehouse
	for k, v := range context.Warehouses.Values {
		if k.String() == ware_id {
			log.Printf("using warehouse %s for ware %s", v, ware_id)
			src = string(v)
		}
	}

	// unpacking may require fetching from a remote source, which may
	// require network access. since we do this in an empty container,
	// we need a resolv.conf for DNS configuration and /etc/ssl/certs
	// for trusted CAs

	// copy host's resolv.conf so we can mount it
	err = os.MkdirAll(warpforge_dir()+"/etc", 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to create etc directory: %s", err)
	}
	dest_resolv, err := os.Create(warpforge_dir() + "/etc/resolv.conf")
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to create resolv.conf file: %s", err)
	}
	defer dest_resolv.Close()
	src_resolv, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to open host resolv.conf: %s", err)
	}
	defer src_resolv.Close()
	_, err = io.Copy(dest_resolv, src_resolv)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to copy resolv.conf: %s", err)
	}
	err = dest_resolv.Sync()
	if err != nil {
		return specs.Mount{}, fmt.Errorf("failed to sync resolv.conf: %s", err)
	}

	// add mounts for resolv.conf and ssl certificates
	etc_mount := specs.Mount{
		Source:      warpforge_dir() + "/etc",
		Destination: "/etc",
		Type:        "none",
		Options:     []string{"rbind"},
	}
	ca_mount := specs.Mount{
		Source:      "/etc/ssl/certs",
		Destination: "/etc/ssl/certs",
		Type:        "none",
		Options:     []string{"rbind", "readonly"},
	}
	s.Mounts = append(s.Mounts, etc_mount)
	s.Mounts = append(s.Mounts, ca_mount)

	s.Process.Env = []string{"RIO_CACHE=/warpforge/cache"}
	s.Process.Args = []string{
		"/warpforge/bin/rio",
		"unpack",
		fmt.Sprintf("--source=%s", src),
		// force uid and gid to zero since these are the values in the container
		// note that the resulting hash used for placing this in the cache dir
		// will end up being different if a tar doesn't only use uid/gid 0!
		// these *must* be zero due to runc issue 1800, otherwise we would
		// choose a more sane value
		"--filters=uid=0,gid=0,mtime=follow",
		"--placer=none",
		"--format=json",
		ware_id,
		"/null",
	}

	log.Println("invoking runc for rio unpack", ware_id)
	out_str, err := invoke_runc(s)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("invoke runc for rio unpack of %s failed: %s", ware_id, err)
	}

	out := RioOutput{}
	for _, line := range strings.Split(out_str, "\n") {
		err := json.Unmarshal([]byte(line), &out)
		if err != nil {
			log.Fatal(err)
		}
		if out.Result.WareId != "" {
			// found ware_id
			break
		}
	}
	if out.Result.WareId == "" {
		log.Fatal("no ware_id output from rio!")
	}
	ware_type := strings.SplitN(out.Result.WareId, ":", 2)[0]
	cache_ware_id := strings.SplitN(out.Result.WareId, ":", 2)[1]

	cache_path := warpforge_dir() + "/cache/" + ware_type + "/fileset/" + cache_ware_id[0:3] + "/" + cache_ware_id[3:6] + "/" + cache_ware_id
	upperdir_path := warpforge_dir() + "/overlays/upper-" + cache_ware_id
	workdir_path := warpforge_dir() + "/overlays/work-" + cache_ware_id

	// remove then (re)create upper and work dirs
	err = os.RemoveAll(upperdir_path)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("removal of upperdir failed: %s", err)
	}
	err = os.MkdirAll(upperdir_path, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of upperdir failed: %s", err)
	}
	err = os.RemoveAll(workdir_path)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("removal of workdir failed: %s", err)
	}
	err = os.MkdirAll(workdir_path, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of workdir failed: %s", err)
	}

	return specs.Mount{
		Destination: dest,
		Source:      "none",
		Type:        "overlay",
		Options: []string{
			"lowerdir=" + cache_path,
			"upperdir=" + upperdir_path,
			"workdir=" + workdir_path,
		},
	}, nil
}

func make_path_mount(path string, dest string) (specs.Mount, error) {
	mount_id := strings.Replace(path, "/", "-", -1)
	mount_id = strings.Replace(mount_id, ".", "-", -1)
	upperdir_path := warpforge_dir() + "/overlays/upper-" + mount_id
	workdir_path := warpforge_dir() + "/overlays/work-" + mount_id

	// remove then (re)create upper and work dirs
	err := os.RemoveAll(upperdir_path)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("removal of upperdir failed: %s", err)
	}
	err = os.MkdirAll(upperdir_path, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of upperdir failed: %s", err)
	}
	err = os.RemoveAll(workdir_path)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("removal of workdir failed: %s", err)
	}
	err = os.MkdirAll(workdir_path, 0755)
	if err != nil {
		return specs.Mount{}, fmt.Errorf("creation of workdir failed: %s", err)
	}

	return specs.Mount{
		Destination: dest,
		Source:      "none",
		Type:        "overlay",
		Options: []string{
			"lowerdir=" + path,
			"upperdir=" + upperdir_path,
			"workdir=" + workdir_path,
		},
	}, nil
}

func invoke_runc(s specs.Spec) (string, error) {
	config, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	err = ioutil.WriteFile("config.json", config, 0644)

	cmd := exec.Command(warpforge_dir()+"/bin/runc", "run", "container-id")
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

func rio_pack(s specs.Spec, path string) (wfapi.WareID, error) {
	s.Process.Args = []string{
		"/warpforge/bin/rio",
		"pack",
		"--format=json",
		"--target=ca+file:///warpforge/warehouse",
		"tar",
		path,
	}

	log.Println("invoking runc for pack", path)
	out_str, err := invoke_runc(s)
	if err != nil {
		return wfapi.WareID{}, fmt.Errorf("invoke runc for rio pack of %s failed: %s", path, err)
	}

	out := RioOutput{}
	for _, line := range strings.Split(out_str, "\n") {
		err := json.Unmarshal([]byte(line), &out)
		if err != nil {
			log.Fatal(err)
		}
		if out.Result.WareId != "" {
			// found ware_id
			break
		}
	}
	if out.Result.WareId == "" {
		log.Fatal("no ware_id output from rio!")
	}

	ware_id := wfapi.WareID{}
	ware_id.Packtype = wfapi.Packtype(strings.Split(out.Result.WareId, ":")[0])
	ware_id.Hash = strings.Split(out.Result.WareId, ":")[1]
	return ware_id, nil
}

func Exec(fc wfapi.FormulaAndContext) error {
	formula := fc.Formula
	context := fc.Context

	// get the directory this executable is in for relative paths
	exec_dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working dir: %s", err)
	}

	// create and cd to working dir
	err = os.MkdirAll(warpforge_run_dir(), 0755)
	if err != nil {
		return errors.New("failed to create run dir")
	}
	err = os.Chdir(warpforge_run_dir())
	if err != nil {
		return errors.New("failed to cd to run dir")
	}

	rootdir, err := ioutil.TempDir(warpforge_run_dir(), "exec")
	if err != nil {
		return fmt.Errorf("failed to create temp root directory: %s", err)
	}
	defer os.RemoveAll(rootdir)
	s, err := get_base_config(rootdir)
	if err != nil {
		return err
	}

	for dest, src := range formula.Inputs.Values {
		var input *wfapi.FormulaInputSimple
		if src.FormulaInputSimple != nil {
			input = src.FormulaInputSimple
		} else if src.FormulaInputComplex != nil {
			input = &src.FormulaInputComplex.Basis
			// TODO deal with complex input fields
			log.Println("WARNING: ignoring complex input (not supported)")
		} else {
			return errors.New(fmt.Sprintf("invalid input spec for dest %s", *dest.SandboxPath))
		}

		var mnt specs.Mount
		if input.Mount != nil {
			// TODO do something with Mount.Mode
			log.Println("WARNING: mount mode is currently ignored, all mounts are overlay")
			// mount uses relative path
			mnt, err = make_path_mount(path.Join(exec_dir, input.Mount.HostPath), "/"+string(*dest.SandboxPath))
			if err != nil {
				return err
			}
		} else if input.WareID != nil {
			mnt, err = make_ware_mount(input.WareID.String(), "/"+string(*dest.SandboxPath), context)
			if err != nil {
				return err
			}
		}

		// root mount must come first
		// leading slash is removed during parsing, so `"/"` will result in `""`
		if *dest.SandboxPath == wfapi.SandboxPath("") {
			s.Mounts = append([]specs.Mount{mnt}, s.Mounts...)
		} else {
			s.Mounts = append(s.Mounts, mnt)
		}
	}

	rr := wfapi.RunRecord{}
	rr.Guid = uuid.New().String()
	rr.Time = time.Now().Unix()
	nFormula, err := bindnode.Wrap(&fc, wfapi.TypeSystem.TypeByName("FormulaAndContext")).LookupByString("formula")
	if err != nil {
		return err
	}
	lsys := cidlink.DefaultLinkSystem()
	lnk, err := lsys.ComputeLink(cidlink.LinkPrototype{cid.Prefix{
		Version:  1,    // Usually '1'.
		Codec:    0x71, // 0x71 means "dag-cbor" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhType:   0x13, // 0x13 means "sha2-512" -- See the multicodecs table: https://github.com/multiformats/multicodec/
		MhLength: 64,   // sha2-512 hash has a 64-byte sum.
	}}, nFormula.(schema.TypedNode).Representation())
	if err != nil {
		return err
	}
	rr.FormulaID = lnk.String()

	// run the exec action
	switch {
	case formula.Action.Exec != nil:
		s.Process.Args = formula.Action.Exec.Command
		s.Process.Cwd = "/"
		log.Println("invoking runc for exec")
		out, err := invoke_runc(s)
		if err != nil {
			return fmt.Errorf("invoke runc for exec failed: %s", err)
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
			ware_id, err := rio_pack(s, path)
			if err != nil {
				return err
			}
			rr.Results.Keys = append(rr.Results.Keys, name)
			rr.Results.Values[name] = wfapi.FormulaInputSimple{WareID: &ware_id}
			log.Println("packed", name, "(", path, "->", ware_id, ")")
		case gather.From.SandboxVar != nil:
			log.Fatal("unsupported output type")
		default:
			log.Fatal("invalid output spec")
		}
	}
	serial, err := ipld.Marshal(ipldjson.Encode, &rr, wfapi.TypeSystem.TypeByName("RunRecord"))
	fmt.Println(string(serial))

	// cd back to original directory
	err = os.Chdir(exec_dir)
	if err != nil {
		return errors.New("failed to cd to run dir")
	}

	return nil
}
