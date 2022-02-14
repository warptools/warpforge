package workspace

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/warpfork/warpforge/wfapi"
)

const (
	magicModuleFilename = "module.json"
)

// The Catalog struct represents a single catalog.
// All methods of Catalog will operate on that specific catalog. Higher level
// functionality to traverse multiple catalogs is provided by Workspace and
// WorkspaceSet structs.
type Catalog struct {
	workspace  *Workspace
	path       string
	moduleList []wfapi.ModuleName
}

// Get the file path for a CatalogModule file.
// This will be [catalog path]/[module name]/module.json
func (cat *Catalog) moduleFilePath(ref wfapi.CatalogRef) string {
	path := filepath.Join(cat.path, string(ref.ModuleName), "module")
	path = strings.Join([]string{path, ".json"}, "")
	return path
}

// Get the file path for a CatalogModule file.
// This will be [catalog path]/[module name]/mirrors.json
func (cat *Catalog) mirrorFilePath(ref wfapi.CatalogRef) string {
	base := filepath.Dir(cat.moduleFilePath(ref))
	path := filepath.Join(base, "mirrors.json")
	return path
}

// Get the path for a CatalogRelease file.
// This will be [catalog path]/[module name]/releases/[release name].json
func (cat *Catalog) releaseFilePath(ref wfapi.CatalogRef) string {
	base := filepath.Dir(cat.moduleFilePath(ref))
	path := filepath.Join(base, "releases", string(ref.ReleaseName))
	path = strings.Join([]string{path, ".json"}, "")
	return path
}

// Get the path for a CatalogReplay file.
// This will be [catalog path]/[module name]/replays/[release name].json
func (cat *Catalog) replayFilePath(ref wfapi.CatalogRef) string {
	base := filepath.Dir(cat.moduleFilePath(ref))
	path := filepath.Join(base, "replays", string(ref.ReleaseName))
	path = strings.Join([]string{path, ".json"}, "")
	return path
}

// Open a catalog.
// This is only intended to be used internally in the workspace package. It
// should be publically accessed through Workspace.OpenCatalog()
//
// Errors:
//
// warpforge-error-io -- when building the module list fails due to I/O error
func openCatalog(ws *Workspace, path string) (Catalog, wfapi.Error) {
	// check that the catalog path exists
	if _, errRaw := fs.Stat(ws.fsys, path); os.IsNotExist(errRaw) {
		return Catalog{}, wfapi.ErrorCatalogInvalid(path, "catalog does not exist")
	}

	cat := Catalog{
		workspace: ws,
		path:      path,
	}

	// build a list of the modules in this catalog
	err := cat.updateModuleList()
	if err != nil {
		return Catalog{}, err
	}

	return cat, nil
}

// Update a Catalog's list of modules.
// This will be called when opening the catalog, and after updating the filesystem.
func (cat *Catalog) updateModuleList() wfapi.Error {
	// clear the list
	cat.moduleList = []wfapi.ModuleName{}

	// recursively assemble module names
	// modules names are subdirectories of the catalog (e.g., "catalog/example.com/package/module")
	// we need to recurse through each directory until we hit a "module.json"
	return recurseModuleDir(cat, cat.path, "")
}

