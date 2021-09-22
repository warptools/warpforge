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

func (ws *Workspace) GetCatalogWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, error) {
	lineage, err := ws.getCatalogLineage(ref)
	if err != nil {
		return nil, nil, err
	}

	// no matching lineage found
	if lineage == nil {
		return nil, nil, nil
	}

	// lineage found, try find the matching release and item
	item := getWareFromLineage(lineage, ref)
	if item == nil {
		// not found in this lineage
		return nil, nil, nil
	}

	mirror, err := ws.getCatalogMirror(ref)
	if err != nil {
		return nil, nil, err
	}

	// TODO: handling of multiple mirrors
	switch {
	case mirror.ByWare != nil:
		return item, &mirror.ByWare.Values[*item][0], nil
	case mirror.ByModule != nil:
		return item, &mirror.ByModule.Values[ref.ModuleName].Values[item.Packtype][0], nil
	default:
		panic("unreachable")
	}
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

func (ws *Workspace) getCatalogMirror(ref wfapi.CatalogRef) (*wfapi.CatalogMirror, error) {
	mirror := wfapi.CatalogMirror{}

	// open lineage file
	mirrorPath := filepath.Join(ws.rootPath, ".warpforge", "catalog", string(ref.ModuleName), "mirrors.json")
	var mirrorFile fs.File
	var err error
	if mirrorFile, err = ws.fsys.Open(mirrorPath); os.IsNotExist(err) {
		// no mirror file for this ware
		return nil, fmt.Errorf("no mirror file found for catalog reference %q", ref.String())
	} else if err != nil {
		return nil, fmt.Errorf("error opening lineage file %q: %s", mirrorPath, err)
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

func (ws *Workspace) getCatalogLineage(ref wfapi.CatalogRef) (*wfapi.CatalogLineage, error) {
	lineage := wfapi.CatalogLineage{}

	// open lineage file
	lineagePath := filepath.Join(ws.rootPath, ".warpforge", "catalog", string(ref.ModuleName), "lineage.json")
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
