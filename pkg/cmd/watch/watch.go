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

// Configuration for the watch command
type Config struct {
	// Path is the path to the directory containing the module you want to watch
	Path string
	// Socket will enable a unix socket that emits watch result status
	Socket bool
	// PlotConfig customizes the plot execution
	PlotConfig wfapi.PlotExecConfig
}

// server stores the current status of the plot execution and responds to clients
type server struct {
	status   int
	listener net.Listener
}

// Extremely basic status responses for the watch server
const (
	statusRunning = -1
	statusOkay    = 0
	statusFailed  = 1
)

// handle is expected to respond to client connections.
// This function should recover from panics and log errors before returning.
// It is expected that handle is run as a goroutine and that errors may not be handled.
//
// handle emits the current status of the watch command over the connection
func (s *server) handle(ctx context.Context, conn net.Conn) error {
	log := logging.Ctx(ctx)
	defer func() {
		r := recover()
		if r != nil {
			log.Info("", "socket handler panic: %s", r)
			return
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// in lieu of doing anything complicated, shoving a status int down the pipe is sufficient
	// for the unix socket implementation we can now use netcat or similar for a status.
	// I.E. nc -U ./sock
	// OR socat - UNIX-CONNECT:./sock
	defer conn.Close()
	enc := json.NewEncoder(conn)
	err := enc.Encode(s.status)
	if err != nil {
		log.Info("", "socket handler: %s", err.Error())
		return err
	}
	return nil
}

// serve will call Accept and block until the listener is closed.
// context will only be checked between accepted connections
// serve should not return under normal circumstances, however if an error occurs during accept then it will log that error and return it
// will log and return immediately if listen was not called
// It is expected that serve will be called as a goroutine and the returned error may not be handled.
// Any errors should be logged before returning.
func (s *server) serve(ctx context.Context) error {
	log := logging.Ctx(ctx)
	if s.listener == nil {
		err := fmt.Errorf("did not call listen on server")
		log.Info("", err.Error())
		return err
	}
	for {
		conn, err := s.listener.Accept() // blocks, doesn't accept a context.
		if err != nil {
			log.Info("", "socket error on accept: %s", err.Error())
			return err
		}
		go s.handle(ctx, conn)
		select {
		case <-ctx.Done():
			log.Info("", "socket no longer accepting connections")
			return nil
		default:
		}
	}
}

// listen will create a unix socket on the given path
// listen should be called before "serve"
func (s *server) listen(ctx context.Context, sockPath string) (err error) {
	cfg := net.ListenConfig{}
	l, err := cfg.Listen(ctx, "unix", sockPath)
	if err != nil {
		return err
	}
	s.listener = l
	return nil
}

func isSocket(m fs.FileMode) bool {
	return m&fs.ModeSocket != 0
}

// rmUnixSocket will perform a racy, lockless "liveness" check on the socket before removing it.
// If socket does not exist then rmUnixSocket will return nil.
// If a non-socket file exists at the given path, an error will be returned.
// If socket exists and a listener responds then we do nothing and the return nil on the assumption that something in the future will return a bind error.
// If the socket exists and a listener does not respond to a dial then the file will be removed.
//
// NOTE: If for some reason the socket exists and the listener is alive but does not respond, the file will still be removed.
// This will likely result in errors for whoever is expecting that socket file to exist, however the listener holds an open file descriptor and will not likely detect any problems.
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
		// There is a 108 char limit on socket paths.
		// This is problematic, but originates in the linux kernel, so options for addressing it are limited.
		// Currently we do not take any special action to avoid this problem.
		// Future work could include either name mangles (e.g. hashing or truncating paths), introducing additional
		// config for customizing socket path in workspaces, or moving directories
		// to affect how the path appears to the dial call relative to the execution context.
		//
		// We would like to put sockets _in_ workspace directories but placing them in /tmp for now seems
		// like a cheap, low effort solution to the problem at the moment.
		// Hopefully you don't run into this as a problem
		//
		// See `man unix`:
		//
		// A UNIX domain socket address is represented in the following structure:
		//   struct sockaddr_un {
		//       sa_family_t sun_family;               /* AF_UNIX */
		//       char        sun_path[108];            /* Pathname */
		//   };
		// The sun_family field always contains AF_UNIX.  On Linux, sun_path is 108 bytes in size; ...
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
	plot, err := dab.PlotFromFile(os.DirFS(c.Path), dab.MagicFilename_Plot)
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

	srv := server{status: statusRunning}
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
		defer runtime.Gosched() // give server a chance to close on context cancel/close
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		if err := srv.listen(ctx, sockPath); err != nil {
			return err
		}
		defer srv.listener.Close()
		go srv.serve(ctx)
		log.Info("", "serving to %q\n", sockPath)
		time.Sleep(time.Second) // give user a second to realize that there's info here. FIXME: Consider literally anything other than a hardcoded sleep.
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
				srv.status = statusRunning

				modulePath := filepath.Join(c.Path, dab.MagicFilename_Module)
				_, err := execModule(innerCtx, c.PlotConfig, modulePath)
				if err != nil {
					fmt.Printf("exec failed: %s\n", err)
					srv.status = statusFailed
				} else {
					srv.status = statusOkay
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
func execModule(ctx context.Context, config wfapi.PlotExecConfig, modulePath string) (wfapi.PlotResults, error) {
	ctx, span := tracing.Start(ctx, "execModule")
	defer span.End()
	result := wfapi.PlotResults{}

	pwd, nerr := os.Getwd()
	if nerr != nil {
		return result, wfapi.ErrorUnknown("unable to get pwd", nerr)
	}

	modulePathAbs, err := filepath.Abs(modulePath)
	if err != nil {
		return result, wfapi.ErrorIo("unable to get absolute path", modulePathAbs, err)
	}
	// parse the module, even though it is not currently used
	if _, werr := dab.ModuleFromFile(os.DirFS("/"), modulePathAbs[1:]); werr != nil {
		return result, werr
	}

	plot, werr := dab.PlotFromFile(os.DirFS("/"), filepath.Join(filepath.Dir(modulePathAbs), dab.MagicFilename_Plot)[1:])
	if werr != nil {
		return result, werr
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", pwd[1:])
	if err != nil {
		return result, wfapi.ErrorWorkspace(pwd, err)
	}

	tmpDir := filepath.Dir(modulePathAbs)
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
