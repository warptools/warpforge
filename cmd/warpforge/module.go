package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/wfapi"
)

var moduleCmdDef = cli.Command{
	Name:  "module",
	Usage: "Subcommands that operate on modules.",
	Subcommands: []*cli.Command{
		{
			Name:   "check",
			Usage:  "Check module file(s) for syntax and sanity.",
			Action: cmdModuleCheck,
		},
		{
			Name:   "init",
			Usage:  "Create a templated module and plot file.",
			Action: cmdModuleInit,
		},
	},
}

func checkModule(fileName string) (*ipld.Node, error) {
	f, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	module := wfapi.Module{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &module, wfapi.TypeSystem.TypeByName("Module"))
	return &n, err
}

func cmdModuleCheck(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	for _, fileName := range c.Args().Slice() {
		n, err := checkModule(fileName)
		if err != nil {
			return err
		}
		if c.Bool("verbose") {
			c.App.Metadata["result"] = *n
		}
	}
	return nil
}

func cmdModuleInit(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("no module name provided")
	}

	_, err := os.Stat("module.json")
	if !os.IsNotExist(err) {
		return fmt.Errorf("module.json file already exists")
	}
	_, err = os.Stat("plot.json")
	if !os.IsNotExist(err) {
		return fmt.Errorf("plot.json file already exists")
	}

	moduleName := c.Args().First()

	module := wfapi.Module{
		Name: wfapi.ModuleName(moduleName),
	}
	moduleSerial, err := ipld.Marshal(json.Encode, &module, wfapi.TypeSystem.TypeByName("Module"))
	if err != nil {
		return fmt.Errorf("failed to serialize module")
	}
	os.WriteFile("module.json", moduleSerial, 0644)

	plot := wfapi.Plot{}
	plotSerial, err := ipld.Marshal(json.Encode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return fmt.Errorf("failed to serialize plot")
	}
	os.WriteFile("plot.json", plotSerial, 0644)

	return nil
}
