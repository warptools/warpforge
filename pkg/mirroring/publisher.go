package mirroring

import (
	"fmt"
	"os"

	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

type pusher interface {
	hasWare(wfapi.WareID) (bool, error)
	pushWare(wfapi.WareID, string) error
}

func pusherFromConfig(cfg wfapi.WarehouseMirroringConfig) (pusher, error) {
	pusher, err := NewS3Publisher(*cfg.PushConfig.S3)
	return &pusher, err
}

func PushToWarehouseAddr(ws workspace.Workspace, cat workspace.Catalog, pushAddr wfapi.WarehouseAddr, cfg wfapi.WarehouseMirroringConfig) error {
	pusher, err := pusherFromConfig(cfg)
	if err != nil {
		return err
	}

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
				if warehouseAddr == nil || *warehouseAddr != pushAddr {
					// this ware's WarehouseAddr does not match the one we're pushing to,
					// ignore this ware
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

				// we have a ware to publish!

				fmt.Println("publish ware: wareId =", wareId, " warePath =", warePath, "pushAddr =", pushAddr)
				hasWare, err := pusher.hasWare(*wareId)
				if err != nil {
					return err
				}
				if !hasWare {
					err := pusher.pushWare(*wareId, warePath)
					if err != nil {
						return err
					}
				} else {
					fmt.Println("bucket already has ware", wareId)
				}
			}
		}
	}
	return nil
}
