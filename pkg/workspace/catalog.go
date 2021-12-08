package workspace

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/warpfork/warpforge/wfapi"
)

// Get a catalog ware from a single workspace, doing lookup by CatalogRef.
// This will first check all catalogs within the "catalogs" subdirectory, if it exists
// then, it will check the "catalog" subdirectory, if it exists
func (ws *Workspace) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, error) {
	// list the catalogs within the "catalogs" subdirectory
	catPaths, err := ws.ListCatalogPaths()
	if err != nil {
		return nil, nil, err
	}

	// if it exists, add the "catalog" subdirectory to the end of the list
	catalogPath := filepath.Join(ws.rootPath, magicWorkspaceDirname, "catalog")
	_, err = fs.Stat(ws.fsys, catalogPath)
	if err == nil {
		// "catalog" subdirectory exists
		catPaths = append(catPaths, catalogPath)
	}

	for _, path := range catPaths {
		// try to find a lineage in this path
		lineage, err := ws.getCatalogLineage(path, ref)
		if err != nil {
			return nil, nil, err
		}
		if lineage == nil {
			// lineage not found in this catalog
			continue
		}

		// lineage found, try find the item matching the catalog ref
		item := getWareFromLineage(lineage, ref)
		if item == nil {
			// item not found in this catalog
			continue
		}

		// item found, try to get the matching mirror
		mirror, err := ws.getCatalogMirror(path, ref)
		if err != nil {
			return nil, nil, err
		}

		// TODO: handling of multiple mirrors
		switch {
		case mirror == nil:
			return item, nil, nil
		case mirror.ByWare != nil:
			if len(mirror.ByWare.Values[*item]) > 0 {
				return item, &mirror.ByWare.Values[*item][0], nil
			} else {
				return item, nil, nil
			}
		case mirror.ByModule != nil:
			return item, &mirror.ByModule.Values[ref.ModuleName].Values[item.Packtype][0], nil
		default:
			panic("unreachable")
		}

	}

	// nothing found
	return nil, nil, nil
}

func getWareFromLineage(l *wfapi.CatalogLineage, ref wfapi.CatalogRef) *wfapi.WareID {
	for _, release := range l.Releases {
		if release.Name == ref.ReleaseName {
			for itemName, item := range release.Items.Values {
				if itemName == ref.ItemName {
					return &item
				}
			}
		}
	}
	return nil
}

func (ws *Workspace) getCatalogMirror(catalogPath string, ref wfapi.CatalogRef) (*wfapi.CatalogMirror, error) {
	mirror := wfapi.CatalogMirror{}

	// open lineage file
	mirrorPath := filepath.Join(catalogPath, string(ref.ModuleName), "mirrors.json")
	var mirrorFile fs.File
	var err error
	if mirrorFile, err = ws.fsys.Open(mirrorPath); os.IsNotExist(err) {
		// no mirror file for this ware
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error opening mirror file %q: %s", mirrorPath, err)
	}

	// read and unmarshal mirror data
	mirrorBytes, err := ioutil.ReadAll(mirrorFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read mirror file: %s", err)
	}
	_, err = ipld.Unmarshal(mirrorBytes, json.Decode, &mirror, wfapi.TypeSystem.TypeByName("CatalogMirror"))
	if err != nil {
		return nil, err
	}

	return &mirror, nil
}

func (ws *Workspace) getCatalogLineage(catalogPath string, ref wfapi.CatalogRef) (*wfapi.CatalogLineage, error) {
	lineage := wfapi.CatalogLineage{}

	// open lineage file
	lineagePath := filepath.Join(catalogPath, string(ref.ModuleName), "lineage.json")
	var lineageFile fs.File
	var err error
	if lineageFile, err = ws.fsys.Open(lineagePath); os.IsNotExist(err) {
		// lineage file does not exist for this workspace
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("error opening lineage file: %s", err)
	}

	// read and unmarshal lineage data
	lineageBytes, err := ioutil.ReadAll(lineageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read lineage file: %s", err)
	}
	_, err = ipld.Unmarshal(lineageBytes, json.Decode, &lineage, wfapi.TypeSystem.TypeByName("CatalogLineage"))
	if err != nil {
		return nil, err
	}

	return &lineage, nil
}

