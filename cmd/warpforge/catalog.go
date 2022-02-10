package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/plotexec"
	"github.com/warpfork/warpforge/wfapi"
)

const defaultCatalogUrl = "https://github.com/warpfork/wfcatalog"

var catalogCmdDef = cli.Command{
	Name:  "catalog",
	Usage: "Subcommands that operate on catalogs.",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "name",
			Aliases: []string{"n"},
			Usage:   "Name of the catalog to operate on",
		},
	},
	Subcommands: []*cli.Command{
		{
			Name:   "init",
			Usage:  "Initialize a catalog in the current directory.",
			Action: cmdCatalogInit,
		},
		{
			Name:   "add",
			Usage:  "Add an item to the catalog.",
			Action: cmdCatalogAdd,
		},
		{
			Name:   "insertreplay",
			Usage:  "Add a replay to the catalog.",
			Action: cmdCatalogInsertReplay,
		},
		{
			Name:   "ls",
			Usage:  "List available catalogs.",
			Action: cmdCatalogLs,
		},
		{
			Name:   "bundle",
			Usage:  "Bundle required catalog items into this project's catalog.",
			Action: cmdCatalogBundle,
		},
		{
			Name:   "update",
			Usage:  "Update remote catalogs.",
			Action: cmdCatalogUpdate,
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

func cmdCatalogInit(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("no catalog name provided")
	}
	catalogName := c.Args().First()

	// open the workspace set and get the catalog path
	wsSet, err := openWorkspaceSet()
	if err != nil {
		return err
	}
	catalogPath := filepath.Join("/", wsSet.Root.CatalogPath(&catalogName))

	// check if the catalog directory exists
	_, err = os.Stat(catalogPath)
	if os.IsNotExist(err) {
		// catalog does not exist, create the dir
		err = os.MkdirAll(catalogPath, 0755)
	} else {
		// catalog already exists
		return fmt.Errorf("catalog %q already exists (path: %q)", catalogName, catalogPath)
	}

	if err != nil {
		// stat or mkdir failed
		return fmt.Errorf("failed to create catalog: %s", err)
	}

	return nil
}

func cmdCatalogAdd(c *cli.Context) error {
	if c.Args().Len() != 3 {
		return fmt.Errorf("invalid input. usage: warpforge catalog add [pack type] [catalog ref] [url]")
	}

	catalogName := c.String("name")
	if catalogName == "" {
		catalogName = "default"
	}

	packType := c.Args().Get(0)
	catalogRefStr := c.Args().Get(1)
	url := c.Args().Get(2)

	// open the workspace set
	wsSet, err := openWorkspaceSet()
	if err != nil {
		return err
	}

	// create the catalog if it does not exist
	exists, err := wsSet.Root.HasCatalog(catalogName)
	if err != nil {
		return err
	}
	if !exists {
		err := wsSet.Root.CreateCatalog(catalogName)
		if err != nil {
			return err
		}
	}

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
		ReleaseName: wfapi.ReleaseName(releaseName),
		ItemName:    wfapi.ItemLabel(itemName),
	}

	// perform rio scan to determine the ware id of the provided item
	scanWareId, err := scanWareId(wfapi.Packtype(packType), wfapi.WarehouseAddr(url))
	if err != nil {
		return fmt.Errorf("scanning %q failed: %s", url, err)
	}

	// add the new item
	cat, err := wsSet.Root.OpenCatalog(&catalogName)
	if err != nil {
		return fmt.Errorf("failed to open catalog %q: %s", catalogName, err)
	}
	err = cat.AddItem(ref, scanWareId)
	if err != nil {
		return fmt.Errorf("failed to add item to catalog: %s", err)
	}
	err = cat.AddByWareMirror(ref, scanWareId, wfapi.WarehouseAddr(url))
	if err != nil {
		return fmt.Errorf("failed to add mirror: %s", err)
	}

	if c.Bool("verbose") {
		fmt.Fprintf(c.App.Writer, "added item to catalog %q\n", wsSet.Root.CatalogPath(&catalogName))
	}

	return nil
}

func cmdCatalogLs(c *cli.Context) error {
	wsSet, err := openWorkspaceSet()
	if err != nil {
		return err
	}

	// get the list of catalogs in this workspace
	catalogs, err := wsSet.Root.ListCatalogs()
	if err != nil {
		return fmt.Errorf("failed to list catalogs: %s", err)
	}

	// print the list
	for _, catalog := range catalogs {
		if catalog != nil {
			fmt.Fprintf(c.App.Writer, "%s\n", *catalog)
		}
	}
	return nil
}

func gatherCatalogRefs(plot wfapi.Plot) []wfapi.CatalogRef {
	refs := []wfapi.CatalogRef{}

	// gather this plot's inputs
	for _, input := range plot.Inputs.Values {
		if input.Basis().CatalogRef != nil {
			refs = append(refs, *input.Basis().CatalogRef)
		}
	}

	// gather subplot inputs
	for _, step := range plot.Steps.Values {
		if step.Plot != nil {
			// recursively gather the refs from subplot(s)
			newRefs := gatherCatalogRefs(*step.Plot)

			// deduplicate
			unique := true
			for _, newRef := range newRefs {
				for _, existingRef := range refs {
					if newRef == existingRef {
						unique = false
						break
					}
				}
				if unique {
					refs = append(refs, newRef)
				}
			}
		}
	}

	return refs
}

