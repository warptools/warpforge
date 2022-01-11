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

	// display version
	fmt.Fprintf(c.App.Writer, "Warpforge Version: %s\n\n", VERSION)

	// check plugins
	pluginsOk := true

	if c.Bool("verbose") {
		fmt.Fprintf(c.App.Writer, "\nPlugin Info:\n")
	}

	binPath, err := formulaexec.GetBinPath()
	if err != nil {
		return fmt.Errorf("could not get binPath: %s", err)
	}
	if c.Bool("verbose") {
		fmt.Fprintf(c.App.Writer, "binPath = %s\n", binPath)
	}

	rioPath := filepath.Join(binPath, "rio")
	if _, err := os.Stat(rioPath); os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, "rio not found (expected at %s)\n", rioPath)
		pluginsOk = false
	} else {
		if c.Bool("verbose") {
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
		if c.Bool("verbose") {
			fmt.Fprintf(c.App.Writer, "found runc\n")
			fmt.Fprintf(c.App.Writer, "%s", &runcVersionOut)
		}
	}

	if !pluginsOk {
		fmtWarning.Fprintf(c.App.Writer, "WARNING: plugins do not appear to be installed correctly.\n\n")
	}

	// display workspace info
	fmt.Fprintf(c.App.Writer, "Workspace:\n")
	wss, err := openWorkspaceSet()
	if err != nil {
		return fmt.Errorf("failed to open workspace set: %s", err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get current directory")
	}

	// handle special case for pwd
	fmt.Fprintf(c.App.Writer, "\t%s (pwd", pwd)
	// check if it's a module (and can therefore run)
	canRun := false
	if _, err := os.Stat(filepath.Join(pwd, "module.json")); !os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, ", module")
		canRun = true
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

	if canRun {
		fmtBold.Fprintf(c.App.Writer, "\nYou can evaluate this module with the `%s run` command.\n", os.Args[0])
	}

	return nil
}
