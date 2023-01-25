package main

import (
	"context"
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

func writePlanResults(ctx context.Context, results map[string][]byte) error {
	logger := logging.Ctx(ctx)
	for path, bytes := range results {
		// determine the output path for this item
		// this is done by replacing the extension with .wf
		dir := filepath.Dir(path)
		fname := filepath.Base(path)
		fnameSplit := strings.Split(fname, ".")
		outputFile := filepath.Join(dir, fnameSplit[0]+".wf")

		logger.Debug("generate", "%s -> %s", path, outputFile)
		os.WriteFile(outputFile, bytes, 0644)
		//TODO: handle error
	}
	return nil
}

func cmdPlanGenerate(c *cli.Context) error {

	if !c.Args().Present() {
		// no args, generate on current directory
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		results, err := util.GenerateDir(cwd)
		if err != nil {
			return err
		}
		return writePlanResults(c.Context, results)
	}
	input := c.Args().First()

	if filepath.Base(input) == "..." {
		// recursively generate plots
		results, err := util.GenerateDirRecusive(filepath.Dir(input))
		if err != nil {
			return err
		}
		return writePlanResults(c.Context, results)
	}

	// input is a file or directory to generate
	info, err := os.Stat(input)
	if err != nil {
		return err
	}
	if info.IsDir() {
		results, err := util.GenerateDir(input)
		if err != nil {
			return err
		}
		return writePlanResults(c.Context, results)
	}

	// this is a file, so put one item into our results map
	data, err := util.GenerateFile(input)
	if err != nil {
		return err
	}
	return writePlanResults(c.Context, map[string][]byte{input: data})
}
