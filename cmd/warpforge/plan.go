package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/logging"
)

var planCmdDef = cli.Command{
	Name:  "plan",
	Usage: "Runs planning commands to generate inputs",
	Subcommands: []*cli.Command{
		{
			Name: "generate",
			Action: util.ChainCmdMiddleware(cmdPlanGenerate,
				util.CmdMiddlewareLogging,
				util.CmdMiddlewareTracingConfig,
				util.CmdMiddlewareTracingSpan,
			),
		},
	},
}

func cmdPlanGenerate(c *cli.Context) error {
	logger := logging.Ctx(c.Context)
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get current directory")
	}

	input := c.Args().First()

	results := map[string][]byte{}
	if !c.Args().Present() {
		// no args, generate on current directory
		results, err = util.GenerateDir(pwd)
	} else if filepath.Base(input) == "..." {
		// recursively generate plots
		results, err = util.GenerateDirRecusive(filepath.Dir(c.Args().First()))
	} else {
		// input is a file or directory to generate
		info, err := os.Stat(input)
		if err != nil {
			return err
		}
		if info.IsDir() {
			results, err = util.GenerateDir(input)
		} else {
			// this is a file, so put one item into our results map
			results[input], err = util.GenerateFile(input)
		}
	}

	if err != nil {
		return err
	}

	for path, bytes := range results {
		// determine the output path for this item
		// this is done by replacing the extension with .wf
		dir := filepath.Dir(path)
		fname := filepath.Base(path)
		fnameSplit := strings.Split(fname, ".")
		outputFile := filepath.Join(dir, fnameSplit[0]+".wf")

		logger.Debug("generate", fmt.Sprintf("%s -> %s", path, outputFile))
		os.WriteFile(outputFile, bytes, 0644)
	}

	return nil
}
