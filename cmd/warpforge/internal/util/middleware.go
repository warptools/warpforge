package util

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/tracing"
)

// ChainCmdMiddleware returns a cli ActionFunc that is wrapped by the given middleware.
// Middleware is executed in order. E.G. `middleware[0](middleware[1](cmd))`
func ChainCmdMiddleware(cmd cli.ActionFunc, middlewares ...func(cli.ActionFunc) cli.ActionFunc) cli.ActionFunc {
	if len(middlewares) < 1 {
		return cmd
	}
	wrapped := cmd
	// loop in reverse to preserve middleware order
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}

	return wrapped
}

// CmdMiddlewareLogging configures the logging system before executing the CLI command
func CmdMiddlewareLogging(f cli.ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		logger := logging.NewLogger(c.App.Writer, c.App.ErrWriter, c.Bool("json"), c.Bool("quiet"), c.Bool("verbose"))
		c.Context = logger.WithContext(c.Context)
		return f(c)
	}
}

// CmdMiddlewareTracingSpan starts a span with the command name that ends when
// the middleware exits after returning from the command or next middleware
func CmdMiddlewareTracingSpan(f cli.ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		ctx, span := tracing.Start(c.Context, c.Command.FullName())
		defer span.End()
		c.Context = ctx
		err := f(c)
		if err != nil {
			setSpanError(ctx, err)
		}
		return err
	}
}

// CmdMiddlewareTracingConfig configures the tracing system before executing the CLI command
func CmdMiddlewareTracingConfig(f cli.ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		// TODO: Adjust the otel default logging apparatus
		// logger := stdr.New(log.New(os.Stdout, "", log.LstdFlags|log.Lshortfile))
		// otel.SetLogger(logger)
		tracerProvider, err := newTracingProvider(c)
		if err != nil {
			return fmt.Errorf("could not initialize tracing: %w", err)
		}
		if tracerProvider == nil {
			c.Context = tracing.SetTracer(c.Context, nil)
			return f(c)
		}
		ctx := c.Context
		defer func() {
			if tracerProvider == nil {
				return
			}
			if err := tracerProvider.Shutdown(ctx); err != nil {
				logger := logging.Ctx(ctx)
				logger.Debug("", "tracing shutdown error: %s", err.Error())
			}
		}()

		tr := tracerProvider.Tracer(Module)
		ctx = tracing.SetTracer(ctx, tr)
		c.Context = ctx
		return f(c)
	}
}
