package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/goccy/go-graphviz"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/wfapi"
)

func cmdPlotGraph(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("a single input file must be provided")
	}

	f, err := ioutil.ReadFile(c.Args().First())
	if err != nil {
		return err
	}

	plot := wfapi.Plot{}
	_, err = ipld.Unmarshal([]byte(f), json.Decode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if fname := c.String("png"); fname != "" {
		err := plotexec.Graph(plot, graphviz.PNG, &buf)
		if err != nil {
			return err
		}
		os.WriteFile(fname, buf.Bytes(), 0644)
	} else {
		err := plotexec.Graph(plot, graphviz.XDOT, &buf)
		if err != nil {
			return err
		}
		fmt.Fprintln(c.App.Writer, buf.String())
	}

	return nil
}
