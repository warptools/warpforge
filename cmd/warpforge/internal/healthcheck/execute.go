package healthcheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"

	"github.com/warptools/warpforge/cmd/warpforge/internal/catalog"
	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/config"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
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

	rootWorkspaceDir, xerr := os.MkdirTemp(e.BasePath, e.TmpDirPrefix)
	if xerr != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(xerr),
			serum.WithMessageTemplate("failed to make temporary directory inside path, {{basepath|q}}"),
			serum.WithDetail("basepath", e.BasePath),
		)
	}

	if err := workspace.PlaceWorkspace(rootWorkspaceDir, workspace.SetRootWorkspaceOpt()); err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageLiteral("failed to create root workspace"),
		)
	}

	localWorkspaceDir := filepath.Join(rootWorkspaceDir, "local")
	if err := os.Mkdir(localWorkspaceDir, 0755|os.ModeDir); err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageTemplate("failed to make directory: {{workspace}}"),
			serum.WithDetail("workspace", localWorkspaceDir),
		)
	}

	if err := workspace.PlaceWorkspace(localWorkspaceDir); err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageTemplate("failed to create local workspace at path: %q: %w"),
			serum.WithDetail("workspace", localWorkspaceDir),
		)
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", localWorkspaceDir[1:])
	if err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageLiteral("unable to find workspace stack: %w"),
		)
	}

	plotCapsule := wfapi.PlotCapsule{}
	if _, err := ipld.Unmarshal([]byte(util.DefaultPlotJson), json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule")); err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err), serum.WithMessageLiteral("failed to deserialize default plot"))
	}
	if plotCapsule.Plot == nil {
		return serum.Errorf(CodeRunFailure, "Execution failed: plot capsule missing plot")
	}

	catalogPath := filepath.Join("/", wss.Root().CatalogBasePath())
	if err := catalog.InstallDefaultRemoteCatalog(ctx, catalogPath); err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err))
	}
	if err := wss.Tidy(ctx, *plotCapsule.Plot, true); err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageLiteral("Execution failed"),
		)
	}

	pltCfg := wfapi.PlotExecConfig{
		Recursive: true,
		FormulaExecConfig: wfapi.FormulaExecConfig{
			DisableMemoization: true,
		},
	}

	exCfg, err := config.PlotExecConfig(&e.BasePath)
	if err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err))
	}
	result, err := plotexec.Exec(ctx, exCfg, wss, plotCapsule, pltCfg)
	if err != nil {
		return serum.Error(CodeRunFailure, serum.WithCause(err),
			serum.WithMessageLiteral("Execution failed"),
		)
	}

	invariant := wfapi.PlotResults{
		Keys: []wfapi.LocalLabel{"output"},
		Values: map[wfapi.LocalLabel]wfapi.WareID{
			"output": {Packtype: "tar", Hash: "6U2WhgnXRCLsNjZLyvLzG6Eer5MH4MpguDeimPrEafHytjmXjbvxjm1STCuqHV5AQA"},
		},
	}
	if !reflect.DeepEqual(result, invariant) {
		return serum.Errorf(CodeRunFailure, "unexpected output: %s", result)
	}

	return serum.Errorf(CodeRunOkay, "Execution Successful")
}
