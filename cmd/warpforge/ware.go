package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/trace"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

var wareCmdDef = cli.Command{
	Name:  "ware",
	Usage: "Subcommands that operate on wares",
	Subcommands: []*cli.Command{
		{
			Name:  "unpack",
			Usage: "Places the contents of a ware onto the filesystem",
			Description: strings.Join([]string{
				`[WareID]: a ware ID such as [packtype]:[hash]. e.g. "tar:4z9DCTxoKkStqXQRwtf9nimpfQQ36dbndDsAPCQgECfbXt3edanUrsVKCjE9TkX2v9"`,
			}, "\n"),
			Flags: []cli.Flag{
				&cli.PathFlag{
					Name:  "path",
					Usage: "Location to place the ware contents. Defaults to current directory.",
				},
				&cli.BoolFlag{
					Name:  "force",
					Usage: "Allow overwriting the given path. Any contents of an existing directory may be lost.",
				},
			},
			Action: util.ChainCmdMiddleware(cmdWareUnpack,
				util.CmdMiddlewareLogging,
				util.CmdMiddlewareTracingConfig,
				util.CmdMiddlewareTracingSpan,
			),
			ArgsUsage: "[WareID]",
			// CustomHelpTemplate: cli.SubcommandHelpTemplate,
		},
	},
}

func cmdWareUnpack(c *cli.Context) error {
	log := logging.Ctx(c.Context)
	if c.Args().Len() != 1 {
		cli.ShowCommandHelp(c, "unpack")
		return fmt.Errorf("invalid number of arguments")
	}
	args := c.Args().Slice()
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	extraWarehouses := []string{}
	warehouse := os.Getenv("WARPFORGE_WAREHOUSE")
	log.Debug("", "WARPFORGE_WAREHOUSE=%q", warehouse)
	if warehouse != "" {
		extraWarehouses = append(extraWarehouses, "ca+file://"+warehouse)
	}

	config := &wareUnpackConfig{
		Ref:        args[0],
		Path:       c.Path("path"),
		Pwd:        pwd,
		Force:      c.Bool("force"),
		Warehouses: extraWarehouses,
	}
	return config.run(c.Context)
}

type wareUnpackConfig struct {
	Ref        string
	Path       string
	Pwd        string
	Force      bool
	Warehouses []string
}

func (c *wareUnpackConfig) run(ctx context.Context) error {
	log := logging.Ctx(ctx)
	wareID, err := wareRefDecode(c.Ref)
	if err != nil {
		return err
	}
	log.Debug("", "wareID: %s", wareID.String())

	path := c.Path
	if path == "" {
		path = c.Pwd
	}
	if err := c.validatePath(); err != nil {
		return err
	}

	addrs, err := c.sources()
	if err != nil {
		return err
	}
	log.Debug("", "sources: %v", addrs)
	return rioUnpack(ctx, wareID, path, addrs)
}

func (c *wareUnpackConfig) sources() ([]wfapi.WarehouseAddr, error) {
	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", c.Pwd[1:])
	if err != nil {
		return nil, err
	}
	addrs := wss.GetWarehouseAddresses()
	sources := make([]wfapi.WarehouseAddr, 0, len(c.Warehouses))
	for _, w := range c.Warehouses {
		sources = append(sources, wfapi.WarehouseAddr(w))
	}
	sources = append(sources, addrs...)
	return sources, nil
}

func wareRefDecode(ref string) (wfapi.WareID, error) {
	//TODO: check for catalog references and convert them to WareID

	// It would be nice if ipld libraries had reasonable examples on how to deserialize a repr to a type
	// Instead, we'll just do it manually instead of wasting additional hours of time.
	refSplit := strings.Split(ref, ":")
	if len(refSplit) != 2 {
		return wfapi.WareID{}, wfapi.ErrorSerialization(ref, fmt.Errorf("ref is not a valid ware ID"))
	}
	result := wfapi.WareID{
		Packtype: wfapi.Packtype(refSplit[0]),
		Hash:     refSplit[1],
	}
	return result, nil
}

// validatePath checks whether or not a ware can be unpacked to path
// validation is required because the placer may delete the contents of any existing path.
// returns nil if force is true
// returns nil if the path does not exist
// returns os.ErrExist if the path is not a directory or is a directory that contains any entries
// returns other errors if os.ReadDir fails on path
func (c *wareUnpackConfig) validatePath() error {
	if c.Force {
		return nil
	}
	path := c.Path
	if path == "" {
		path = c.Pwd
	}
	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if !fi.IsDir() {
		return fmt.Errorf("path is not a directory: %w", os.ErrExist)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("path is a non-empty directory: %w", os.ErrExist)
	}
	return nil
}

func rioUnpack(ctx context.Context, wareID wfapi.WareID, path string, addrs []wfapi.WarehouseAddr) error {
	if len(addrs) == 0 {
		return fmt.Errorf("must provide at least one warehouse address")
	}
	log := logging.Ctx(ctx)
	rioArgs := []string{
		"unpack",
		"--placer=direct",
	}
	for _, a := range addrs {
		rioArgs = append(rioArgs, "--source="+string(a))
	}
	rioArgs = append(rioArgs, wareID.String(), path)

	rioPath, err := util.BinPath("rio")
	if err != nil {
		return fmt.Errorf("failed to get path to rio")
	}
	cmdCtx, cmdSpan := tracing.Start(ctx, "rio unpack", trace.WithAttributes(tracing.AttrFullExecNameRio))
	defer cmdSpan.End()
	rioCmd := exec.CommandContext(
		cmdCtx, rioPath, rioArgs...,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rioCmd.Stdout = &stdout
	rioCmd.Stderr = &stderr
	log.Debug("", "execute rio: %s", rioCmd.Args)
	cmdErr := rioCmd.Run()
	tracing.EndWithStatus(cmdSpan, cmdErr)
	if cmdErr != nil {
		return fmt.Errorf("failed to run rio unpack command: %s\n%s", cmdErr, stderr.String())
	}

	return nil
}
