package watch

import (
	"context"
	"errors"
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

	"github.com/warptools/warpforge/pkg/config"
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

// Extremely basic status responses for the watch server
const (
	statusRunning = -1
	statusOkay    = 0
	statusFailed  = 1
)

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

// generateSocketPath converts the path of the module to a path where the socket will be created
//
// Errors:
//
//   - warpforge-error-io -- when socket path is too long
//   - warpforge-error-io -- when socket path cannot be canonicalized
func GenerateSocketPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", wfapi.ErrorIo("could not canonicalize path", path, err)
	}
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
		return sockPath, serum.Error("warpforge-error-io",
			serum.WithMessageTemplate("cannot establish unix socket because of path length: unix socket filenames have a length limit of 108; the computed socket file name for the module at {{path|q}} is {{socketPath|q}}, which is {{socketPathLen}} long."),
			serum.WithDetail("modulePath", path),
			serum.WithDetail("socketPath", sockPath),
			serum.WithDetail("socketPathLen", strconv.Itoa(len(sockPath))),
		)
	}
	return sockPath, nil
}

// canonicalize is like filepath.Abs but assumes we already have a working directory path which is absolute
func canonicalizePath(pwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	if !filepath.IsAbs(pwd) {
		panic(fmt.Sprintf("working directory must be an absolute path: %q", pwd))
	}
	return filepath.Join(pwd, path)
}

// Run will execute the watch command
//
// Errors:
//
//    - warpforge-error-datatoonew -- when module or plot has an unrecognized version number
//    - warpforge-error-git --
//    - warpforge-error-io -- when socket path is too long
//    - warpforge-error-io -- when the module or plot files cannot be read or cannot change directory.
//    - warpforge-error-serialization -- when the module or plot cannot be parsed
//    - warpforge-error-unknown -- when changing directories fails
//    - warpforge-error-unknown -- when context ends for reasons other than being canceled
//    - warpforge-error-initialization -- unable to get working directory or executable path
func (c *Config) Run(ctx context.Context) error {
	log := logging.Ctx(ctx)

	wd, xerr := os.Getwd()
	if xerr != nil {
		return serum.Error(wfapi.ECodeInitialization, serum.WithCause(xerr), serum.WithMessageLiteral("unable to get working directory"))
	}

	modulePath := filepath.Join(c.Path, dab.MagicFilename_Module)
	modulePathAbs := canonicalizePath(wd, modulePath)

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
		absPath := canonicalizePath(wd, c.Path)
		sockPath, err := GenerateSocketPath(absPath)
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
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			return serum.Errorf(wfapi.ECodeUnknown, "context error: %w", ctx.Err())
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
			// this is where things are kind of weird. We already initialized a lot of stuff but the new clone could have
			// different plot/ingests/workspace stack etc. Currently we handle as few of these potential inconsistencies as possible.
			tracing.EndWithStatus(gitSpan, err)
			if err != nil {
				return serum.Error(wfapi.ECodeGit,
					serum.WithMessageTemplate("failed to checkout git repository {{ repository | q }} to memory"),
					serum.WithDetail("repository", path),
					serum.WithCause(err),
				)
			}

			hashBytes, err := r.ResolveRevision(plumbing.Revision(rev))
			if err != nil {
				return serum.Error(wfapi.ECodeGit,
					serum.WithMessageTemplate("failed to resolve git hash for revision {{ revision | q }}"),
					serum.WithDetail("revision", rev),
					serum.WithCause(err),
				)
			}
			hash := hashBytes.String()

			if ingestCache[path] != hash {
				innerSpan.AddEvent("ingest updated", trace.WithAttributes(attribute.String(tracing.AttrKeyWarpforgeIngestHash, hash)))
				log.Info("", "path %q changed; new hash %q", path, hash)
				ingestCache[path] = hash
				srv.status = statusRunning

				_, err := exec(innerCtx, c.PlotConfig, modulePathAbs)
				if err != nil {
					log.Info("", "exec failed: %s", err)
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

// exec executes default plot file in the same directory.
//
// Errors:
//
//    - warpforge-error-catalog-invalid --
//    - warpforge-error-catalog-parse --
//    - warpforge-error-git --
//    - warpforge-error-io -- when the module or plot files cannot be read or cannot change directory.
//    - warpforge-error-catalog-missing-entry --
//    - warpforge-error-plot-execution-failed --
//    - warpforge-error-plot-invalid -- when the plot data is invalid
//    - warpforge-error-plot-step-failed --
//    - warpforge-error-serialization -- when the module or plot cannot be parsed
//    - warpforge-error-module-invalid -- when module name is invalid
//    - warpforge-error-datatoonew -- module or plot contains newer-than-recognized versions
//    - warpforge-error-searching-filesystem -- when an unexpected error occurs traversing the search path
//    - warpforge-error-initialization -- when working directory or binary path cannot be found
//    - warpforge-error-workspace-missing -- when opening the workspace set fails
func exec(ctx context.Context, pltCfg wfapi.PlotExecConfig, modulePathAbs string) (wfapi.PlotResults, error) {
	ctx, span := tracing.Start(ctx, "execModule")
	defer span.End()
	result := wfapi.PlotResults{}

	// parse the module, even though it is not currently used
	if _, err := dab.ModuleFromFile(os.DirFS("/"), modulePathAbs[1:]); err != nil {
		return result, err
	}

	moduleDirAbs := filepath.Dir(modulePathAbs)
	plotPath := filepath.Join(moduleDirAbs, dab.MagicFilename_Plot)
	plot, err := dab.PlotFromFile(os.DirFS("/"), plotPath[1:])
	if err != nil {
		return result, err
	}

	wss, err := workspace.FindWorkspaceStack(os.DirFS("/"), "", modulePathAbs)
	if err != nil {
		return result, err
	}
	exCfg, err := config.PlotExecConfig(&modulePathAbs)
	if err != nil {
		return result, err
	}
	exCfg.WorkingDirectory = moduleDirAbs
	result, err = plotexec.Exec(ctx, exCfg, wss, wfapi.PlotCapsule{Plot: &plot}, pltCfg)

	if err != nil {
		return result, wfapi.ErrorPlotExecutionFailed(err)
	}

	return result, nil
}
