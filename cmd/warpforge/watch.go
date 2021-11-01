package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/wfapi"
)

var watchCmdDef = cli.Command{
	Name:   "watch",
	Usage:  "Watch a directory for changes to ingests",
	Action: cmdWatch,
}

func cmdWatch(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("invalid args")
	}

	path := c.Args().First()

	plot, err := plotFromFile(filepath.Join(path, "plot.json"))
	if err != nil {
		return err
	}

	ingests := make(map[string]string)
	var allInputs []wfapi.PlotInput
	for _, input := range plot.Inputs.Values {
		allInputs = append(allInputs, input)
	}
	for _, step := range plot.Steps.Values {
		for _, input := range step.Protoformula.Inputs.Values {
			allInputs = append(allInputs, input)
		}
	}

	for _, input := range allInputs {
		if input.Basis().Ingest != nil && input.Basis().Ingest.GitIngest != nil {
			ingest := input.Basis().Ingest.GitIngest
			ingests[ingest.HostPath] = ingest.Ref
		}
	}

	fmt.Println(ingests)

	ingestCache := make(map[string]string)
	for k, v := range ingests {
		ingestCache[k] = v
	}

	for {
		for path, ref := range ingests {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			gitCmd := exec.Command(
				"git",
				"--git-dir",
				filepath.Join(path, ".git"),
				"rev-parse",
				ref)
			gitCmd.Stdout = &stdout
			gitCmd.Stderr = &stderr
			err = gitCmd.Run()
			if err != nil {
				return fmt.Errorf("git rev-parse failed: %s", stderr.String())
			}
			hash := strings.TrimSpace(stdout.String())

			if ingestCache[path] != hash {
				fmt.Println("path", path, "changed, new hash", hash)
				ingestCache[path] = hash
				execModule(c, filepath.Join(c.Args().First(), "module.json"))
			}
		}
		time.Sleep(time.Millisecond * 100)
	}
}
