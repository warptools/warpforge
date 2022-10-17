package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/warpfork/warpforge/pkg/logging"
	"github.com/warpfork/warpforge/pkg/tracing"
	"github.com/warpfork/warpforge/wfapi"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

// cmdMiddlewareTracing configures the logging system before executing the CLI command
func cmdMiddlewareLogging(f cli.ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		logger := logging.NewLogger(c.App.Writer, c.App.ErrWriter, c.Bool("json"), c.Bool("quiet"), c.Bool("verbose"))
		c.Context = logger.WithContext(c.Context)
		return f(c)
	}
}

func setSpanError(ctx context.Context, err error) {
	wfErr, ok := err.(wfapi.Error)
	if !ok {
		wfErr = wfapi.ErrorUnknown("command failed", err)
	}
	tracing.SetSpanError(ctx, wfErr)
}

// cmdMiddlewareTracingSpan starts a span with the command name that ends when
// the middleware exits after returning from the command or next middleware
func cmdMiddlewareTracingSpan(f cli.ActionFunc) cli.ActionFunc {
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

// cmdMiddlewareTracingConfig configures the tracing system before executing the CLI command
func cmdMiddlewareTracingConfig(f cli.ActionFunc) cli.ActionFunc {
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

		tr := tracerProvider.Tracer(MODULE)
		ctx = tracing.SetTracer(ctx, tr)
		c.Context = ctx
		return f(c)
	}
}

// mergeResources takes all the open telemetry resources and merges them in order.
// If resources is empty then an an empty resource is returned
func mergeResources(resources ...*resource.Resource) (*resource.Resource, error) {
	if len(resources) == 0 {
		return resource.Empty(), nil
	}
	var err error
	result := resources[0]
	for _, r := range resources[1:] {
		result, err = resource.Merge(result, r)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// newResource is generally where we add our identifying keys for the process
func newResource() (*resource.Resource, error) {
	defaultResource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(MODULE),
		semconv.ServiceVersionKey.String(VERSION),
	)
	return mergeResources(
		resource.Default(),
		defaultResource,
		resource.Environment(),
	)
}

// newTracingProvider creates a tracer provider from CLI flags
func newTracingProvider(c *cli.Context) (_ *sdktrace.TracerProvider, retErr error) {
	logger := logging.Ctx(c.Context)
	res, err := newResource()
	if err != nil {
		return nil, err
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}
	exporters := []sdktrace.TracerProviderOption{}
	fileExporter, err := newFileSpanExporter(c.Context, c.String("trace.file"))
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			fileExporter.Shutdown(c.Context)
		}
	}()
	if fileExporter != nil {
		exporters = append(exporters, sdktrace.WithBatcher(fileExporter))
	}

	if c.Bool("trace.http.enable") {
		logger.Debug("", "trace.http.enable: %t", c.Bool("trace.http.enable"))
		httpOpts := []otlptracehttp.Option{}
		if c.Bool("trace.http.insecure") {
			logger.Debug("", "trace.http.enable: %t", c.Bool("trace.http.insecure"))
			httpOpts = append(httpOpts, otlptracehttp.WithInsecure())
		}
		client := otlptracehttp.NewClient(httpOpts...)
		httpExporter, err := otlptrace.New(c.Context, client)
		if err != nil {
			return nil, err
		}
		exporters = append(exporters, sdktrace.WithBatcher(httpExporter))
	}
	if len(exporters) == 0 {
		return nil, nil
	}
	opts = append(opts, exporters...)
	tracerProvider := sdktrace.NewTracerProvider(opts...)
	return tracerProvider, nil
}

// fileSpanExporter calls Close() during Shutdown, simplifying the
// implementation for file handling
type fileSpanExporter struct {
	sdktrace.SpanExporter
	io.Closer
}

// Shutdown handles cleaning up the span exporter
//
// Errors:
//
//     - warpforge-error-internal -- when an error occurs during tracing shutdown
func (e *fileSpanExporter) Shutdown(ctx context.Context) error {
	if e == nil {
		return nil
	}
	defer e.Closer.Close() // consume file close errors
	if err := e.SpanExporter.Shutdown(ctx); err != nil {
		return wfapi.ErrorInternal("tracing shutdown failed", err)
	}
	return nil
}

// newFileSpanExporter creates or truncates the named file and uses the file with a console exporter.
func newFileSpanExporter(ctx context.Context, name string) (*fileSpanExporter, error) {
	logger := logging.Ctx(ctx)
	if name == "" {
		return nil, nil
	}
	logger.Debug("", "trace file path: %s", name)
	f, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	exp, err := stdouttrace.New(
		stdouttrace.WithWriter(f),
		// Use human-readable output.
		stdouttrace.WithPrettyPrint(),
		// Do not print timestamps for the demo.
		stdouttrace.WithoutTimestamps(),
	)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &fileSpanExporter{exp, f}, err
}