func recurseModuleDir(cat *Catalog, basePath string, moduleName wfapi.ModuleName) wfapi.Error {
	// list the directory
	thisDir := filepath.Join(basePath, string(moduleName))
	files, errRaw := fs.ReadDir(cat.workspace.fsys, thisDir)
	if errRaw != nil {
		return wfapi.ErrorIo("could not read catalog directory", &cat.path, errRaw)
	}

	// check if a "module.json" exists
	for _, info := range files {
		if info.Name() == magicModuleFilename {
			// found the file, add this module name to the catalog
			cat.moduleList = append(cat.moduleList, moduleName)
			return nil
		}
	}

	// this directory does not contain a module
	// recurse into subdirectories to find additional modules
	for _, info := range files {
		if info.IsDir() {
			err := recurseModuleDir(cat, basePath, wfapi.ModuleName(filepath.Join(string(moduleName), info.Name())))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Get a release from a given catalog.
//
// Errors:
//
//     - warpforge-error-io -- when reading of lineage or mirror files fails
//     - warpforge-error-catalog-parse -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-catalog-invalid -- when catalog files or entries are not found
func (cat *Catalog) GetRelease(ref wfapi.CatalogRef) (*wfapi.CatalogRelease, wfapi.Error) {
	// try to find a module in this catalog
	mod, err := cat.GetModule(ref)
	if err != nil {
		return nil, err
	}
	if mod == nil {
		// module not found in this catalog
		return nil, nil
	}

	// find the specified release
	releaseCid, releaseFound := mod.Releases.Values[ref.ReleaseName]
	if !releaseFound {
		// release was not found in catalog
		return nil, nil
	}

	// release was found in catalog, attempt to open release file
	releasePath := cat.releaseFilePath(ref)
	releaseBytes, errRaw := fs.ReadFile(cat.workspace.fsys, releasePath)
	if os.IsNotExist(errRaw) {
		return nil, nil
	} else if errRaw != nil {
		return nil, wfapi.ErrorIo("failed to read catalog release file", &releasePath, errRaw)
	}

	// parse the release file
	release := wfapi.CatalogRelease{}
	_, errRaw = ipld.Unmarshal(releaseBytes, json.Decode, &release, wfapi.TypeSystem.TypeByName("CatalogRelease"))
	if errRaw != nil {
		return nil, wfapi.ErrorCatalogParse(releasePath, err)
	}

	// ensure this matches the expected value
	if release.Cid() != releaseCid {
		return nil, wfapi.ErrorCatalogInvalid(releasePath,
			fmt.Sprintf("expected CID %q for release, actual CID is %q", releaseCid, release.Cid()))
	}

	return &release, nil
}

// Get a ware from a given catalog.
//
// Errors:
//
//     - warpforge-error-io -- when reading of lineage or mirror files fails
//     - warpforge-error-catalog-parse -- when ipld parsing of lineage or mirror files fails
//     - warpforge-error-catalog-invalid -- when catalog files or entries are not found
func (cat *Catalog) GetWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, wfapi.Error) {
	release, err := cat.GetRelease(ref)
	if err != nil {
		return nil, nil, err
	}
	if release == nil {
		// matching release was not found, return nil WareID
		return nil, nil, nil
	}

	// valid release found found, now try find the item
	wareId, itemFound := release.Items.Values[ref.ItemName]
	if !itemFound {
		return nil, nil, wfapi.ErrorCatalogInvalid(
			cat.releaseFilePath(ref),
			fmt.Sprintf("release %q does not contain the requested item %q", release.Name, ref.ItemName))
	}

	// item found, check for a matching mirror
	mirror, err := cat.GetMirror(ref)
	if err != nil {
		return nil, nil, err
	}

	// TODO: handling of multiple mirrors
	switch {
	case mirror == nil:
		return &wareId, nil, nil
	case mirror.ByWare != nil:
		if len(mirror.ByWare.Values[wareId]) > 0 {
			return &wareId, &mirror.ByWare.Values[wareId][0], nil
		} else {
			return &wareId, nil, nil
		}
	case mirror.ByModule != nil:
		return &wareId, &mirror.ByModule.Values[ref.ModuleName].Values[wareId.Packtype][0], nil
	default:
		panic("unreachable")
	}
}

// Get a catalog mirror for a given catalog reference.
//
// Errors:
//
//    - warpforge-error-io -- when reading lineage file fails
//    - warpforge-error-catalog-parse -- when ipld parsing of mirror file fails
func (cat *Catalog) GetMirror(ref wfapi.CatalogRef) (*wfapi.CatalogMirror, wfapi.Error) {
	mirror := wfapi.CatalogMirror{}

	// open lineage file
	mirrorPath := cat.mirrorFilePath(ref)
	var mirrorFile fs.File
	var err error
	if mirrorFile, err = cat.workspace.fsys.Open(mirrorPath); os.IsNotExist(err) {
		// no mirror file for this ware
		return nil, nil
	} else if err != nil {
		return nil, wfapi.ErrorIo("error opening mirror file", &mirrorPath, err)
	}

	// read and unmarshal mirror data
	mirrorBytes, err := ioutil.ReadAll(mirrorFile)
	if err != nil {
		return nil, wfapi.ErrorIo("failed to read mirror file", &mirrorPath, err)
	}
	_, err = ipld.Unmarshal(mirrorBytes, json.Decode, &mirror, wfapi.TypeSystem.TypeByName("CatalogMirror"))
	if err != nil {
		return nil, wfapi.ErrorCatalogParse(mirrorPath, err)
	}

	return &mirror, nil
}

// Get a lineage for a given catalog reference.
//
// Errors:
//
//    - warpforge-error-io -- when reading lineage file fails
//    - warpforge-error-catalog-parse -- when ipld unmarshaling fails
func (cat *Catalog) GetModule(ref wfapi.CatalogRef) (*wfapi.CatalogModule, wfapi.Error) {
	mod := wfapi.CatalogModule{}

	// open lineage file
	modPath := cat.moduleFilePath(ref)
	var modFile fs.File
	var err error
	if modFile, err = cat.workspace.fsys.Open(modPath); os.IsNotExist(err) {
		// lineage file does not exist for this workspace
		return nil, nil
	} else if err != nil {
		return nil, wfapi.ErrorIo("error opening module file", &modPath, err)
	}

	// read and unmarshal module data
	modBytes, err := ioutil.ReadAll(modFile)
	if err != nil {
		return nil, wfapi.ErrorIo("error reading module file", &modPath, err)
	}
	_, err = ipld.Unmarshal(modBytes, json.Decode, &mod, wfapi.TypeSystem.TypeByName("CatalogModule"))
	if err != nil {
		return nil, wfapi.ErrorCatalogParse(modPath, err)
	}

	return &mod, nil
}

// Adds a new item to the catalog.
//
// Errors:
//
//    - warpforge-error-catalog-parse -- when parsing of the lineage file fails
//    - warpforge-error-io -- when reading or writing the lineage file fails
//    - warpforge-error-serialization -- when serializing the lineage fails
//    - warpforge-error-catalog-invalid -- when an error occurs while searching for module or release
func (cat *Catalog) AddItem(
	ref wfapi.CatalogRef,
	wareId wfapi.WareID) error {

	// determine paths for the module, release, and the corresponding files
	moduleFilePath := filepath.Join("/", cat.moduleFilePath(ref))
	releaseFilePath := filepath.Join("/", cat.releaseFilePath(ref))
	releasesPath := filepath.Dir(releaseFilePath)

	// attempt to load the release
	release, err := cat.GetRelease(ref)
	if err != nil {
		return err
	}
	if release == nil {
		// release does not exist, create a new one
		release = &wfapi.CatalogRelease{
			Name: ref.ReleaseName,
			Items: struct {
				Keys   []wfapi.ItemLabel
				Values map[wfapi.ItemLabel]wfapi.WareID
			}{},
			Metadata: struct {
				Keys   []string
				Values map[string]string
			}{},
		}
		release.Items.Values = map[wfapi.ItemLabel]wfapi.WareID{}
		release.Metadata.Values = map[string]string{}
	}

	// ensure the item does not already exist
	_, hasItem := release.Items.Values[ref.ItemName]
	if hasItem {
		return wfapi.ErrorCatalogInvalid(releaseFilePath,
			fmt.Sprintf("release %q already has item %q", ref.ReleaseName, ref.ItemName))
	}

	release.Items.Keys = append(release.Items.Keys, ref.ItemName)
	release.Items.Values[ref.ItemName] = wareId

	// attempt to load the module
	module, err := cat.GetModule(ref)
	if err != nil {
		return err
	}

	if module == nil {
		// module does not exist, create a new one
		module = &wfapi.CatalogModule{
			Name: ref.ModuleName,
			Releases: struct {
				Keys   []wfapi.ReleaseName
				Values map[wfapi.ReleaseName]wfapi.CatalogReleaseCID
			}{},
			Metadata: struct {
				Keys   []string
				Values map[string]string
			}{},
		}
		module.Releases.Values = map[wfapi.ReleaseName]wfapi.CatalogReleaseCID{}
		module.Metadata.Values = map[string]string{}

		// create the module and releases since they should not already exist
		errRaw := os.MkdirAll(releasesPath, 0755)
		if errRaw != nil {
			return wfapi.ErrorIo("failed to create releases directory", &releasesPath, errRaw)
		}
	}

	// if the release does not already exists, append the key
	releaseExists := false
	for _, k := range module.Releases.Keys {
		if k == ref.ReleaseName {
			releaseExists = true
			break
		}
	}
	if !releaseExists {
		module.Releases.Keys = append(module.Releases.Keys, ref.ReleaseName)
	}
	// create or update the CID link to the module
	module.Releases.Values[ref.ReleaseName] = release.Cid()

	// serialize the updated structures
	moduleSerial, errRaw := ipld.Marshal(json.Encode, module, wfapi.TypeSystem.TypeByName("CatalogModule"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize module", errRaw)
	}
	releaseSerial, errRaw := ipld.Marshal(json.Encode, release, wfapi.TypeSystem.TypeByName("CatalogRelease"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize release", errRaw)
	}

	// write the updated structures
	errRaw = os.WriteFile(moduleFilePath, moduleSerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write module file", &moduleFilePath, errRaw)
	}
	errRaw = os.WriteFile(releaseFilePath, releaseSerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write release file", &releaseFilePath, err)
	}

	return nil
}

// Adds a ByWare mirror to a catalog entry
//
// Errors:
//
//    - warpforge-error-io -- when reading or writing mirror file fails
//    - warpforge-error-catalog-parse -- when parsing existing mirror file fails
//    - warpforge-error-serialization -- when serializing the new mirror file fails
//    - warpforge-error-catalog-invalid -- when the wrong type of mirror file already exists
func (cat *Catalog) AddByWareMirror(
	ref wfapi.CatalogRef,
	wareId wfapi.WareID,
	addr wfapi.WarehouseAddr) error {
	// load mirrors file, or create it if it doesn't exist
	mirrorsPath := filepath.Join(
		cat.path,
		string(ref.ModuleName),
		"mirrors.json",
	)

	var mirrors wfapi.CatalogMirror
	mirrorsBytes, err := fs.ReadFile(cat.workspace.fsys, mirrorsPath)
	if os.IsNotExist(err) {
		mirrors = wfapi.CatalogMirror{
			ByWare: &wfapi.CatalogMirrorByWare{},
		}
	} else if err == nil {
		_, err = ipld.Unmarshal(mirrorsBytes, json.Decode, &mirrors, wfapi.TypeSystem.TypeByName("CatalogMirror"))
		if err != nil {
			return wfapi.ErrorCatalogParse(mirrorsPath, err)
		}
	} else {
		return wfapi.ErrorIo("could not open mirrors file", &mirrorsPath, err)
	}

	// add this item to the mirrors file
	if mirrors.ByWare == nil {
		return wfapi.ErrorCatalogInvalid(mirrorsPath, "existing mirrors file is not of type ByWare")
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
	path := filepath.Join("/", filepath.Dir(mirrorsPath))
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return wfapi.ErrorIo("could not create catalog path", &path, err)
	}
	mirrorSerial, err := ipld.Marshal(json.Encode, &mirrors, wfapi.TypeSystem.TypeByName("CatalogMirror"))
	if err != nil {
		return wfapi.ErrorSerialization("could not serialize mirror", err)
	}
	os.WriteFile(filepath.Join("/", mirrorsPath), mirrorSerial, 0644)
	if err != nil {
		return wfapi.ErrorIo("failed to write mirrors file", &mirrorsPath, err)
	}

	return nil
}

// Get the list of modules within this catalog.
func (cat *Catalog) Modules() []wfapi.ModuleName {
	return cat.moduleList
}

// Get a replay plot from a catalog.
//
// Errors:
//
//    - warpforge-error-catalog-invalid -- when the contents of the catalog is invalid
//    - warpforge-error-catalog-parse -- when parsing of catalog data fails
//    - warpforge-error-io -- when an io error occurs while opening the catalog
func (cat *Catalog) GetReplay(ref wfapi.CatalogRef) (*wfapi.Plot, wfapi.Error) {
	release, err := cat.GetRelease(ref)
	if err != nil {
		return nil, err
	}
	if release == nil {
		// release not found in catalog
		return nil, nil
	}

	replayCid, replayExists := release.Metadata.Values["replay"]
	if !replayExists {
		// release exists, but replay does not
		return nil, nil
	}

	replayPath := filepath.Join(
		cat.path,
		string(ref.ModuleName),
		"replays",
		string(ref.ReleaseName))
	replayPath = strings.Join([]string{replayPath, ".json"}, "")

	replayBytes, errRaw := fs.ReadFile(cat.workspace.fsys, replayPath)
	if os.IsNotExist(errRaw) {
		return nil, wfapi.ErrorCatalogInvalid(replayPath, "referenced replay file does not exist")
	} else if errRaw != nil {
		return nil, wfapi.ErrorIo("could not stat replay file", &replayPath, errRaw)
	}

	replay := wfapi.Plot{}
	_, errRaw = ipld.Unmarshal(replayBytes, json.Decode, &replay, wfapi.TypeSystem.TypeByName("Plot"))
	if errRaw != nil {
		return nil, wfapi.ErrorCatalogParse(replayPath, err)
	}

	// ensure the CID matches the expected value
	if replay.Cid() != wfapi.PlotCID(replayCid) {
		return nil, wfapi.ErrorCatalogInvalid(replayPath,
			fmt.Sprintf("expected CID %q for plot, actual CID is %q", replayCid, replay.Cid()))
	}

	return &replay, nil
}

// Add a replay plot to a catalog.
//
// Errors:
//
//    - warpforge-error-catalog-invalid -- when the contents of the catalog is invalid
//    - warpforge-error-catalog-parse -- when parsing of catalog data fails
//    - warpforge-error-io -- when an io error occurs while opening the catalog
//    - warpforge-error-serialization -- when the updated structures fail to serialize
func (cat *Catalog) AddReplay(ref wfapi.CatalogRef, plot wfapi.Plot) wfapi.Error {
	// first, write the Plot to the replay file

	// determine the release and replay paths
	releasePath := filepath.Join("/", cat.releaseFilePath(ref))
	replayPath := filepath.Join("/", cat.replayFilePath(ref))

	// determine where the replay should be stored, and create the dir if it does not exist
	replaysPath := filepath.Dir(replayPath)
	errRaw := os.MkdirAll(replaysPath, 0755)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to create replays directory", &replaysPath, errRaw)
	}

	// serialize the replay Plot and write the file
	replaySerial, errRaw := ipld.Marshal(json.Encode, &plot, wfapi.TypeSystem.TypeByName("Plot"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize replay", errRaw)
	}
	errRaw = os.WriteFile(replayPath, replaySerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write replay file", &replayPath, errRaw)
	}

	// next, update the CatalogRelease to add the replay CID

	// get the current release contents
	release, err := cat.GetRelease(ref)
	if err != nil {
		return err
	}
	if release == nil {
		return wfapi.ErrorCatalogInvalid(releasePath, "release does not exist")
	}

	// check if a replay already exists, if so, fail.
	_, hasReplay := release.Metadata.Values["replay"]
	if hasReplay {
		return wfapi.ErrorCatalogInvalid(releasePath, "release already has replay")
	}

	// no replay exists, update the metadata to add it
	release.Metadata.Keys = append(release.Metadata.Keys, "replay")
	release.Metadata.Values["replay"] = string(plot.Cid())

	// serialize the CatalogRelease and write the file
	releaseSerial, errRaw := ipld.Marshal(json.Encode, release, wfapi.TypeSystem.TypeByName("CatalogRelease"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize release", errRaw)
	}
	errRaw = os.WriteFile(releasePath, releaseSerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write release file", &releasePath, errRaw)
	}

	// lastly, we will need to update the CatalogRelease CID in the CatalogModule

	// determine the path and flename
	moduleFilePath := filepath.Join("/", cat.moduleFilePath(ref))

	// load the existing module data
	module, err := cat.GetModule(ref)
	if err != nil {
		return err
	}
	if release == nil {
		return wfapi.ErrorCatalogInvalid(moduleFilePath, "module does not exist")
	}

	// ensure that this module already has a CatalogRelease for the replay
	// this release must have been created before adding replay data, otherwise fail
	_, hasRelease := module.Releases.Values[ref.ReleaseName]
	if !hasRelease {
		return wfapi.ErrorCatalogInvalid(moduleFilePath,
			fmt.Sprintf("module does not have release %q", ref.ReleaseName))
	}

	// update the CatalogRelease CID
	module.Releases.Values[ref.ReleaseName] = release.Cid()

	// serialize the CatalogModule and write the file
	moduleSerial, errRaw := ipld.Marshal(json.Encode, module, wfapi.TypeSystem.TypeByName("CatalogModule"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize module", errRaw)
	}
	errRaw = os.WriteFile(moduleFilePath, moduleSerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write module file", &moduleFilePath, errRaw)
	}

	return nil
}
