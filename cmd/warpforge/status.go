package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/formulaexec"
	"github.com/warpfork/warpforge/wfapi"
)

var statusCmdDef = cli.Command{
	Name:    "status",
	Usage:   "Get status of workspaces and installation.",
	Action:  cmdStatus,
	Aliases: []string{"info"},
}

func cmdStatus(c *cli.Context) error {
	fmtBold := color.New(color.Bold)
	fmtWarning := color.New(color.FgHiRed, color.Bold)
	verbose := c.Bool("verbose")

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get current directory")
	}

	// display version
	if verbose {
		fmt.Fprintf(c.App.Writer, "Warpforge Version: %s\n\n", VERSION)
	}

	// check plugins
	pluginsOk := true

	if verbose {
		fmt.Fprintf(c.App.Writer, "\nPlugin Info:\n")
	}

	binPath, err := formulaexec.GetBinPath()
	if err != nil {
		return fmt.Errorf("could not get binPath: %s", err)
	}
	if verbose {
		fmt.Fprintf(c.App.Writer, "binPath = %s\n", binPath)
	}

	rioPath := filepath.Join(binPath, "rio")
	if _, err := os.Stat(rioPath); os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, "rio not found (expected at %s)\n", rioPath)
		pluginsOk = false
	} else {
		if verbose {
			fmt.Fprintf(c.App.Writer, "found rio\n")
		}
	}

	runcPath := filepath.Join(binPath, "runc")
	if _, err := os.Stat(runcPath); os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, "runc not found (expected at %s)\n", runcPath)
		pluginsOk = false
	} else {
		runcVersionCmd := exec.Command(filepath.Join(binPath, "runc"), "--version")
		var runcVersionOut bytes.Buffer
		runcVersionCmd.Stdout = &runcVersionOut
		err = runcVersionCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to get runc version information: %s", err)
		}
		if verbose {
			fmt.Fprintf(c.App.Writer, "found runc\n")
			fmt.Fprintf(c.App.Writer, "%s", &runcVersionOut)
		}
	}

	if !pluginsOk {
		fmtWarning.Fprintf(c.App.Writer, "WARNING: plugins do not appear to be installed correctly.\n\n")
	}

	// check if pwd is a module, read module and set flag
	isModule := false
	var module wfapi.Module
	if _, err := os.Stat(filepath.Join(pwd, MODULE_FILE_NAME)); err == nil {
		isModule = true
		module, err = moduleFromFile(filepath.Join(pwd, MODULE_FILE_NAME))
		if err != nil {
			return fmt.Errorf("failed to open module file: %s", err)
		}
	}

	if isModule {
		fmt.Fprintf(c.App.Writer, "Module %q:\n", module.Name)
	} else {
		fmt.Fprintf(c.App.Writer, "No module in this directory.\n")
	}

	// display module and plot info
	var plot wfapi.Plot
	hasPlot := false
	_, err = os.Stat(filepath.Join(pwd, PLOT_FILE_NAME))
	if isModule && err == nil {
		// module.wf and plot.wf exists, read the plot
		hasPlot = true
		plot, err = plotFromFile(filepath.Join(pwd, PLOT_FILE_NAME))
		if err != nil {
			return fmt.Errorf("failed to open plot file: %s", err)
		}
	}

	if hasPlot {
		fmt.Fprintf(c.App.Writer, "\tPlot has %d inputs, %d steps, and %d outputs.\n",
			len(plot.Inputs.Keys),
			len(plot.Steps.Keys),
			len(plot.Outputs.Keys))

		// check for missing catalog refs
		wss, err := openWorkspaceSet()
		if err != nil {
			return fmt.Errorf("failed to open workspace: %s", err)
		}
		catalogRefCount := 0
		ingestCount := 0
		mountCount := 0
		for _, input := range plot.Inputs.Values {
			if input.Basis().Mount != nil {
				mountCount++
			} else if input.Basis().Ingest != nil {
				ingestCount++
			} else if input.Basis().CatalogRef != nil {
				ware, _, err := wss.GetCatalogWare(*input.PlotInputSimple.CatalogRef)
				if err != nil {
					return fmt.Errorf("failed to lookup catalog ref: %s", err)
				}
				if ware == nil {
					fmt.Fprintf(c.App.Writer, "\tMissing catalog item: %q.\n", input.Basis().CatalogRef.String())
				} else if err == nil {
					catalogRefCount++
				}
			}
		}
		fmt.Fprintf(c.App.Writer, "\tPlot contains %d resolved catalog inputs(s).\n", catalogRefCount)
		if ingestCount > 0 {
			fmt.Fprintf(c.App.Writer, "\tWarning: plot contains %d ingest input(s) and is not hermetic!\n", ingestCount)
		}
		if mountCount > 0 {
			fmt.Fprintf(c.App.Writer, "\tWarning: plot contains %d mount input(s) and is not hermetic!\n", mountCount)
		}

	} else if isModule {
		// directory is a module, but has no plot
		fmt.Fprintf(c.App.Writer, "\tNo plot file for module.\n")
	}

	// display workspace info
	fmt.Fprintf(c.App.Writer, "\nWorkspace:\n")
	wss, err := openWorkspaceSet()
	if err != nil {
		return fmt.Errorf("failed to open workspace set: %s", err)
	}

	// handle special case for pwd
	fmt.Fprintf(c.App.Writer, "\t%s (pwd", pwd)
	if isModule {
		fmt.Fprintf(c.App.Writer, ", module")
	}
	// check if it's a workspace
	if _, err := os.Stat(filepath.Join(pwd, ".warpforge")); !os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, ", workspace")
	}
	// check if it's a git repo
	if _, err := os.Stat(filepath.Join(pwd, ".git")); !os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, ", git repo")
	}

	fmt.Fprintf(c.App.Writer, ")\n")

	// handle all other workspaces
	for _, ws := range wss.Stack {
		fs, subPath := ws.Path()
		path := fmt.Sprintf("\t%s%s", fs, subPath)

		if path == pwd {
			// we handle pwd earlier, ignore
			continue
		}

		labels := []string{}

		// collect workspaces labels
		if *ws == *wss.Root {
			labels = append(labels, "root workspace")
		}
		if *ws == *wss.Home {
			labels = append(labels, "home workspace")
		}
		if *ws != *wss.Root && *ws != *wss.Home {
			labels = append(labels, "workspace")
		}

		// label if it's a git repo
		if _, err := os.Stat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
			labels = append(labels, "git repo")
		}

		// print a line for this dir
		fmt.Fprintf(c.App.Writer, "%s (", path)
		for n, label := range labels {
			fmt.Fprintf(c.App.Writer, "%s", label)
			if n != len(labels)-1 {
				// this is not the last label
				fmt.Fprintf(c.App.Writer, ", ")
			}
		}
		fmt.Fprintf(c.App.Writer, ")\n")
	}

	if isModule && hasPlot {
		fmtBold.Fprintf(c.App.Writer, "\nYou can evaluate this module with the `%s run` command.\n", os.Args[0])
	}

	return nil
}
