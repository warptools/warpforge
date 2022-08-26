package tracing

import (
	"context"

	"github.com/warpfork/warpforge/wfapi"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ctxKey struct{}

// TracerFromCtx returns the tracer set for the current context.
// If no tracer is currently set in ctx, a new no-op tracer will be returned.
func TracerFromCtx(ctx context.Context) trace.Tracer {
	tracer, ok := ctx.Value(ctxKey{}).(trace.Tracer)
	// tracer should not be nil here because SetTracer should check for that.
	// Do not allow a nil tracer to be inserted into context.
	if !ok {
		// I could use a global here to reduce mallocs. Unsure if that's preferable.
		// Ideally I'd be able to declare a variable as a noopTracer struct directly
		// but I don't want to maintain it and the upstream implementation is private.
		// Generally the user should shove a noop tracer into the context to get the same effect.
		// It's an empty struct anyway so the compiler might optimize it out.
		return trace.NewNoopTracerProvider().Tracer("")
	}
	return tracer
}

// SetTracer returns a new context with the given tracer associated with it.
// Setting the tracer to nil will create a noop tracer and insert it into the context.
func SetTracer(ctx context.Context, tracer trace.Tracer) context.Context {
	if tracer == nil {
		tracer = trace.NewNoopTracerProvider().Tracer("")
	}
	if existing, ok := ctx.Value(ctxKey{}).(trace.Tracer); ok {
		if existing == tracer {
			// Do not store same object twice.
			return ctx
		}
	}
	return context.WithValue(ctx, ctxKey{}, tracer)
}

// Start is a shortcut for retrieving the context tracer and calling Start.
// Start creates a span and a context.Context containing the newly-created span.
//
// If the current context does not contain a tracer then a new no-op tracer will be created for the new context.
// See go.opentelemetry.io/otel/trace.Tracer.Start for more information on the Start function.
func Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return TracerFromCtx(ctx).Start(ctx, spanName, opts...)
}

const SpanAttrWarpforgeErrorCode = "warpforge.error.code"
const SpanAttrWarpforgeFormulaId = "warpforge.formula.id"
const SpanAttrWarpforgePackId = "warpforge.pack.id"

// SetSpanError is a helper function to set the span error based on a wfapi.Error
func SetSpanError(ctx context.Context, err wfapi.Error) {
	e := err.(*wfapi.ErrorVal)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String(SpanAttrWarpforgeErrorCode, e.Code()),
	)
	span.SetStatus(codes.Error, e.Error())
}
