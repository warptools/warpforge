package mirroring

import (
	"fmt"
	"os"
	"strings"

	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

type publisher interface {
	hasWare(wfapi.WareID) (bool, error)
	publishWare(wfapi.WareID, string) error
}

func PushCatalogWares(ws workspace.Workspace, cat workspace.Catalog) error {
	for _, m := range cat.Modules() {
		ref := wfapi.CatalogRef{ModuleName: m}
		module, err := cat.GetModule(ref)
		if err != nil {
			return err
		}

		for _, r := range module.Releases.Keys {
			ref.ReleaseName = r
			rel, err := cat.GetRelease(ref)
			if err != nil {
				return err
			}
			for _, i := range rel.Items.Keys {
				ref.ItemName = i
				wareId, warehouseAddr, err := cat.GetWare(ref)
				if err != nil {
					return err
				}
				if warehouseAddr == nil {
					// we do not have a place to publish to, so skip this item
					continue
				}

				// get the path to the data we want to publish
				warePath, _ := ws.WarePath(*wareId)

				// it is possible we don't have this ware, in which case we just want to skip over it
				if _, err := os.Stat(warePath); os.IsNotExist(err) {
					fmt.Println("no file @", warePath)
					continue
				} else if err != nil {
					return err
				}

				// we have the data and an address to publish to!

				// determine which URI scheme is in use
				schemeSplit := strings.Split(string(*warehouseAddr), "://")
				if len(schemeSplit) < 2 {
					return fmt.Errorf("invalid URL: %q", *warehouseAddr)
				}
				scheme := schemeSplit[0]

				fmt.Println("publish:", ref.String(), " wareId =", wareId, "warehouseAddr =", *warehouseAddr, "warePath =", warePath, "scheme =", scheme)
				switch scheme {
				case "ca+s3":
					err := publishToS3(*warehouseAddr, *wareId, warePath)
					if err != nil {
						return err
					}
				case "https":
					// readonly scheme, no-op
					continue
				default:
					return fmt.Errorf("unsupported scheme %q", scheme)
				}
			}
		}
	}
	return nil
}
