package config

import (
	"os"
	"path/filepath"

	"github.com/warptools/warpforge/pkg/formulaexec"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/wfapi"

	"github.com/serum-errors/go-serum"
)

// Errors:
//
//    - warpforge-error-initialization -- unable to get executable directory
func BinPath() (string, error) {
	// determine the path of the running executable
	// other binaries (runc, rio) will be located here as well
	path, ok := os.LookupEnv(EnvWarpforgePath)
	if ok {
		return path, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", serum.Error(wfapi.ECodeInitialization,
			serum.WithMessageLiteral("failed to locate executable path"),
			serum.WithCause(err),
		)
	}
	return filepath.Dir(executable), nil
}

func KeepRunDir() bool {
	_, ok := os.LookupEnv(EnvWarpforgeKeepRundir)
	return ok
}

func RunPathBase() string {
	if value, ok := os.LookupEnv(EnvWarpforgeRunPath); ok {
		return value
	}
	return os.TempDir()
}

func WarehousePathOverride() *string {
	value, ok := os.LookupEnv(EnvWarpforgeWarehouse)
	if !ok {
		return nil
	}
	return &value
}

// Errors:
//
//    - warpforge-error-initialization -- unable to get working or executable directories
func PlotExecConfig() (plotexec.ExecConfig, error) {
	cfg, err := FormulaExecConfig()
	return plotexec.ExecConfig(cfg), err
}

// Errors:
//
//    - warpforge-error-initialization -- unable to get working or executable directories
func FormulaExecConfig() (cfg formulaexec.ExecConfig, _ error) {
	binpath, err := BinPath()
	if err != nil {
		return cfg, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return cfg, serum.Error(wfapi.ECodeInitialization,
			serum.WithMessageLiteral("unable to get working directory"),
			serum.WithCause(err),
		)
	}
	return formulaexec.ExecConfig{
		BinPath:          binpath,
		KeepRunDir:       KeepRunDir(),
		RunPathBase:      RunPathBase(),
		WhPathOverride:   WarehousePathOverride(),
		WorkingDirectory: wd,
	}, nil
}
