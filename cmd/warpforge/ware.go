package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/trace"

	"github.com/warpfork/warpforge/cmd/warpforge/internal/util"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/tracing"
	"github.com/warpfork/warpforge/pkg/workspace"
	"github.com/warpfork/warpforge/wfapi"
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
	if c.Args().Len() != 1 {
		cli.ShowCommandHelp(c, "unpack")
		return fmt.Errorf("invalid number of arguments")
	}
	args := c.Args().Slice()
	ref := args[0]
	path := c.Path("path")
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	return wareUnpack(c.Context, ref, path, pwd)
}

func wareUnpack(ctx context.Context, ref string, path string, pwd string) error {
	log := logging.Ctx(ctx)
	wareID, err := unpackRefDecode(ref)
	if err != nil {
		return err
	}
	log.Debug("", "wareID: %s", wareID.String())

	if path == "" {
		path = pwd
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", pwd[1:])
	if err != nil {
		return err
	}
	//TODO: check the workspaces for additional mirrors to a ware.
	addrs := wss.GetWarehouseAddresses()
	log.Debug("", "sources: %v", addrs)
	return rioUnpack(ctx, wareID, path, addrs)
}

func unpackRefDecode(ref string) (wfapi.WareID, error) {
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
