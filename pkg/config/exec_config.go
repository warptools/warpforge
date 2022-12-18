package config

import (
	"os"
	"path/filepath"

	"github.com/warptools/warpforge/pkg/formulaexec"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/pkg/workspace"
)

func BinPath(state State) string {
	// determine the path of the running executable
	// other binaries (runc, rio) will be located here as well
	path, ok := state.Env[EnvWarpforgePath]
	if ok {
		return path
	}
	return filepath.Dir(state.ExecutablePath)
}

func KeepRunDir(state State) bool {
	_, ok := state.Env[EnvWarpforgeKeepRundir]
	return ok
}

func RunPathBase(state State) string {
	if value, ok := state.Env[EnvWarpforgeRunPath]; ok {
		return value
	}
	return state.TempDir
}

func WarehousePathOverride(state State) *string {
	value, ok := state.Env[EnvWarpforgeWarehouse]
	if !ok {
		return nil
	}
	return &value
}

// Retrieves workspace stack at state.WorkingDirectory
// Errors:
//
//  - warpforge-error-searching-filesystem -- unexpected error traversing filesystem
func DefaultWorkspaceStack(state State) (workspace.WorkspaceSet, error) {
	fsys := os.DirFS("/")
	wss, err := workspace.FindWorkspaceStack(fsys, "", state.WorkingDirectory[1:])
	if err != nil {
		return nil, err
	}
	return wss, nil
}

func PlotExecConfig(state State) plotexec.ExecConfig {
	return plotexec.ExecConfig(FormulaExecConfig(state))
}

func FormulaExecConfig(state State) formulaexec.ExecConfig {
	return formulaexec.ExecConfig{
		BinPath:          BinPath(state),
		KeepRunDir:       KeepRunDir(state),
		RunPathBase:      RunPathBase(state),
		WhPathOverride:   WarehousePathOverride(state),
		WorkingDirectory: state.WorkingDirectory,
	}
}
