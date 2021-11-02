package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
)

// Returns the file type, which is the file name without extension
// e.g., formula.json -> formula, module.json -> module, etc...
func getFileType(name string) (string, error) {
	split := strings.Split(filepath.Base(name), ".")
	if len(split) < 2 {
		// ignore files without extensions
		return "", nil
	}
	return split[0], nil
}

// helper function for finding the path to internally used binaries (e.g, rio, runc)
func binPath(bin string) (string, error) {
	path, override := os.LookupEnv("WARPFORGE_PATH")
	if override {
		return filepath.Join(path, bin), nil
	}

	path, err := os.Executable()
	if err != nil {
		return "", err
	}

	return filepath.Join(filepath.Dir(path), bin), nil
}

func unimplemented(c *cli.Context) error {
	return fmt.Errorf("sorry, command %s is not implemented", c.Command.Name)
}
