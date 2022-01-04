package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/formulaexec"
)

var statusCmdDef = cli.Command{
	Name:   "status",
	Usage:  "Get status of workspaces and installation.",
	Action: cmdStatus,
}

func cmdStatus(c *cli.Context) error {
	// display version
	fmt.Fprintf(c.App.Writer, "Warpforge Version %s\n", VERSION)

	// display plugin info
	fmt.Fprintf(c.App.Writer, "\nPlugin Info:\n")

	binPath, err := formulaexec.GetBinPath()
	if err != nil {
		return fmt.Errorf("could not get binPath: %s", err)
	}
	fmt.Fprintf(c.App.Writer, "binPath = %s\n", binPath)

	rioPath := filepath.Join(binPath, "rio")
	if _, err := os.Stat(rioPath); os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, "MISSING rio (expected %s)\n", rioPath)
	} else {
		fmt.Fprintf(c.App.Writer, "found rio\n")
	}

	runcPath := filepath.Join(binPath, "runc")
	if _, err := os.Stat(runcPath); os.IsNotExist(err) {
		fmt.Fprintf(c.App.Writer, "MISSING runc (expected %s)\n", runcPath)
	} else {
		fmt.Fprintf(c.App.Writer, "found runc\n")
		runcVersionCmd := exec.Command(filepath.Join(binPath, "runc"), "--version")
		var runcVersionOut bytes.Buffer
		runcVersionCmd.Stdout = &runcVersionOut
		err = runcVersionCmd.Run()
		if err != nil {
			return fmt.Errorf("failed to get runc version information: %s", err)
		}
		fmt.Fprintf(c.App.Writer, "%s", &runcVersionOut)
	}

	// display workspace info
	fmt.Fprintf(c.App.Writer, "\nWorkspace Info:\n")
	wss, err := openWorkspaceSet()
	if err != nil {
		return fmt.Errorf("failed to open workspace set: %s", err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("could not get current directory")
	}

	// handle special case for pwd
	fmt.Fprintf(c.App.Writer, "%s (pwd", pwd)
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
		path := fmt.Sprintf("%s%s", fs, subPath)

		if path == pwd {
			// we handle pwd earlier, ignore
			continue
		}

		fmt.Fprintf(c.App.Writer, "%s (workspace", path)

		if *ws == *wss.Root {
			fmt.Fprintf(c.App.Writer, ", root")
		}
		if *ws == *wss.Home {
			fmt.Fprintf(c.App.Writer, ", home")
		}

		// check if it's a git repo
		if _, err := os.Stat(filepath.Join(path, ".git")); !os.IsNotExist(err) {
			fmt.Fprintf(c.App.Writer, ", git repo")
		}

		fmt.Fprintf(c.App.Writer, ")\n")
	}

	if canRun {
		fmt.Fprintf(c.App.Writer, "\nYou can evaluate this module with the `%s run` command.\n", os.Args[0])
	}

	return nil
}
