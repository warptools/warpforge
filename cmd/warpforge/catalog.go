package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
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

	// get pwd, create workspace if it doesn't exist, then open the workspace
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
	ws, err := workspace.OpenWorkspace(os.DirFS("/"), pwd[1:])
	if err != nil {
		return fmt.Errorf("failed to open workspace: %s", err)
	}

	// perform rio scan to determine the ware id of the provided item
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %s", err)
	}
	rioPath := filepath.Join(filepath.Dir(execPath), "rio")
	rioScan := exec.Command(
		rioPath, "scan", "--source="+url, packType,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rioScan.Stdout = &stdout
	rioScan.Stderr = &stderr
	err = rioScan.Run()
	if err != nil {
		return fmt.Errorf("failed to run rio scan command: %s\n%s", err, stderr.String())
	}
	wareIdStr := strings.TrimSpace(stdout.String())
	hash := strings.Split(wareIdStr, ":")[1]
	scanWareId := wfapi.WareID{
		Packtype: wfapi.Packtype(packType),
		Hash:     hash,
	}

	// attempt to load the existing lineage and mirrors files
	_, wsPath := ws.Path()
	path := filepath.Join("/", wsPath, ".warpforge", "catalog", moduleName)
	lineagePath := filepath.Join(path, "lineage.json")
	mirrorsPath := filepath.Join(path, "mirrors.json")

	var lineage wfapi.CatalogLineage
	lineageBytes, err := os.ReadFile(lineagePath)
	if os.IsNotExist(err) {
		lineage = wfapi.CatalogLineage{
			Name: moduleName,
		}
	} else if err == nil {
		_, err = ipld.Unmarshal(lineageBytes, json.Decode, &lineage, wfapi.TypeSystem.TypeByName("CatalogLineage"))
		if err != nil {
			return fmt.Errorf("could not parse lineage file %q: %s", lineagePath, err)
		}
	} else {
		return fmt.Errorf("could not open lineage file %q: %s", lineagePath, err)
	}

	var mirrors wfapi.CatalogMirror
	mirrorsBytes, err := os.ReadFile(mirrorsPath)
	if os.IsNotExist(err) {
		mirrors = wfapi.CatalogMirror{
			ByWare: &wfapi.CatalogMirrorByWare{},
		}
	} else if err == nil {
		_, err = ipld.Unmarshal(mirrorsBytes, json.Decode, &mirrors, wfapi.TypeSystem.TypeByName("CatalogMirror"))
		if err != nil {
			return fmt.Errorf("could not parse mirrors file %q: %s", lineagePath, err)
		}
	} else {
		return fmt.Errorf("could not open mirrors file %q: %s", lineagePath, err)
	}

	// add this item to the lineage file
	// first, search for the release
	releaseIdx := -1
	for i, release := range lineage.Releases {
		if release.Name == releaseName {
			// release was found, we will add to it
			releaseIdx = i
			break
		}
	}
	if releaseIdx == -1 {
		// release was not found, add a new release value
		lineage.Releases = append(lineage.Releases, wfapi.CatalogRelease{
			Name: releaseName,
		})
		releaseIdx = len(lineage.Releases) - 1
	}
	// check if this item exists for the release
	existingWareId, found := lineage.Releases[releaseIdx].Items.Values[itemName]
	if found {
		// ensure the ware ids match
		if existingWareId != scanWareId {
			return fmt.Errorf("computed ware id does not match existing catalog value")
		}
		// if the ware ids match, the lineage file is fine, do nothing
	} else {
		// if the item does not exist, add it
		lineage.Releases[releaseIdx].Items.Keys = append(lineage.Releases[releaseIdx].Items.Keys, itemName)
		if lineage.Releases[releaseIdx].Items.Values == nil {
			lineage.Releases[releaseIdx].Items.Values = make(map[string]wfapi.WareID)
		}
		lineage.Releases[releaseIdx].Items.Values[itemName] = scanWareId
	}

	// add this item to the mirrors file
	switch {
	case mirrors.ByWare != nil:
		mirrorList, wareIdExists := mirrors.ByWare.Values[scanWareId]
		if wareIdExists {
			// avoid adding duplicate values
			mirrorExists := false
			for _, m := range mirrorList {
				if m == wfapi.WarehouseAddr(url) {
					mirrorExists = true
					break
				}
			}
			if !mirrorExists {
				mirrors.ByWare.Values[scanWareId] = append(mirrors.ByWare.Values[scanWareId], wfapi.WarehouseAddr(url))
			}
		} else {
			mirrors.ByWare.Keys = append(mirrors.ByWare.Keys, scanWareId)
			if mirrors.ByWare.Values == nil {
				mirrors.ByWare.Values = make(map[wfapi.WareID][]wfapi.WarehouseAddr)
			}
			mirrors.ByWare.Values[scanWareId] = []wfapi.WarehouseAddr{wfapi.WarehouseAddr(url)}
		}
	default:
		panic("unsupported")
	}

	// serialize the updated structs
	lineageSerial, err := ipld.Marshal(json.Encode, &lineage, wfapi.TypeSystem.TypeByName("CatalogLineage"))
	if err != nil {
		return fmt.Errorf("failed to serialize lineage: %s", err)
	}
	mirrorSerial, err := ipld.Marshal(json.Encode, &mirrors, wfapi.TypeSystem.TypeByName("CatalogMirror"))
	if err != nil {
		return fmt.Errorf("failed to serialize mirror: %s", err)
	}

	fmt.Println(string(lineageSerial))
	fmt.Println(string(mirrorSerial))

	// write the updated structs
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory for catalog entry: %s", err)
	}

	err = os.WriteFile(filepath.Join(path, "lineage.json"), lineageSerial, 0644)
	if err != nil {
		return fmt.Errorf("failed to write lineage file: %s", err)
	}
	os.WriteFile(filepath.Join(path, "mirrors.json"), mirrorSerial, 0644)
	if err != nil {
		return fmt.Errorf("failed to write mirror file: %s", err)
	}

	return nil
}
