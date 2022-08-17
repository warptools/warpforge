package main

import (
	"context"
	"io"
	"os"

	"github.com/warpfork/warpforge/wfapi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

const TRACER_NAME = "main"

// configTracer sets the default tracing configuration
// The caller must call Shutdown on the provider
func configTracer(filename string) (*sdktrace.TracerProvider, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(MODULE),
			semconv.ServiceVersionKey.String(VERSION),
		),
	)
	if err != nil {
		return nil, err
	}
	exp, err := newFileSpanExporter(filename)
	if err != nil {
		return nil, err
	}
	if exp == nil {
		return nil, nil
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

func traceShutdown(ctx context.Context, tp *sdktrace.TracerProvider) error {
	if tp == nil {
		return nil
	}
	return tp.Shutdown(ctx)
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
func newFileSpanExporter(name string) (*fileSpanExporter, error) {
	if name == "" {
		return nil, nil
	}
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
