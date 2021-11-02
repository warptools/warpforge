package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

var catalogCmdDef = cli.Command{
	Name:  "catalog",
	Usage: "Subcommands that operate on catalogs.",
	Subcommands: []*cli.Command{
		{
			Name:   "add",
			Usage:  "Add an item to the catalog.",
			Action: cmdCatalogAdd,
		},
	},
}

func scanWareId(packType wfapi.Packtype, addr wfapi.WarehouseAddr) (wfapi.WareID, error) {
	result := wfapi.WareID{}
	rioPath, err := binPath("rio")
	if err != nil {
		return result, fmt.Errorf("failed to get path to rio")
	}
	rioScan := exec.Command(
		rioPath, "scan", "--source="+string(addr), string(packType),
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rioScan.Stdout = &stdout
	rioScan.Stderr = &stderr
	err = rioScan.Run()
	if err != nil {
		fmt.Println(err)
		return result, fmt.Errorf("failed to run rio scan command: %s\n%s", err, stderr.String())
	}
	wareIdStr := strings.TrimSpace(stdout.String())
	hash := strings.Split(wareIdStr, ":")[1]
	result = wfapi.WareID{
		Packtype: wfapi.Packtype(packType),
		Hash:     hash,
	}

	return result, nil
}

func cmdCatalogAdd(c *cli.Context) error {
	if c.Args().Len() != 3 {
		return fmt.Errorf("invalid input")
	}

	packType := c.Args().Get(0)
	catalogRefStr := c.Args().Get(1)
	url := c.Args().Get(2)

	// get the module, release, and item values (in format `module:release:item`)
	catalogRefSplit := strings.Split(catalogRefStr, ":")
	if len(catalogRefSplit) != 3 {
		return fmt.Errorf("invalid catalog reference %q", catalogRefStr)
	}
	moduleName := catalogRefSplit[0]
	releaseName := catalogRefSplit[1]
	itemName := catalogRefSplit[2]

	ref := wfapi.CatalogRef{
		ModuleName:  wfapi.ModuleName(moduleName),
		ReleaseName: releaseName,
		ItemName:    itemName,
	}

	// get pwd and catalog dir, creating it if it doesn't exist
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %s", err)
	}
	catalogDir := filepath.Join(pwd, ".warpforge", "catalog")
	_, err = os.Stat(catalogDir)
	if os.IsNotExist(err) {
		os.MkdirAll(catalogDir, 0755)
	} else if err != nil {
		return fmt.Errorf("failed to create catalog directory: %s", err)
	}

	// open the workspace
	ws, err := workspace.OpenWorkspace(os.DirFS("/"), pwd[1:])
	if err != nil {
		return fmt.Errorf("failed to open workspace: %s", err)
	}

	// perform rio scan to determine the ware id of the provided item
	scanWareId, err := scanWareId(wfapi.Packtype(packType), wfapi.WarehouseAddr(url))
	if err != nil {
		return fmt.Errorf("scanning %q failed: %s", url, err)
	}

	// add the new item
	err = ws.AddCatalogItem(ref, scanWareId)
	if err != nil {
		return fmt.Errorf("failed to add item to catalog: %s", err)
	}
	err = ws.AddByWareMirror(ref, scanWareId, wfapi.WarehouseAddr(url))
	if err != nil {
		return fmt.Errorf("failed to add mirror: %s", err)
	}

	return nil
}
