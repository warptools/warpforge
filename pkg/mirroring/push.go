package mirroring

import (
	"context"
	"os"

	"github.com/serum-errors/go-serum"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

type pusher interface {
	// Errors:
	//
	// 	- warpforge-error-io -- for IO errors that occur during push operations
	hasWare(wfapi.WareID) (bool, error)
	// Errors:
	//
	// 	- warpforge-error-io -- for IO errors that occur during push operations
	pushWare(wfapi.WareID, string) error
}

func pusherFromConfig(ctx context.Context, cfg wfapi.WarehouseMirroringConfig) (pusher, error) {
	if cfg.PushConfig.S3 != nil {
		pusher, err := newS3Pusher(ctx, *cfg.PushConfig.S3)
		return &pusher, err
	} else if cfg.PushConfig.Mock != nil {
		pusher, err := newMockPusher(ctx, *cfg.PushConfig.Mock)
		return &pusher, err
	}
	// this should be unreachable due to IPLD validation
	panic("no supported push configuration provided")
}

// PushToWarehouseAddr puts files into a mirror
//
// It requires a workspace and catalog to operate on, and the address and configuration
// for mirroring. The given catalog will be scanned for wares that can be pushed based on
// the configuration.
//
// Errors:
//
// 	- warpforge-error-io -- for IO errors that occur during push operations
//  - warpforge-error-catalog-invalid -- when the provided catalog contains invalid data
//  - warpforge-error-catalog-missing-entry -- should never occur, as we iterate over the contents of the catalog
//  - warpforge-error-catalog-parse -- when the provided catalog cannot be parsed
func PushToWarehouseAddr(ctx context.Context, ws workspace.Workspace, cat workspace.Catalog, pushAddr wfapi.WarehouseAddr, cfg wfapi.WarehouseMirroringConfig) error {
	log := logging.Ctx(ctx)

	pusher, err := pusherFromConfig(ctx, cfg)
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

				// get the path to the data we want to push
				warePath, _ := ws.WarePath(*wareId)

				// it is possible we don't have this ware, in which case we just want to skip over it
				if _, err := os.Stat(warePath); os.IsNotExist(err) {
					log.Debug("mirror", "no local copy of wareId %q (expected at %q), skipping", wareId.String(), warePath)
					continue
				} else if err != nil {
					return serum.Errorf(wfapi.ECodeIo, "failed to stat %q: %s", warePath, err)
				}

				// we have a ware to push!

				log.Info("mirror", "pushing ware: wareId = %s, warePath = %s, pushAddr = %s", wareId.String(), warePath, pushAddr)
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
					log.Debug("mirror", "bucket already has wareId %q, skipping", wareId.String())
				}
			}
		}
	}
	return nil
}