func cmdCatalogBundle(c *cli.Context) error {
	wsSet, err := openWorkspaceSet()
	if err != nil {
		return err
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get pwd: %s", err)
	}

	plot, err := plotFromFile(filepath.Join(pwd, PLOT_FILE_NAME))
	if err != nil {
		return err
	}

	refs := gatherCatalogRefs(plot)

	catalogPath := filepath.Join(pwd, ".warpforge", "catalog")
	// create a catalog if it does not exist
	if _, err = os.Stat(catalogPath); os.IsNotExist(err) {
		err = os.MkdirAll(catalogPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create catalog directory: %s", err)
		}

		// we need to reopen the workspace set after creating the directory
		wsSet, err = openWorkspaceSet()
		if err != nil {
			return err
		}
	}

	for _, ref := range refs {
		wareId, wareAddr, err := wsSet.Root.GetCatalogWare(ref)
		if err != nil {
			return err
		}

		if wareId == nil {
			return fmt.Errorf("could not find catalog entry for %s:%s:%s",
				ref.ModuleName, ref.ReleaseName, ref.ItemName)
		}

		fmt.Fprintf(c.App.Writer, "bundled \"%s:%s:%s\"\n", ref.ModuleName, ref.ReleaseName, ref.ItemName)
		cat, err := wsSet.Stack[0].OpenCatalog(nil)
		if err != nil {
			return fmt.Errorf("failed to open catalog: %s", err)
		}
		cat.AddItem(ref, *wareId)
		if wareAddr != nil {
			cat.AddByWareMirror(ref, *wareId, *wareAddr)
		}
	}

	return nil
}

func installDefaultRemoteCatalog(c *cli.Context, path string) error {
	// install our default remote catalog as "default-remote" by cloning from git
	// this will noop if the catalog already exists

	defaultCatalogPath := filepath.Join(path, "default-remote")
	if _, err := os.Stat(defaultCatalogPath); !os.IsNotExist(err) {
		// a dir exists for this catalog, do nothing
		return nil
	}

	fmt.Fprintf(c.App.Writer, "installing default catalog to %s...", defaultCatalogPath)
	_, err := git.PlainClone(defaultCatalogPath, false, &git.CloneOptions{
		URL: defaultCatalogUrl,
	})

	fmt.Fprintf(c.App.Writer, " done.\n")

	if err != nil {
		return err
	}

	return nil
}

func cmdCatalogUpdate(c *cli.Context) error {
	wss, err := openWorkspaceSet()
	if err != nil {
		return fmt.Errorf("failed to open workspace set: %s", err)
	}

	// get the catalog path for the root workspace
	catalogPath := filepath.Join("/", wss.Root.CatalogBasePath())
	// create the path if it does not exist
	if _, err := os.Stat(catalogPath); os.IsNotExist(err) {
		err = os.MkdirAll(catalogPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create catalog path: %s", err)
		}
	}

	err = installDefaultRemoteCatalog(c, catalogPath)
	if err != nil {
		return fmt.Errorf("failed to install default catalog: %s", err)
	}

	catalogs, err := os.ReadDir(catalogPath)
	if err != nil {
		return fmt.Errorf("failed to list catalog path: %s", err)
	}

	for _, cat := range catalogs {
		if !cat.IsDir() {
			// ignore non-directory items
			continue
		}

		path := filepath.Join(catalogPath, cat.Name())

		r, err := git.PlainOpen(path)
		if err == git.ErrRepositoryNotExists {
			fmt.Fprintf(c.App.Writer, "%s: local catalog\n", cat.Name())
			continue
		} else if err != nil {
			return fmt.Errorf("failed to open git repo: %s", err)
		}

		wt, err := r.Worktree()
		if err != nil {
			return fmt.Errorf("failed to open git worktree: %s", err)
		}

		err = wt.Pull(&git.PullOptions{})
		if err == git.NoErrAlreadyUpToDate {
			fmt.Fprintf(c.App.Writer, "%s: already up to date\n", cat.Name())
		} else if err != nil {
			return fmt.Errorf("failed to pull from git: %s", err)
		} else {
			fmt.Fprintf(c.App.Writer, "%s: updated\n", cat.Name())
		}
	}

	return nil
}

func cmdCatalogInsertReplay(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("invalid input. usage: warpforge catalog insertreplay [release name]")
	}
	catalogName := c.String("name")
	if catalogName == "" {
		catalogName = "default"
	}

	// open the workspace set
	wsSet, err := openWorkspaceSet()
	if err != nil {
		return err
	}

	// create the catalog if it does not exist
	exists, err := wsSet.Root.HasCatalog(catalogName)
	if err != nil {
		return err
	}
	if !exists {
		err := wsSet.Root.CreateCatalog(catalogName)
		if err != nil {
			return err
		}
	}

	// get the module, release, and item values (in format `module:release:item`)
	module, err := moduleFromFile("module.wf")
	if err != nil {
		return err
	}

	releaseName := c.Args().Get(0)

	fmt.Printf("building replay for module = %q, release = %q, executing plot...\n", module.Name, releaseName)
	plot, err := plotFromFile(PLOT_FILE_NAME)
	if err != nil {
		return err
	}
	results, err := plotexec.Exec(wsSet, plot, logging.NewLogger(c.App.Writer, c.App.ErrWriter, c.Bool("verbose")))
	if err != nil {
		return err
	}

	cat, err := wsSet.Root.OpenCatalog(&catalogName)
	if err != nil {
		return err
	}

	for itemName, wareId := range results.Values {
		ref := wfapi.CatalogRef{
			ModuleName:  module.Name,
			ReleaseName: wfapi.ReleaseName(releaseName),
			ItemName:    wfapi.ItemLabel(itemName),
		}

		fmt.Println(ref.String(), "->", wareId)
		err := cat.AddItem(ref, wareId)
		if err != nil {
			return err
		}
		err = cat.AddReplay(ref, plot)
		if err != nil {
			return err
		}

	}

	return nil
}
