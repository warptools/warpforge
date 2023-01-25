package workspace

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/facette/natsort"
	"github.com/ipld/go-ipld-prime"
	_ "github.com/ipld/go-ipld-prime/codec/dagcbor"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/wfapi"
)

const (
	magicModuleFilename = "_module.json"
	CatalogNameFormat   = `^[A-Za-z0-9][-A-Za-z0-9_.]{0,62}$`
)

var reCatalogName = regexp.MustCompile(CatalogNameFormat)

// The Catalog struct represents a single catalog.
// All methods of Catalog will operate on that specific catalog. Higher level
// functionality to traverse multiple catalogs is provided by Workspace and
// WorkspaceSet structs.
type Catalog struct {
	fsys       fs.FS  // Usually `osfs.Dir("/")` when live, but may vary for tests.
	path       string // Always concatenated to the front of anything else we do.
	moduleList []wfapi.ModuleName
}

// OpenCatalog creates an object that can be used to access catalog data on the local filesystem.
// It will check if files looking like catalog data exist, and error if not.
// It also immediately loads a list of available modules into memory.
//
// Note that you can get a similar object through `Workspace.OpenCatalog()`,
// which is often more convenient.
//
// Will return an empty catalog object if the directory does not exist.
//
// Errors:
//
// 	- warpforge-error-io -- when building the module list fails due to I/O error
// 	- warpforge-error-catalog-invalid -- when the catalog file exists but cannot be opened
func OpenCatalog(fsys fs.FS, path string) (Catalog, error) {
	if filepath.IsAbs(path) {
		path = path[1:]
	}
	// check that the catalog path exists
	// FUTURE: We should add more indicator files here, to help avoid some forms of possible user error and give better messages.
	if _, err := fs.Stat(fsys, path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Catalog{
				fsys: fsys,
				path: path,
			}, nil
		}
		return Catalog{}, serum.Error(wfapi.ECodeCatalogInvalid,
			serum.WithMessageTemplate("catalog not found at path {{path | q}}"),
			serum.WithDetail("path", path),
			serum.WithCause(err),
		)
	}

	cat := Catalog{
		fsys: fsys,
		path: path,
	}

	// build a list of the modules in this catalog
	if err := cat.updateModuleList(); err != nil {
		return Catalog{}, err
	}

	return cat, nil
}

// Get the file path for a CatalogModule file.
// This will be [catalog path]/[module name]/_module.json
func (cat *Catalog) moduleFilePath(ref wfapi.CatalogRef) string {
	path := filepath.Join(cat.path, string(ref.ModuleName), "_module")
	path = strings.Join([]string{path, ".json"}, "")
	return path
}

// Get the file path for a CatalogModule file.
// This will be [catalog path]/[module name]/_mirrors.json
func (cat *Catalog) mirrorFilePath(ref wfapi.CatalogRef) string {
	base := filepath.Dir(cat.moduleFilePath(ref))
	path := filepath.Join(base, "_mirrors.json")
	return path
}

// Get the path for a CatalogRelease file.
// This will be [catalog path]/[module name]/_releases/[release name].json
func (cat *Catalog) releaseFilePath(ref wfapi.CatalogRef) string {
	base := filepath.Dir(cat.moduleFilePath(ref))
	path := filepath.Join(base, "_releases", string(ref.ReleaseName))
	path = strings.Join([]string{path, ".json"}, "")
	return path
}

// Get the path for a CatalogReplay file.
// This will be [catalog path]/[module name]/_replays/[release name].json
func (cat *Catalog) replayFilePath(ref wfapi.CatalogRef) (string, error) {
	base := filepath.Dir(cat.moduleFilePath(ref))
	release, err := cat.GetRelease(ref)
	if err != nil {
		return "", err
	}
	path := filepath.Join(base, "_replays", string(release.Metadata.Values["replay"]))
	path = strings.Join([]string{path, ".json"}, "")
	return path, nil
}

// Update a Catalog's list of modules.
// This will be called when opening the catalog, and after updating the filesystem.
func (cat *Catalog) updateModuleList() error {
	// clear the list
	cat.moduleList = []wfapi.ModuleName{}

	// recursively assemble module names
	// modules names are subdirectories of the catalog (e.g., "catalog/example.com/package/module")
	// we need to recurse through each directory until we hit a "_module.json"
	return recurseModuleDir(cat, cat.path, "")
}

