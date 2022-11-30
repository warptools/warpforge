package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/warptools/warpforge/cmd/warpforge/internal/util"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/wfapi"
)

var watchCmdDef = cli.Command{
	Name:  "watch",
	Usage: "Watch a directory for git commits, executing plot on each new commit",
	Action: util.ChainCmdMiddleware(cmdWatch,
		util.CmdMiddlewareLogging,
		util.CmdMiddlewareTracingConfig,
	),
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "socket",
			Usage: "Experimental flag for getting execution status of the watched plot externally via unix socket",
		},
	},
}

type watchCfg struct {
	path         string
	recursive    bool
	socket       bool
	plotCfg      wfapi.PlotExecConfig
	recentStatus int
}

func (c *watchCfg) handle(ctx context.Context, conn net.Conn) error {
	// in lieu of doing anything complicated, shoving a status int down the pipe is sufficient
	// for the unix socket implementation we can now use netcat or similar for a status.
	// I.E. nc -U ./sock
	// OR socat - UNIX-CONNECT:./sock
	defer conn.Close()
	enc := json.NewEncoder(conn)
	err := enc.Encode(c.recentStatus)
	if err != nil {
		return err
	}
	return nil
}

//serve will create and listen to a unix socket on the given socket path
func (c *watchCfg) serve(ctx context.Context, sockPath string) error {
	log := logging.Ctx(ctx)
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}
	// unix socks: they have permissions systems.
	os.Chmod(sockPath, 0777) // ignore err?

	ul := l.(*net.UnixListener)
	ul.SetUnlinkOnClose(true) // this mostly doesn't do anything, we need to handle SIGINT to remove these gracefully
	defer l.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer func() {
				r := recover()
				if r != nil {
					log.Info("", "socket handler panic: %s", r)
				}
			}()
			if err := c.handle(ctx, conn); err != nil {
				log.Info("", "socket handler: %s", err.Error())
			}
		}()
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
}

func isSocket(m fs.FileMode) bool {
	return m&fs.ModeSocket != 0
}

// rmUnixSocket will perform a racy, lockless "liveness" check on the socket before removing it.
func (c *watchCfg) rmUnixSocket(path string) error {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if fi == nil {
		return fmt.Errorf("file info could not be read: %w", err)
	}
	if !isSocket(fi.Mode()) {
		return fmt.Errorf("file at path is not a socket")
	}
	// if it exists: dial it; if we can't, rm it.
	// race?  yep.  if anyone knows how to do this right, uh, plz call.

	conn, err := net.Dial("unix", path)
	if err != nil {
		os.Remove(path)
		return nil
	}
	conn.Close()
	return nil
}

func generateSocketPath(path string) string {
	// This socket path generation is dumb. and also one of the simplest thing to do right now
	sockPath := fmt.Sprintf("/tmp/warpforge-%s", url.PathEscape(path))
	if len(sockPath) > 108 {
		// There is a 108 char limit on socket paths. This is also dumb, but not our fault.
		// Hopefully you don't run into this as a problem
		sockPath = sockPath[:108]
	}
	return sockPath
}

func (c *watchCfg) run(ctx context.Context) error {
	log := logging.Ctx(ctx)
	// TODO: currently we read the module/plot from the provided path.
	// instead, we should read it from the git cache dir
	plot, err := util.PlotFromFile(filepath.Join(c.path, util.PlotFilename))
	if err != nil {
		return err
	}

	ingests := make(map[string]string)
	var allInputs []wfapi.PlotInput
	for _, input := range plot.Inputs.Values {
		allInputs = append(allInputs, input)
	}
	for _, step := range plot.Steps.Values {
		for _, input := range step.Protoformula.Inputs.Values {
			allInputs = append(allInputs, input)
		}
	}

	for _, input := range allInputs {
		if input.Basis().Ingest != nil && input.Basis().Ingest.GitIngest != nil {
			ingest := input.Basis().Ingest.GitIngest
			ingests[ingest.HostPath] = ingest.Ref
		}
	}

	ingestCache := make(map[string]string)
	for k, v := range ingests {
		ingestCache[k] = v
	}

	c.recentStatus = -1
	if c.socket {
		absPath, err := filepath.Abs(c.path)
		if err != nil {
			return err
		}
		sockPath := generateSocketPath(absPath)
		if err := c.rmUnixSocket(sockPath); err != nil {
			log.Info("", "removing socket %q: %s", sockPath, err.Error())
		}
		serveCtx, cancel := context.WithCancel(ctx)
		ctx = serveCtx
		go func() {
			defer cancel()
			if err := c.serve(serveCtx, sockPath); err != nil {
				log.Info("", "socket server closed: %s", err.Error())
			}
		}()
		log.Info("", "serving to %q\n", sockPath)
		runtime.Gosched()
		time.Sleep(time.Second) // give user a second to realize that there's info here.
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		outerCtx, outerSpan := tracing.Start(ctx, "watch-loop")
		for path, rev := range ingests {
			innerCtx, innerSpan := tracing.Start(outerCtx, "watch-loop-ingest",
				trace.WithAttributes(
					attribute.String(tracing.AttrKeyWarpforgeIngestPath, path),
					attribute.String(tracing.AttrKeyWarpforgeIngestRev, rev),
				),
			)
			gitCtx, gitSpan := tracing.Start(innerCtx, "copy local repo", trace.WithAttributes(tracing.AttrFullExecNameGit, tracing.AttrFullExecOperationGitClone))
			defer gitSpan.End()
			r, err := git.CloneContext(gitCtx, memory.NewStorage(), nil, &git.CloneOptions{
				URL: "file://" + path,
			})
			tracing.EndWithStatus(gitSpan, err)
			if err != nil {
				return fmt.Errorf("failed to checkout git repository at %q to memory: %s", path, err)
			}

			hashBytes, err := r.ResolveRevision(plumbing.Revision(rev))
			if err != nil {
				return fmt.Errorf("failed to resolve git hash: %s", err)
			}
			hash := hashBytes.String()

			if ingestCache[path] != hash {
				innerSpan.AddEvent("ingest updated", trace.WithAttributes(attribute.String(tracing.AttrKeyWarpforgeIngestHash, hash)))
				fmt.Println("path", path, "changed, new hash", hash)
				ingestCache[path] = hash
				c.recentStatus = -1
				_, err := util.ExecModule(innerCtx, c.plotCfg, filepath.Join(c.path, util.ModuleFilename))
				if err != nil {
					fmt.Printf("exec failed: %s\n", err)
					c.recentStatus = 1
				} else {
					c.recentStatus = 0
				}
			}
			innerSpan.End()
		}
		outerSpan.End()
		time.Sleep(time.Millisecond * 100)
	}
}

func cmdWatch(c *cli.Context) error {
	if c.Args().Len() != 1 {
		return fmt.Errorf("invalid args")
	}
	cfg := &watchCfg{
		path:   c.Args().First(),
		socket: c.Bool("socket"),
		plotCfg: wfapi.PlotExecConfig{
			Recursive: c.Bool("recursive"),
			FormulaExecConfig: wfapi.FormulaExecConfig{
				DisableMemoization: c.Bool("force"),
			},
		},
	}
	return cfg.run(c.Context)
}
