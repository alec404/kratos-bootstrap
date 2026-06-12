package rpc

import (
	"context"
	"crypto/rand"
	"strings"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/middleware"
	kratosTracing "github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var tracingSuppressPropagator = propagation.NewCompositeTextMapPropagator(
	kratosTracing.Metadata{},
	propagation.Baggage{},
	propagation.TraceContext{},
)

func tracingServer(cfg *conf.Middleware_Tracing) middleware.Middleware {
	return tracingWithOperationSkipper(kratosTracing.Server(), cfg, serverTraceOperation, nil)
}

func tracingClient(cfg *conf.Middleware_Tracing) middleware.Middleware {
	return tracingWithOperationSkipper(kratosTracing.Client(), cfg, clientTraceOperation, injectSuppressedTraceContext)
}

func tracingWithOperationSkipper(base middleware.Middleware, cfg *conf.Middleware_Tracing, operationFn func(context.Context) string, onSkip func(context.Context) context.Context) middleware.Middleware {
	if cfg == nil || (len(cfg.GetExcludeOperations()) == 0 && len(cfg.GetExcludeOperationPrefixes()) == 0) {
		return base
	}

	return func(handler middleware.Handler) middleware.Handler {
		tracedHandler := base(handler)
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if shouldSkipTracingByOperation(ctx, cfg, operationFn) {
				ctx = contextWithTracingSuppressed(ctx)
				if onSkip != nil {
					ctx = onSkip(ctx)
				}
				return handler(ctx, req)
			}
			return tracedHandler(ctx, req)
		}
	}
}

func shouldSkipTracingByOperation(ctx context.Context, cfg *conf.Middleware_Tracing, operationFn func(context.Context) string) bool {
	if cfg == nil {
		return false
	}

	operation := operationFn(ctx)
	return traceOperationMatches(operation, cfg.GetExcludeOperations(), cfg.GetExcludeOperationPrefixes())
}

func serverTraceOperation(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}

func clientTraceOperation(ctx context.Context) string {
	if tr, ok := transport.FromClientContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}

func traceOperationMatches(operation string, exactOperations, operationPrefixes []string) bool {
	operation = strings.TrimSpace(operation)
	if operation == "" {
		return false
	}

	for _, exactOperation := range exactOperations {
		if operation == strings.TrimSpace(exactOperation) {
			return true
		}
	}

	for _, prefix := range operationPrefixes {
		prefix = strings.TrimSpace(prefix)
		if prefix != "" && strings.HasPrefix(operation, prefix) {
			return true
		}
	}

	return false
}

func contextWithTracingSuppressed(ctx context.Context) context.Context {
	// A valid but unsampled remote SpanContext makes ParentBased samplers drop
	// descendant spans. Using an invalid/noop context would let child
	// instrumentation start new root spans and still export traces.
	traceID := newTraceID()
	spanID := newSpanID()
	spanCtx := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	return oteltrace.ContextWithRemoteSpanContext(ctx, spanCtx)
}

func injectSuppressedTraceContext(ctx context.Context) context.Context {
	if tr, ok := transport.FromClientContext(ctx); ok {
		tracingSuppressPropagator.Inject(ctx, tr.RequestHeader())
	}
	return ctx
}

func newTraceID() oteltrace.TraceID {
	var traceID oteltrace.TraceID
	if _, err := rand.Read(traceID[:]); err != nil || !traceID.IsValid() {
		traceID[len(traceID)-1] = 1
	}
	return traceID
}

func newSpanID() oteltrace.SpanID {
	var spanID oteltrace.SpanID
	if _, err := rand.Read(spanID[:]); err != nil || !spanID.IsValid() {
		spanID[len(spanID)-1] = 1
	}
	return spanID
}
