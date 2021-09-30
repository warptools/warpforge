package main

import (
	"fmt"

	"github.com/ipld/go-ipld-prime"
	"github.com/urfave/cli/v2"
)

var checkCmdDef = cli.Command{
	Name:   "check",
	Usage:  "Check file(s) for syntax and sanity.",
	Action: cmdCheck,
}

func cmdCheck(c *cli.Context) error {
	if !c.Args().Present() {
		return fmt.Errorf("no input files provided")
	}

	for _, fileName := range c.Args().Slice() {
		t, err := getFileType(fileName)
		if err != nil {
			return err
		}

		var n *ipld.Node
		switch t {
		case "formula":
			n, err = checkFormula(fileName)
			if err != nil {
				return fmt.Errorf("%s: %s", fileName, err)
			}
		case "plot":
			n, err = checkPlot(fileName)
			if err != nil {
				return fmt.Errorf("%s: %s", fileName, err)
			}
		case "module":
			n, err = checkModule(fileName)
			if err != nil {
				return fmt.Errorf("%s: %s", fileName, err)
			}
		default:
			if c.Bool("verbose") {
				fmt.Fprintf(c.App.ErrWriter, "ignoring unrecoginzed file: %q\n", fileName)
			}
		}
		if c.Bool("verbose") && n != nil {
			c.App.Metadata["result"] = *n
		}
	}

	return nil
}
