package watch

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
	"strconv"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/serum-errors/go-serum"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/warptools/warpforge/pkg/dab"
	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/plotexec"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/pkg/workspace"
	"github.com/warptools/warpforge/wfapi"
)

type Config struct {
	Path       string
	Socket     bool
	PlotConfig wfapi.PlotExecConfig
}

type server struct {
	status int
}

func (s *server) handle(ctx context.Context, conn net.Conn) error {
	// in lieu of doing anything complicated, shoving a status int down the pipe is sufficient
	// for the unix socket implementation we can now use netcat or similar for a status.
	// I.E. nc -U ./sock
	// OR socat - UNIX-CONNECT:./sock
	defer conn.Close()
	enc := json.NewEncoder(conn)
	err := enc.Encode(s.status)
	if err != nil {
		return err
	}
	return nil
}

func (s *server) serveLoop(ctx context.Context, l net.Listener) error {
	log := logging.Ctx(ctx)
	for {
		conn, err := l.Accept() // blocks, doesn't accept a context.
		if err != nil {
			return err
		}
		go func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			defer func() {
				r := recover()
				if r != nil {
					log.Info("", "socket handler panic: %s", r)
				}
			}()
			if err := s.handle(ctx, conn); err != nil {
				log.Info("", "socket handler: %s", err.Error())
			}
		}()
		select {
		case <-ctx.Done():
			log.Info("", "socket no longer accepting connections")
			return nil
		default:
		}
	}
}

//serve will create and listen to a unix socket on the given socket path
func (s *server) listen(ctx context.Context, sockPath string) error {
	log := logging.Ctx(ctx)
	cfg := net.ListenConfig{}
	l, err := cfg.Listen(ctx, "unix", sockPath)
	if err != nil {
		return err
	}
	// unix socks: they have permissions systems.
	os.Chmod(sockPath, 0777) // ignore err?

	result := make(chan error)
	go func() {
		result <- s.serveLoop(ctx, l)
	}()
	<-ctx.Done()
	l.Close()
	log.Info("", "listener closed")
	return <-result
}

func isSocket(m fs.FileMode) bool {
	return m&fs.ModeSocket != 0
}

// rmUnixSocket will perform a racy, lockless "liveness" check on the socket before removing it.
func (s *server) rmUnixSocket(path string) error {
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

func generateSocketPath(path string) (string, error) {
	// This socket path generation is dumb. and also one of the simplest thing to do right now
	sockPath := fmt.Sprintf("/tmp/warpforge-%s", url.PathEscape(path))
	if len(sockPath) > 108 {
		// There is a 108 char limit on socket paths. This is also dumb, but not our fault.
		// Hopefully you don't run into this as a problem
		// See: man unix -> sockaddr_un.sun_path (i.e. [108]char)
		return sockPath, serum.Error("warpforge-error-situation",
			serum.WithMessageTemplate("cannot establish unix socket because of path length: unix socket filenames have a length limit of 108; the computed socket file name for the module at {{path|q}} is {{socketPath|q}}, which is {{socketPathLen}} long."),
			serum.WithDetail("modulePath", path),
			serum.WithDetail("socketPath", sockPath),
			serum.WithDetail("socketPathLen", strconv.Itoa(len(sockPath))),
		)
	}
	return sockPath, nil
}

func (c *Config) Run(ctx context.Context) error {
	log := logging.Ctx(ctx)
	// TODO: currently we read the module/plot from the provided path.
	// instead, we should read it from the git cache dir
	plot, err := dab.PlotFromFile(os.DirFS("/"), filepath.Join(c.Path, dab.MagicFilename_Plot))
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

	srv := server{status: -1}
	if c.Socket {
		absPath, err := filepath.Abs(c.Path)
		if err != nil {
			return err
		}
		sockPath, err := generateSocketPath(absPath)
		if err != nil {
			return err
		}
		if err := srv.rmUnixSocket(sockPath); err != nil {
			log.Info("", "removing socket %q: %s", sockPath, err.Error())
		}
		go func() {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			if err := srv.listen(ctx, sockPath); err != nil {
				log.Info("", "socket server closed: %s", err.Error())
			}
		}()
		log.Info("", "serving to %q\n", sockPath)
		runtime.Gosched()
		time.Sleep(time.Second) // give user a second to realize that there's info here.
		defer runtime.Gosched() // give server a chance to close on context cancel
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
				srv.status = -1
				_, err := execModule(innerCtx, c.PlotConfig, filepath.Join(c.Path, dab.MagicFilename_Module))
				if err != nil {
					fmt.Printf("exec failed: %s\n", err)
					srv.status = 1
				} else {
					srv.status = 0
				}
			}
			innerSpan.End()
		}
		outerSpan.End()
		time.Sleep(time.Millisecond * 100)
	}
}

// DEPRECATED: This should be refactored significantly in the near future to remove reliance on PWD in plot exec.
// ExecModule executes the given module file with the default plot file in the same directory.
// WARNING: This function calls Chdir and may not change back on errors
//
// Errors:
//
//    - warpforge-error-catalog-invalid --
//    - warpforge-error-catalog-parse --
//    - warpforge-error-git --
//    - warpforge-error-io -- when the module or plot files cannot be read or cannot change directory.
//    - warpforge-error-missing-catalog-entry --
//    - warpforge-error-module-invalid -- when the module data is invalid
//    - warpforge-error-plot-execution-failed --
//    - warpforge-error-plot-invalid -- when the plot data is invalid
//    - warpforge-error-plot-step-failed --
//    - warpforge-error-serialization -- when the module or plot cannot be parsed
//    - warpforge-error-unknown -- when changing directories fails
//    - warpforge-error-workspace -- when opening the workspace set fails
func execModule(ctx context.Context, config wfapi.PlotExecConfig, fileName string) (wfapi.PlotResults, error) {
	ctx, span := tracing.Start(ctx, "execModule")
	defer span.End()
	result := wfapi.PlotResults{}

	// parse the module, even though it is not currently used
	if _, werr := dab.ModuleFromFile(os.DirFS("/"), fileName); werr != nil {
		return result, werr
	}

	plot, werr := dab.PlotFromFile(os.DirFS("/"), filepath.Join(filepath.Dir(fileName), dab.MagicFilename_Plot))
	if werr != nil {
		return result, werr
	}

	pwd, nerr := os.Getwd()
	if nerr != nil {
		return result, wfapi.ErrorUnknown("unable to get pwd", nerr)
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", pwd[1:])
	if err != nil {
		return result, wfapi.ErrorWorkspace(pwd, err)
	}

	tmpDir := filepath.Dir(fileName)
	// FIXME: it would be nice if we could avoid changing directories.
	//  This generally means removing Getwd calls from pkg libs
	if nerr := os.Chdir(tmpDir); nerr != nil {
		return result, wfapi.ErrorIo("cannot change directory", tmpDir, nerr)
	}

	result, werr = plotexec.Exec(ctx, wss, wfapi.PlotCapsule{Plot: &plot}, config)

	if nerr := os.Chdir(pwd); nerr != nil {
		return result, wfapi.ErrorIo("cannot return to pwd", pwd, nerr)
	}

	if werr != nil {
		return result, wfapi.ErrorPlotExecutionFailed(werr)
	}

	return result, nil
}