// Adds a new item to the specified catalog.
// If nil is provided as a catalog name, this will write to the workspace's "catalog" subdirectory
// Otherwise, it will write to the catalog at "catalogs/[catalogName]"
func (ws *Workspace) AddCatalogItem(catalogName *string,
	ref wfapi.CatalogRef,
	wareId wfapi.WareID) error {

	lineagePath := filepath.Join(
		ws.CatalogPath(catalogName),
		string(ref.ModuleName),
		"lineage.json",
	)

	// load lineage file
	var lineage wfapi.CatalogLineage
	lineageBytes, err := fs.ReadFile(ws.fsys, lineagePath)
	if os.IsNotExist(err) {
		lineage = wfapi.CatalogLineage{
			Name: string(ref.ModuleName),
		}
	} else if err == nil {
		_, err = ipld.Unmarshal(lineageBytes, json.Decode, &lineage, wfapi.TypeSystem.TypeByName("CatalogLineage"))
		if err != nil {
			return fmt.Errorf("could not parse lineage file %q: %s", lineagePath, err)
		}
	} else {
		return fmt.Errorf("could not open lineage file %q: %s", lineagePath, err)
	}

	// add this item to the lineage file
	// first, search for the release in the existing file
	releaseIdx := -1
	for i, release := range lineage.Releases {
		if release.Name == ref.ReleaseName {
			// release was found, we will add to it
			releaseIdx = i
			break
		}
	}
	if releaseIdx == -1 {
		// release was not found, insert a new release value
		lineage.Releases = append(lineage.Releases, wfapi.CatalogRelease{
			Name: ref.ReleaseName,
		})
		releaseIdx = len(lineage.Releases) - 1
	}

	// check if this item exists for the release
	_, found := lineage.Releases[releaseIdx].Items.Values[ref.ItemName]
	if found {
		// this item already exists, do nothing
		// TODO: do we want to check things match when adding?
	} else {
		// the item does not exist, add it
		lineage.Releases[releaseIdx].Items.Keys = append(lineage.Releases[releaseIdx].Items.Keys, ref.ItemName)
		if lineage.Releases[releaseIdx].Items.Values == nil {
			lineage.Releases[releaseIdx].Items.Values = make(map[string]wfapi.WareID)
		}
		lineage.Releases[releaseIdx].Items.Values[ref.ItemName] = wareId
	}

	// write the updated structure
	lineageSerial, err := ipld.Marshal(json.Encode, &lineage, wfapi.TypeSystem.TypeByName("CatalogLineage"))
	if err != nil {
		return fmt.Errorf("failed to serialize lineage: %s", err)
	}

	err = os.MkdirAll(filepath.Dir(filepath.Join("/", lineagePath)), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory for catalog entry: %s", err)
	}
	err = os.WriteFile(filepath.Join("/", lineagePath), lineageSerial, 0644)
	if err != nil {
		return fmt.Errorf("failed to write lineage file: %s", err)
	}

	return nil
}

func (ws *Workspace) AddByWareMirror(catalogName *string,
	ref wfapi.CatalogRef,
	wareId wfapi.WareID,
	addr wfapi.WarehouseAddr) error {
	// load mirrors file, or create it if it doesn't exist
	mirrorsPath := filepath.Join(
		ws.CatalogPath(catalogName),
		string(ref.ModuleName),
		"mirrors.json",
	)

	var mirrors wfapi.CatalogMirror
	mirrorsBytes, err := os.ReadFile(mirrorsPath)
	if os.IsNotExist(err) {
		mirrors = wfapi.CatalogMirror{
			ByWare: &wfapi.CatalogMirrorByWare{},
		}
	} else if err == nil {
		_, err = ipld.Unmarshal(mirrorsBytes, json.Decode, &mirrors, wfapi.TypeSystem.TypeByName("CatalogMirror"))
		if err != nil {
			return fmt.Errorf("could not parse mirrors file %q: %s", mirrorsPath, err)
		}
	} else {
		return fmt.Errorf("could not open mirrors file %q: %s", mirrorsPath, err)
	}

	// add this item to the mirrors file
	if mirrors.ByWare == nil {
		return fmt.Errorf("existing mirrors file is not of type ByWare")
	}
	mirrorList, wareIdExists := mirrors.ByWare.Values[wareId]
	if wareIdExists {
		// avoid adding duplicate values
		mirrorExists := false
		for _, m := range mirrorList {
			if m == addr {
				mirrorExists = true
				break
			}
		}
		if !mirrorExists {
			mirrors.ByWare.Values[wareId] = append(mirrors.ByWare.Values[wareId], addr)
		}
	} else {
		mirrors.ByWare.Keys = append(mirrors.ByWare.Keys, wareId)
		if mirrors.ByWare.Values == nil {
			mirrors.ByWare.Values = make(map[wfapi.WareID][]wfapi.WarehouseAddr)
		}
		mirrors.ByWare.Values[wareId] = []wfapi.WarehouseAddr{addr}
	}

	// write the updated data to the mirrors file
	err = os.MkdirAll(filepath.Join("/", filepath.Dir(mirrorsPath)), 0755)
	if err != nil {
		return fmt.Errorf("failed to create catalog dir: %s", err)
	}
	mirrorSerial, err := ipld.Marshal(json.Encode, &mirrors, wfapi.TypeSystem.TypeByName("CatalogMirror"))
	if err != nil {
		return fmt.Errorf("failed to serialize mirrors: %s", err)
	}
	os.WriteFile(filepath.Join("/", mirrorsPath), mirrorSerial, 0644)
	if err != nil {
		return fmt.Errorf("failed to write mirrors file: %s", err)
	}

	return nil
}

// Get a catalog ware from a workspace set.
// looks up a ware by CatalogRef, traversing the workspace set:
//  1. traverses the workspace stack looking in "catalog" dirs.
//  2. looks through all catalogs (within the "catalogs" dir) of the root workspace
//     in alphabetical order, picking the first matching ware found.
func (wsSet *WorkspaceSet) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, error) {

	// traverse workspace stack
	for _, ws := range wsSet.Stack {
		wareId, wareAddr, err := ws.GetCatalogWare(ref)
		if err != nil {
			return nil, nil, err
		}
		if wareId != nil {
			return wareId, wareAddr, nil
		}
	}

	// search root workspace
	wareId, wareAddr, err := wsSet.Root.GetCatalogWare(ref)
	if err != nil {
		return nil, nil, err
	}
	if wareId != nil {
		return wareId, wareAddr, nil
	}

	return nil, nil, nil
}
