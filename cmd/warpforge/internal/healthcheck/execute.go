package healthcheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warpfork/warpforge/cmd/warpforge/internal/catalog"
	"github.com/warpfork/warpforge/cmd/warpforge/internal/util"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
)

type ExecutionInfo struct {
	// The directory where a workspace directory will be created
	BasePath string
	// Prefix for the root workspace directory
	TmpDirPrefix string
	// The root workspace directory
	RootWorkspaceDir  string
	LocalWorkspaceDir string
}

func (e *ExecutionInfo) String() string {
	path := e.RootWorkspaceDir
	if path == "" {
		path = e.BasePath
	}
	if path == "" {
		path = DefaultBasePath
	}
	return fmt.Sprintf("Execute: %s", path)
}

// Run will configure and execute a basic warpforge plot to check for errors
//
// Errors:
//
//    - warpforge-error-healthcheck-run-okay --
//    - warpforge-error-healthcheck-run-fail --
func (e *ExecutionInfo) Run(ctx context.Context) error {
	if e.BasePath == "" {
		e.BasePath = DefaultBasePath
	}
	if e.TmpDirPrefix == "" {
		e.TmpDirPrefix = DefaultRootWorkspacePrefix
	}

	rootWorkspaceDir, err := os.MkdirTemp(e.BasePath, e.TmpDirPrefix)
	if err != nil {
		return serum.Errorf(CodeRunFailure, "failed to make temporary directory inside path, %q: %w", e.BasePath, err)
	}

	if err := workspace.PlaceWorkspace(rootWorkspaceDir, workspace.SetRootWorkspaceOpt()); err != nil {
		return serum.Errorf(CodeRunFailure, "failed to create root workspace: %w", err)
	}

	localWorkspaceDir := filepath.Join(rootWorkspaceDir, "local")
	if err := os.Mkdir(localWorkspaceDir, 0755|os.ModeDir); err != nil {
		return serum.Errorf(CodeRunFailure, "failed to make directory: %q: %w", localWorkspaceDir, err)
	}

	if err := workspace.PlaceWorkspace(localWorkspaceDir); err != nil {
		return serum.Errorf(CodeRunFailure, "failed to create local workspace at path: %q: %w", localWorkspaceDir, err)
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", localWorkspaceDir[1:])
	if err != nil {
		return serum.Errorf(CodeRunFailure, "unable to find workspace stack: %w", err)
	}

	// localWs := wss.Local()

	// if err := localWs.CreateCatalog(""); err != nil {
	// 	return serum.Errorf(CodeRunFailure, "unable to create local catalog: %w", err)
	// }

	// if err := os.Chdir(localWorkspaceDir); err != nil {
	// 	return serum.Errorf(CodeRunFailure, "failed to change directories: %q: %w", localWorkspaceDir, err)
	// }

	moduleCapsule := wfapi.ModuleCapsule{
		Module: &wfapi.Module{
			Name: wfapi.ModuleName("warpforge-internal-healthcheck"),
		},
	}

	moduleSerial, err := ipld.Marshal(json.Encode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	if err != nil {
		return serum.Errorf(CodeRunFailure, "failed to serialize module: %w", err)
	}

	modulePath := filepath.Join(localWorkspaceDir, util.ModuleFilename)
	if err := os.WriteFile(modulePath, moduleSerial, 0644); err != nil {
		return serum.Errorf(CodeRunFailure, "failed to write module file: %q: %w", modulePath, err)
	}

	plotCapsule := wfapi.PlotCapsule{}
	_, err = ipld.Unmarshal([]byte(util.DefaultPlotJson), json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return serum.Errorf(CodeRunFailure, "failed to deserialize default plot: %w", err)
	}

	plotSerial, err := ipld.Marshal(json.Encode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return serum.Errorf(CodeRunFailure, "failed to serialize plot: %w", err)
	}

	plotFilePath := filepath.Join(localWorkspaceDir, util.PlotFilename)
	if err := os.WriteFile(plotFilePath, plotSerial, 0644); err != nil {
		return serum.Errorf(CodeRunFailure, "failed to write plot file: %q: %w", plotFilePath, err)
	}

	catalogPath := filepath.Join("/", wss.Root().CatalogBasePath())
	if err := catalog.InstallDefaultRemoteCatalog(ctx, catalogPath); err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err))
	}
	if plotCapsule.Plot == nil {
		return serum.Errorf(CodeRunFailure, "Execution failed: plot capsule missing plot")
	}
	if err := wss.Tidy(ctx, *plotCapsule.Plot, true); err != nil {
		return serum.Errorf(CodeRunFailure, "Execution failed: %w", err)
	}

	config := wfapi.PlotExecConfig{
		Recursive: true,
		FormulaExecConfig: wfapi.FormulaExecConfig{
			DisableMemoization: true,
		},
	}

	if _, err := util.ExecModule(ctx, config, util.ModuleFilename); err != nil {
		return serum.Errorf(CodeRunFailure, "Execution failed: %w", err)
	}
	return serum.Errorf(CodeRunOkay, "Execution Successful")
}
