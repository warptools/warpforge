package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/wfapi"
)

var watchCmdDef = cli.Command{
	Name:   "watch",
	Usage:  "Watch a directory for git commits, executing plot on each new commit",
	Action: cmdWatch,
}

func cmdWatch(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("invalid args")
	}

	path := c.Args().First()

	// TODO: currently we read the module/plot from the provided path.
	// instead, we should read it from the git cache dir
	plot, err := plotFromFile(filepath.Join(path, PLOT_FILE_NAME))
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

	ingestCache := make(map[string]string)
	for k, v := range ingests {
		ingestCache[k] = v
	}

	for {
		for path, rev := range ingests {
			r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
				URL: "file://" + path,
			})
			if err != nil {
				return fmt.Errorf("failed to checkout git repository at %q to memory: %s", path, err)
			}

			hashBytes, err := r.ResolveRevision(plumbing.Revision(rev))
			if err != nil {
				return fmt.Errorf("failed to resolve git hash: %s", err)
			}
			hash := hashBytes.String()

			if ingestCache[path] != hash {
				fmt.Println("path", path, "changed, new hash", hash)
				ingestCache[path] = hash
				_, err := execModule(c, filepath.Join(c.Args().First(), MODULE_FILE_NAME))
				if err != nil {
					fmt.Printf("exec failed: %s\n", err)
				}
			}
		}
		time.Sleep(time.Millisecond * 100)
	}
}
