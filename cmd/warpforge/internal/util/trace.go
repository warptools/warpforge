package util

import (
	"context"
	"io"
	"os"

	"github.com/urfave/cli/v2"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	"github.com/warptools/warpforge/pkg/logging"
	"github.com/warptools/warpforge/pkg/tracing"
	"github.com/warptools/warpforge/wfapi"
)

// The module name used for unique strings, such as tracing identifiers
// Grab it via `go list -m` or manually. It's not available at runtime and
// it's too trivial to generate. Might inject with LDFLAGS later.
const Module = "github.com/warptools/warpforge"

func setSpanError(ctx context.Context, err error) {
	wfErr, ok := err.(wfapi.Error)
	if !ok {
		wfErr = wfapi.ErrorUnknown("command failed", err)
	}
	tracing.SetSpanError(ctx, wfErr)
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
func newResource(version string, module string) (*resource.Resource, error) {
	defaultResource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(module),
		semconv.ServiceVersionKey.String(version),
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
	res, err := newResource(c.App.Version, Module)
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
