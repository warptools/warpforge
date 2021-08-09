package main

import (
	"bytes"
	"strings"
	"fmt"
	"log"
	"encoding/json"
	"io/ioutil"
	"github.com/opencontainers/runtime-spec/specs-go"
	//"github.com/davecgh/go-spew/spew"
	"os"
	"os/exec"
)


type RioResult struct {
	WareId string `json:"wareID"`
}
type RioOutput struct {
	Result RioResult `json:"result"`
}

type FormulaInput struct {
	Source string `json:"src"`
	Destination string `json:"dest"`
}
type FormulaExec struct {
	Args []string `json:"args"`

}
type FormulaOutput struct {
	Path string
}

type Formula struct {
	Inputs []FormulaInput `json:"inputs"`
	Exec FormulaExec `json:"exec"`
	Outputs []FormulaOutput `json:"outputs"`
}

func warpforge_dir() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return (homedir + "/.warpforge")
}

func warpforge_run_dir() string {
	return (warpforge_dir() + "/run")
}

func get_base_config() specs.Spec {
	_ = os.Remove("config.json")
	cmd := exec.Command("runc", "spec", "--rootless")
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
	config_file, err := ioutil.ReadFile("config.json")
	if err != nil {
		log.Fatal(err)
	}

	s := specs.Spec{}
	err = json.Unmarshal([]byte(config_file), &s)

	// set up fake root -- this is not actually used since it is replaced with an overlayfs
	_ = os.RemoveAll("/tmp/fakeroot")
	_ = os.Mkdir("/tmp/fakeroot", 0755)
	root := specs.Root{
		Path: "/tmp/fakeroot",
		// if root is readonly, the overlayfs mount at '/' will also be readonly
		// force as rw for now
		Readonly: false,
	}
	s.Root = &root

	wf_mount := specs.Mount{
		Source: warpforge_dir(),
		Destination: "/warpforge",
		Type: "none",
		Options: []string{"rbind"},
	}
	s.Mounts = append(s.Mounts, wf_mount)

	return s
}

func make_rio_mount(ware_id string, dest string) specs.Mount {
	s := get_base_config()

	/*
	proc := specs.Process{
		Env: []string{"RIO_CACHE=/warpforge/cache"},
		Args: []string{
			"/warpforge/bin/rio",
			"unpack",
			"--source=ca+file:///warpforge/warehouse",
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
		},
		Cwd: "/",
	}
	s.Process = &proc
	*/

	s.Process.Env = []string{"RIO_CACHE=/warpforge/cache"}
	s.Process.Args = []string{
		"/warpforge/bin/rio",
		"unpack",
		"--source=ca+file:///warpforge/warehouse",
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


	out_str := invoke_runc(s)
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

	_ = os.RemoveAll(upperdir_path)
	_ = os.MkdirAll(upperdir_path, 0755)
	_ = os.RemoveAll(workdir_path)
	_ = os.MkdirAll(workdir_path, 0755)
	return specs.Mount{
		Destination: dest,
		Source: "none",
		Type: "overlay",
		Options: []string{
			"lowerdir=" + cache_path,
			"upperdir=" + upperdir_path,
			"workdir=" + workdir_path,
		},
	}
}

func make_dir_mount(path string, dest string) specs.Mount {
	uid := strings.Replace(path, "/", "-", -1)
	upperdir_path := warpforge_dir() + "/overlays/upper-" + uid
	workdir_path := warpforge_dir() + "/overlays/work-" + uid

	_ = os.RemoveAll(upperdir_path)
	_ = os.MkdirAll(upperdir_path, 0755)
	_ = os.RemoveAll(workdir_path)
	_ = os.MkdirAll(workdir_path, 0755)
	return specs.Mount{
		Destination: dest,
		Source: "none",
		Type: "overlay",
		Options: []string{
			"lowerdir=" + path,
			"upperdir=" + upperdir_path,
			"workdir=" + workdir_path,
		},
	}
}

func invoke_runc(s specs.Spec) string {
	config, err := json.Marshal(s)
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile("config.json", config, 0644)

	cmd := exec.Command("runc", "run", "container-id")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err = cmd.Run()
	if err != nil {
		fmt.Printf("%s\n", stderr)
		log.Fatal(err)
	}
	return stdout.String()
}

func main() {
	// read formula from pwd
	formula_file, err := ioutil.ReadFile("formula.json")
	if err != nil {
		log.Fatal(err)
	}
	formula := Formula{}
	err = json.Unmarshal([]byte(formula_file), &formula)
	if err != nil {
		log.Fatal(err)
	}

	// create and cd to working dir
	_ = os.MkdirAll(warpforge_run_dir(), 0755)
	os.Chdir(warpforge_run_dir())

	s := get_base_config()

	for _, input := range formula.Inputs {
		var mnt specs.Mount
		if _, err := os.Stat(input.Source); !os.IsNotExist(err) {
			mnt = make_dir_mount(input.Source, input.Destination)
		} else {
			mnt = make_rio_mount(input.Source, input.Destination)
		}

		// root mount must come first
		if input.Destination == "/" {
			s.Mounts = append([]specs.Mount{mnt}, s.Mounts...)
		} else {
			s.Mounts = append(s.Mounts, mnt)
		}
	}

	proc := specs.Process{
		Args: formula.Exec.Args,
		Cwd: "/",
	}
	s.Process = &proc

	out := invoke_runc(s)
	fmt.Printf("%s\n", out)
}