func recurseModuleDir(cat *Catalog, basePath string, moduleName wfapi.ModuleName) error {
	// list the directory
	thisDir := filepath.Join(basePath, string(moduleName))
	files, errRaw := fs.ReadDir(cat.fsys, thisDir)
	if errRaw != nil {
		return wfapi.ErrorIo("could not read catalog directory", cat.path, errRaw)
	}

	// check if a "_module.json" exists
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
func (cat *Catalog) GetRelease(ref wfapi.CatalogRef) (*wfapi.CatalogRelease, error) {
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
	fmt.Println(releasePath)
	releaseBytes, errRaw := fs.ReadFile(cat.fsys, releasePath)
	if os.IsNotExist(errRaw) {
		return nil, nil
	}
	if errRaw != nil {
		return nil, wfapi.ErrorIo("failed to read catalog release file", releasePath, errRaw)
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
//     - warpforge-error-catalog-invalid -- when catalog files are not found
//     - warpforge-error-catalog-missing-entry -- when catalog item is not found
func (cat *Catalog) GetWare(ref wfapi.CatalogRef) (*wfapi.WareID, *wfapi.WarehouseAddr, error) {
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
		// it doesn't make sense to check for a replay when we don't have an ID
		return nil, nil, wfapi.ErrorMissingCatalogEntry(ref, false)
	}

	// item found, check for a matching mirror
	mirror, err := cat.GetMirror(ref)
	if err != nil {
		return nil, nil, err
	}

	// resolve which WarehouseAddr will get returned for this particular ware
	// this is done by first looking for a specific ByWare mirror for the WareID
	// if none exists, the ByModule address is returned if it exists
	// TODO: handling of multiple mirrors
	if mirror == nil {
		// no mirror exists at all, return nil
		return &wareId, nil, nil
	}

	// check ByWare for a matching WareId
	if mirror.ByWare != nil {
		if addrs, exists := mirror.ByWare.Values[wareId]; exists {
			// match found, return it
			return &wareId, &addrs[0], nil
		}
	}

	// check if we can return a ByModule WarehouseAddress
	// note, this address may not actually contain the ware we're looking for
	if mirror.ByModule != nil {
		if len(mirror.ByModule.Values[ref.ModuleName].Values[wareId.Packtype]) > 0 {
			return &wareId, &mirror.ByModule.Values[ref.ModuleName].Values[wareId.Packtype][0], nil
		}
		return &wareId, nil, nil
	}

	// we have exhausted our options, no mirror exists
	// this is not an error, but we will return a nil WarehouseAddr
	return &wareId, nil, nil
}

// Get a catalog mirror for a given catalog reference.
//
// Errors:
//
//    - warpforge-error-io -- when reading lineage file fails
//    - warpforge-error-catalog-parse -- when ipld parsing of mirror file fails
func (cat *Catalog) GetMirror(ref wfapi.CatalogRef) (*wfapi.CatalogMirrors, error) {
	// open lineage file
	mirrorPath := cat.mirrorFilePath(ref)
	var mirrorFile fs.File
	var err error
	if mirrorFile, err = cat.fsys.Open(mirrorPath); os.IsNotExist(err) {
		// no mirror file for this ware
		return nil, nil
	} else if err != nil {
		return nil, wfapi.ErrorIo("error opening mirror file", mirrorPath, err)
	}

	// read and unmarshal mirror data
	mirrorBytes, err := ioutil.ReadAll(mirrorFile)
	if err != nil {
		return nil, wfapi.ErrorIo("failed to read mirror file", mirrorPath, err)
	}
	mirrorCapsule := wfapi.CatalogMirrorsCapsule{}
	_, err = ipld.Unmarshal(mirrorBytes, json.Decode, &mirrorCapsule, wfapi.TypeSystem.TypeByName("CatalogMirrorsCapsule"))
	if err != nil {
		return nil, wfapi.ErrorCatalogParse(mirrorPath, err)
	}
	if mirrorCapsule.CatalogMirrors == nil {
		return nil, wfapi.ErrorCatalogParse(mirrorPath, fmt.Errorf("no v1 CatalogMirrors in capsule"))
	}

	return mirrorCapsule.CatalogMirrors, nil
}

// Get a module for a given catalog reference.
//
// Errors:
//
//    - warpforge-error-io -- when reading module file fails
//    - warpforge-error-catalog-parse -- when ipld unmarshaling fails
func (cat *Catalog) GetModule(ref wfapi.CatalogRef) (*wfapi.CatalogModule, error) {
	// open module file
	modPath := cat.moduleFilePath(ref)
	var modFile fs.File
	var err error
	if modFile, err = cat.fsys.Open(modPath); os.IsNotExist(err) {
		// module file does not exist for this workspace
		return nil, nil
	} else if err != nil {
		return nil, wfapi.ErrorIo("error opening module file", modPath, err)
	}

	// read and unmarshal module data
	modBytes, err := ioutil.ReadAll(modFile)
	if err != nil {
		return nil, wfapi.ErrorIo("error reading module file", modPath, err)
	}

	modCapsule := wfapi.CatalogModuleCapsule{}
	_, err = ipld.Unmarshal(modBytes, json.Decode, &modCapsule, wfapi.TypeSystem.TypeByName("CatalogModuleCapsule"))
	if err != nil {
		return nil, wfapi.ErrorCatalogParse(modPath, err)
	}
	if modCapsule.CatalogModule == nil {
		return nil, wfapi.ErrorCatalogParse(modPath, fmt.Errorf("no v1 CatalogModule in CatalogModuleCapsule"))
	}

	return modCapsule.CatalogModule, nil
}

// Adds a new item to the catalog.
//
// Errors:
//
//    - warpforge-error-catalog-parse -- when parsing of the lineage file fails
//    - warpforge-error-io -- when reading or writing the lineage file fails
//    - warpforge-error-serialization -- when serializing the lineage fails
//    - warpforge-error-catalog-invalid -- when an error occurs while searching for module or release
//    - warpforge-error-already-exists -- when trying to insert an already existing item
func (cat *Catalog) AddItem(
	ref wfapi.CatalogRef,
	wareId wfapi.WareID,
	overwrite bool) error {

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
			ReleaseName: ref.ReleaseName,
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

	// check if the item exists
	_, hasItem := release.Items.Values[ref.ItemName]
	if hasItem && !overwrite {
		// item exists but overwrite not requested, error
		return wfapi.ErrorCatalogItemAlreadyExists(releaseFilePath, ref.ItemName)
	}
	if !hasItem {
		// item does not exist, add key
		release.Items.Keys = append(release.Items.Keys, ref.ItemName)
	}
	// update the item wareID
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
			return wfapi.ErrorIo("failed to create releases directory", releasesPath, errRaw)
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

	// sort the release list
	releaseList := []string{}
	for _, r := range module.Releases.Keys {
		releaseList = append(releaseList, string(r))
	}
	natsort.Sort(releaseList)
	module.Releases.Keys = []wfapi.ReleaseName{}
	for _, r := range releaseList {
		module.Releases.Keys = append(module.Releases.Keys, wfapi.ReleaseName(r))
	}

	// serialize the updated structures
	modCapsule := wfapi.CatalogModuleCapsule{CatalogModule: module}
	moduleSerial, errRaw := ipld.Marshal(json.Encode, &modCapsule, wfapi.TypeSystem.TypeByName("CatalogModuleCapsule"))
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
		return wfapi.ErrorIo("failed to write module file", moduleFilePath, errRaw)
	}
	errRaw = os.WriteFile(releaseFilePath, releaseSerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write release file", releaseFilePath, err)
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
	mirrorsPath := cat.mirrorFilePath(ref)

	var mirrorsCapsule wfapi.CatalogMirrorsCapsule
	mirrorsBytes, err := fs.ReadFile(cat.fsys, mirrorsPath)
	if os.IsNotExist(err) {
		mirrorsCapsule = wfapi.CatalogMirrorsCapsule{
			CatalogMirrors: &wfapi.CatalogMirrors{
				ByWare: &wfapi.CatalogMirrorsByWare{},
			},
		}
	} else if err == nil {
		_, err = ipld.Unmarshal(mirrorsBytes, json.Decode, &mirrorsCapsule, wfapi.TypeSystem.TypeByName("CatalogMirrorsCapsule"))
		if err != nil {
			return wfapi.ErrorCatalogParse(mirrorsPath, err)
		}
		if mirrorsCapsule.CatalogMirrors == nil {
			return wfapi.ErrorCatalogParse(mirrorsPath, fmt.Errorf("no v1 CatalogMirrors in CatalogMirrorsCapsule"))
		}
	} else {
		return wfapi.ErrorIo("could not open mirrors file", mirrorsPath, err)
	}

	mirrors := mirrorsCapsule.CatalogMirrors

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
		return wfapi.ErrorIo("could not create catalog path", path, err)
	}
	mirrorSerial, err := ipld.Marshal(json.Encode, &mirrorsCapsule, wfapi.TypeSystem.TypeByName("CatalogMirrorsCapsule"))
	if err != nil {
		return wfapi.ErrorSerialization("could not serialize mirror", err)
	}
	os.WriteFile(filepath.Join("/", mirrorsPath), mirrorSerial, 0644)
	if err != nil {
		return wfapi.ErrorIo("failed to write mirrors file", mirrorsPath, err)
	}

	return nil
}

// Adds a ByModule mirror to a catalog entry
//
// Errors:
//
//    - warpforge-error-io -- when reading or writing mirror file fails
//    - warpforge-error-catalog-parse -- when parsing existing mirror file fails
//    - warpforge-error-serialization -- when serializing the new mirror file fails
//    - warpforge-error-catalog-invalid -- when the wrong type of mirror file already exists
func (cat *Catalog) AddByModuleMirror(
	ref wfapi.CatalogRef,
	packType wfapi.Packtype,
	addr wfapi.WarehouseAddr) error {
	// load mirrors file, or create it if it doesn't exist
	mirrorsPath := cat.mirrorFilePath(ref)

	var mirrorsCapsule wfapi.CatalogMirrorsCapsule
	mirrorsBytes, err := fs.ReadFile(cat.fsys, mirrorsPath)
	if os.IsNotExist(err) {
		mirrorsCapsule = wfapi.CatalogMirrorsCapsule{
			CatalogMirrors: &wfapi.CatalogMirrors{
				ByModule: &wfapi.CatalogMirrorsByModule{
					Values: make(map[wfapi.ModuleName]wfapi.CatalogMirrorsByPacktype),
				},
			},
		}
	} else if err == nil {
		_, err = ipld.Unmarshal(mirrorsBytes, json.Decode, &mirrorsCapsule, wfapi.TypeSystem.TypeByName("CatalogMirrorsCapsule"))
		if err != nil {
			return wfapi.ErrorCatalogParse(mirrorsPath, err)
		}
	} else {
		return wfapi.ErrorIo("could not open mirrors file", mirrorsPath, err)
	}

	mirrors := mirrorsCapsule.CatalogMirrors

	// add this item to the mirrors file
	if mirrors.ByModule == nil {
		return wfapi.ErrorCatalogInvalid(mirrorsPath, "existing mirrors file is not of type ByModule")
	}

	// ensure mirror file has this module
	_, hasModule := mirrors.ByModule.Values[ref.ModuleName]
	if !hasModule {
		mirrors.ByModule.Keys = append(mirrors.ByModule.Keys, ref.ModuleName)
		mirrors.ByModule.Values[ref.ModuleName] = wfapi.CatalogMirrorsByPacktype{
			Values: make(map[wfapi.Packtype][]wfapi.WarehouseAddr),
		}
	}

	// ensure module has this packtype
	_, hasPacktype := mirrors.ByModule.Values[ref.ModuleName].Values[packType]
	if !hasPacktype {
		m := mirrors.ByModule.Values[ref.ModuleName]
		m.Keys = append(m.Keys, wfapi.Packtype(packType))
		m.Values[packType] = []wfapi.WarehouseAddr{}
		mirrors.ByModule.Values[ref.ModuleName] = m
	}

	mirrorList := mirrors.ByModule.Values[ref.ModuleName].Values[packType]
	// avoid adding duplicate values
	mirrorExists := false
	for _, m := range mirrorList {
		if m == addr {
			mirrorExists = true
			break
		}
	}
	if !mirrorExists {
		mirrorList = append(mirrorList, addr)
		mirrors.ByModule.Values[ref.ModuleName].Values[packType] = mirrorList
	}

	// write the updated data to the mirrors file
	path := filepath.Join("/", filepath.Dir(mirrorsPath))
	err = os.MkdirAll(path, 0755)
	if err != nil {
		return wfapi.ErrorIo("could not create catalog path", path, err)
	}
	mirrorSerial, err := ipld.Marshal(json.Encode, &mirrorsCapsule, wfapi.TypeSystem.TypeByName("CatalogMirrorsCapsule"))
	if err != nil {
		return wfapi.ErrorSerialization("could not serialize mirror", err)
	}
	os.WriteFile(filepath.Join("/", mirrorsPath), mirrorSerial, 0644)
	if err != nil {
		return wfapi.ErrorIo("failed to write mirrors file", mirrorsPath, err)
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
func (cat *Catalog) GetReplay(ref wfapi.CatalogRef) (*wfapi.Plot, error) {
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

	replayPath, err := cat.replayFilePath(ref)
	if err != nil {
		return nil, err
	}

	replayBytes, errRaw := fs.ReadFile(cat.fsys, replayPath)
	if os.IsNotExist(errRaw) {
		return nil, wfapi.ErrorCatalogInvalid(replayPath, "referenced replay file does not exist")
	} else if errRaw != nil {
		return nil, wfapi.ErrorIo("could not stat replay file", replayPath, errRaw)
	}

	replayCapsule := wfapi.ReplayCapsule{}
	_, errRaw = ipld.Unmarshal(replayBytes, json.Decode, &replayCapsule, wfapi.TypeSystem.TypeByName("ReplayCapsule"))
	if errRaw != nil {
		return nil, wfapi.ErrorCatalogParse(replayPath, err)
	}
	if replayCapsule.Plot == nil {
		return nil, wfapi.ErrorCatalogParse(replayPath, fmt.Errorf("no v1 Plot in ReplayCapsule"))
	}
	replay := replayCapsule.Plot

	// ensure the CID matches the expected value
	if replay.Cid() != wfapi.PlotCID(replayCid) {
		return nil, wfapi.ErrorCatalogInvalid(replayPath,
			fmt.Sprintf("expected CID %q for plot, actual CID is %q", replayCid, replay.Cid()))
	}

	return replay, nil
}

// Add a replay plot to a catalog.
//
// Errors:
//
//    - warpforge-error-catalog-invalid -- when the contents of the catalog is invalid
//    - warpforge-error-catalog-parse -- when parsing of catalog data fails
//    - warpforge-error-io -- when an io error occurs while opening the catalog
//    - warpforge-error-serialization -- when the updated structures fail to serialize
func (cat *Catalog) AddReplay(ref wfapi.CatalogRef, plot wfapi.Plot, overwrite bool) error {
	// first, update the CatalogRelease to add the replay CID
	// note: this must be done first, since we read this file to determine the replay file name!

	releasePath := filepath.Join("/", cat.releaseFilePath(ref))

	// get the current release contents
	release, err := cat.GetRelease(ref)
	if err != nil {
		return err
	}
	if release == nil {
		return wfapi.ErrorCatalogInvalid(releasePath, "release does not exist")
	}

	// check if a replay already exists
	_, hasReplay := release.Metadata.Values["replay"]
	if hasReplay && !overwrite {
		// replay exists and we do not want to overwrite it, fail
		return wfapi.ErrorCatalogInvalid(releasePath, "release already has replay")
	} else if !hasReplay {
		// no replay exists, update the metadata to add it
		release.Metadata.Keys = append(release.Metadata.Keys, "replay")
	}
	// update the replay value
	release.Metadata.Values["replay"] = string(plot.Cid())

	// serialize the CatalogRelease and write the file
	releaseSerial, errRaw := ipld.Marshal(json.Encode, release, wfapi.TypeSystem.TypeByName("CatalogRelease"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize release", errRaw)
	}
	errRaw = os.WriteFile(releasePath, releaseSerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write release file", releasePath, errRaw)
	}

	// next, we will need to update the CatalogRelease CID in the CatalogModule

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
	moduleCapsule := wfapi.CatalogModuleCapsule{CatalogModule: module}
	moduleSerial, errRaw := ipld.Marshal(json.Encode, &moduleCapsule, wfapi.TypeSystem.TypeByName("CatalogModuleCapsule"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize module", errRaw)
	}
	errRaw = os.WriteFile(moduleFilePath, moduleSerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write module file", moduleFilePath, errRaw)
	}

	// finally, write the Plot to the replay file

	replayPath, err := cat.replayFilePath(ref)
	if err != nil {
		return err
	}
	replayPath = filepath.Join("/", replayPath)

	// determine where the replay should be stored, and create the dir if it does not exist
	replayDir := filepath.Dir(replayPath)
	errRaw = os.MkdirAll(replayDir, 0755)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to create replays directory", replayDir, errRaw)
	}

	// serialize the replay Plot and write the file
	plotCapsule := wfapi.PlotCapsule{Plot: &plot}
	replaySerial, errRaw := ipld.Marshal(json.Encode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if errRaw != nil {
		return wfapi.ErrorSerialization("failed to serialize replay", errRaw)
	}
	errRaw = os.WriteFile(replayPath, replaySerial, 0644)
	if errRaw != nil {
		return wfapi.ErrorIo("failed to write replay file", replayPath, errRaw)
	}

	return nil
}
