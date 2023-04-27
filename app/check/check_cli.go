package checkcli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/json"
	"github.com/serum-errors/go-serum"
	"github.com/urfave/cli/v2"

	appbase "github.com/warptools/warpforge/app/base"
	"github.com/warptools/warpforge/app/base/util"
	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/wfapi"
)

func init() {
	appbase.App.Commands = append(appbase.App.Commands, checkCmdDef)
}

var checkCmdDef = &cli.Command{
	Name:  "check",
	Usage: "Check file(s) for syntax and sanity",
	Action: util.ChainCmdMiddleware(cmdCheck,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
		util.CmdMiddlewareTracingSpan,
	),
}

func checkModule(fsys fs.FS, fileName string) (*ipld.Node, error) {
	f, err := fs.ReadFile(fsys, fileName)
	if err != nil {
		return nil, wfapi.ErrorIo("cannot read module file", fileName, err)
	}

	moduleCapsule := wfapi.ModuleCapsule{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &moduleCapsule, wfapi.TypeSystem.TypeByName("ModuleCapsule"))
	if err != nil {
		return nil, wfapi.ErrorSerialization("cannot deserialize module", err)
	}

	if err := dab.ValidateModuleName(moduleCapsule.Module.Name); err != nil {
		return nil, err
	}

	return &n, nil
}

func checkPlot(ctx context.Context, fsys fs.FS, fileName string) (*ipld.Node, error) {
	f, err := fs.ReadFile(fsys, fileName)
	if err != nil {
		return nil, wfapi.ErrorIo("cannot read plot file", fileName, err)
	}

	// parse the Plot
	plotCapsule := wfapi.PlotCapsule{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &plotCapsule, wfapi.TypeSystem.TypeByName("PlotCapsule"))
	if err != nil {
		return nil, wfapi.ErrorSerialization("cannot deserialize plot", err)
	}
	if plotCapsule.Plot == nil {
		return nil, wfapi.ErrorPlotInvalid("missing Plot")
	}
	// ensure Plot order can be resolved
	if _, err := plotexec.OrderSteps(ctx, *plotCapsule.Plot); err != nil {
		return &n, err
	}

	return &n, nil
}

func checkFormula(fsys fs.FS, fileName string) (*ipld.Node, error) {
	f, err := fs.ReadFile(fsys, fileName)
	if err != nil {
		return nil, wfapi.ErrorIo("cannot read formula file", fileName, err)
	}

	frmAndCtx := wfapi.FormulaAndContext{}
	n, err := ipld.Unmarshal([]byte(f), json.Decode, &frmAndCtx, wfapi.TypeSystem.TypeByName("FormulaAndContext"))
	if err != nil {
		return nil, wfapi.ErrorSerialization("cannot deserialize formula", err)
	}
	return &n, nil
}

// Errors:
//
//   - warpforge-error-invalid --
//   - warpforge-error-io -- unable to get working directory
//   - warpforge-error-io -- unable to read files
//   - warpforge-error-module-invalid -- module data is invalid
//   - warpforge-error-plot-invalid -- plot data is invalid
//   - warpforge-error-serialization -- unable to parse files
//   - warpforge-error-invalid-argument -- cli argument is invalid
func cmdCheck(c *cli.Context) error {
	if !c.Args().Present() {
		return serum.Error(wfapi.ECodeArgument,
			serum.WithMessageLiteral("no input files provided"),
		)
	}
	ctx := c.Context
	pwd, err := os.Getwd()
	if err != nil {
		return serum.Errorf(wfapi.ECodeIo, "unable to get workding directory: %w", err)
	}
	for _, filename := range c.Args().Slice() {
		t, err := dab.GetFileType(filename)
		if err != nil {
			return err
		}

		fsys := os.DirFS(pwd)
		if strings.HasPrefix(filename, string(filepath.Separator)) {
			fsys = os.DirFS("/")
			filename = filename[1:]
		}

		var n *ipld.Node
		switch t {
		case dab.FileType_Formula:
			n, err = checkFormula(fsys, filename)
			if err != nil {
				return err
			}

		case dab.FileType_Plot:
			n, err = checkPlot(ctx, fsys, filename)
			if err != nil {
				return err
			}
		case dab.FileType_Module:
			n, err = checkModule(fsys, filename)
			if err != nil {
				return err
			}
		default:
			if c.Bool("verbose") {
				fmt.Fprintf(c.App.ErrWriter, "ignoring unrecognized file: %q\n", filename)
			}
			continue
		}
		if c.Bool("verbose") && n != nil {
			serial, err := ipld.Encode(*n, json.Encode)
			if err != nil {
				panic("failed to serialize output")
			}
			fmt.Fprintf(c.App.Writer, "%s\n", serial)
		}
	}

	return nil
}
