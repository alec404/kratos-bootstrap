package rpc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestCreateHTTPServerUsesDefaultErrorEncoder(t *testing.T) {
	srv := CreateHTTPServer(testBootstrapConfig(), log.DefaultLogger)
	srv.Route("/").GET("/boom", func(_ kratosHttp.Context) error {
		return errors.New("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestCreateHTTPServerWithOptionsInjectsErrorEncoder(t *testing.T) {
	srv := CreateHTTPServerWithOptions(
		testBootstrapConfig(),
		log.DefaultLogger,
		[]kratosHttp.ServerOption{
			kratosHttp.ErrorEncoder(func(w http.ResponseWriter, _ *http.Request, _ error) {
				w.WriteHeader(http.StatusAccepted)
			}),
		},
	)
	srv.Route("/").GET("/boom", func(_ kratosHttp.Context) error {
		return errors.New("boom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected status code: got %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestTracingServerSkipsExcludedOperation(t *testing.T) {
	recorder := installTestTracerProvider(t)
	cfg := &conf.Middleware_Tracing{
		ExcludeOperations: []string{"/skip-trace"},
	}

	handler := tracingServer(cfg)(func(context.Context, interface{}) (interface{}, error) {
		return "ok", nil
	})

	_, err := handler(newTestServerContext(transport.KindHTTP, "/skip-trace"), nil)
	if err != nil {
		t.Fatalf("unexpected skipped operation error: %v", err)
	}
	if got := len(recorder.Ended()); got != 0 {
		t.Fatalf("unexpected ended span count for skipped operation: got %d, want 0", got)
	}

	_, err = handler(newTestServerContext(transport.KindHTTP, "/with-trace"), nil)
	if err != nil {
		t.Fatalf("unexpected traced operation error: %v", err)
	}
	if got := len(recorder.Ended()); got != 1 {
		t.Fatalf("unexpected ended span count for traced operation: got %d, want 1", got)
	}
}

func TestTracingServerSkipsExcludedGRPCOperationPrefix(t *testing.T) {
	recorder := installTestTracerProvider(t)
	cfg := &conf.Middleware_Tracing{
		ExcludeOperationPrefixes: []string{"/helloworld.Greeter/"},
	}

	handler := tracingServer(cfg)(func(context.Context, interface{}) (interface{}, error) {
		return "ok", nil
	})

	_, err := handler(newTestServerContext(transport.KindGRPC, "/helloworld.Greeter/SayHello"), nil)
	if err != nil {
		t.Fatalf("unexpected skipped grpc operation error: %v", err)
	}
	if got := len(recorder.Ended()); got != 0 {
		t.Fatalf("unexpected ended span count for skipped grpc operation: got %d, want 0", got)
	}
}

func TestTracingServerSuppressesChildSpansForExcludedOperation(t *testing.T) {
	recorder := installTestTracerProvider(t)
	cfg := &conf.Middleware_Tracing{
		ExcludeOperations: []string{"/skip-with-child"},
	}

	handler := tracingServer(cfg)(func(ctx context.Context, _ interface{}) (interface{}, error) {
		_, span := otel.Tracer("test").Start(ctx, "child-span")
		span.End()

		clientCtx := transport.NewClientContext(ctx, &testTransport{
			kind:          transport.KindGRPC,
			operation:     "/core.service.v1.NewsService/ListNews",
			requestHeader: http.Header{},
			replyHeader:   http.Header{},
		})
		clientHandler := tracingClient(nil)(func(context.Context, interface{}) (interface{}, error) {
			return "client-ok", nil
		})
		if _, err := clientHandler(clientCtx, nil); err != nil {
			return nil, err
		}
		return "ok", nil
	})

	_, err := handler(newTestServerContext(transport.KindHTTP, "/skip-with-child"), nil)
	if err != nil {
		t.Fatalf("unexpected skipped operation error: %v", err)
	}
	if got := len(recorder.Ended()); got != 0 {
		t.Fatalf("unexpected ended span count for skipped operation and child span: got %d, want 0", got)
	}
}

func TestTracingClientMatchesClientOperationWhenServerContextExists(t *testing.T) {
	recorder := installTestTracerProvider(t)
	cfg := &conf.Middleware_Tracing{
		ExcludeOperations: []string{"/core.service.v1.NewsService/ListNews"},
	}

	handler := tracingClient(cfg)(func(context.Context, interface{}) (interface{}, error) {
		return "ok", nil
	})

	serverCtx := newTestServerContext(transport.KindHTTP, "/interface.service.v1.NewsAPIService/ListNewsByCategory")
	clientCtx := transport.NewClientContext(serverCtx, &testTransport{
		kind:          transport.KindGRPC,
		operation:     "/core.service.v1.NewsService/ListNews",
		requestHeader: http.Header{},
		replyHeader:   http.Header{},
	})
	_, err := handler(clientCtx, nil)
	if err != nil {
		t.Fatalf("unexpected skipped client operation error: %v", err)
	}
	if got := len(recorder.Ended()); got != 0 {
		t.Fatalf("unexpected ended span count for skipped client operation: got %d, want 0", got)
	}
}

func TestTracingClientPropagatesUnsampledContextForExcludedOperation(t *testing.T) {
	recorder := installTestTracerProvider(t)
	cfg := &conf.Middleware_Tracing{
		ExcludeOperations: []string{"/core.service.v1.NewsService/ListNews"},
	}

	clientTransport := newTestTransport(transport.KindGRPC, "/core.service.v1.NewsService/ListNews")
	clientCtx := transport.NewClientContext(context.Background(), clientTransport)
	clientHandler := tracingClient(cfg)(func(context.Context, interface{}) (interface{}, error) {
		return "ok", nil
	})
	_, err := clientHandler(clientCtx, nil)
	if err != nil {
		t.Fatalf("unexpected skipped client operation error: %v", err)
	}

	traceparent := clientTransport.requestHeader.Get("traceparent")
	if traceparent == "" {
		t.Fatal("traceparent header is empty")
	}
	if !strings.HasSuffix(traceparent, "-00") {
		t.Fatalf("traceparent should be unsampled: got %q", traceparent)
	}

	downstreamTransport := newTestTransport(transport.KindGRPC, "/core.service.v1.NewsService/ListNews")
	downstreamTransport.requestHeader = clientTransport.requestHeader
	downstreamCtx := transport.NewServerContext(context.Background(), downstreamTransport)
	downstreamHandler := tracingServer(nil)(func(context.Context, interface{}) (interface{}, error) {
		return "downstream-ok", nil
	})
	_, err = downstreamHandler(downstreamCtx, nil)
	if err != nil {
		t.Fatalf("unexpected downstream operation error: %v", err)
	}
	if got := len(recorder.Ended()); got != 0 {
		t.Fatalf("unexpected ended span count for skipped client and downstream operation: got %d, want 0", got)
	}
}

func testBootstrapConfig() *conf.Bootstrap {
	return &conf.Bootstrap{
		Server: &conf.Server{
			Http: &conf.Server_HTTP{
				Middleware: &conf.Middleware{},
			},
		},
	}
}

func installTestTracerProvider(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithSpanProcessor(recorder),
	)
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)

	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
		otel.SetTracerProvider(previous)
	})

	return recorder
}

func newTestServerContext(kind transport.Kind, operation string) context.Context {
	return transport.NewServerContext(context.Background(), newTestTransport(kind, operation))
}

type testTransport struct {
	kind          transport.Kind
	operation     string
	requestHeader http.Header
	replyHeader   http.Header
}

func (t *testTransport) Kind() transport.Kind            { return t.kind }
func (t *testTransport) Endpoint() string                { return "" }
func (t *testTransport) Operation() string               { return t.operation }
func (t *testTransport) RequestHeader() transport.Header { return transportHeader(t.requestHeader) }
func (t *testTransport) ReplyHeader() transport.Header   { return transportHeader(t.replyHeader) }

func newTestTransport(kind transport.Kind, operation string) *testTransport {
	return &testTransport{
		kind:          kind,
		operation:     operation,
		requestHeader: http.Header{},
		replyHeader:   http.Header{},
	}
}

type transportHeader http.Header

func (h transportHeader) Get(key string) string      { return http.Header(h).Get(key) }
func (h transportHeader) Set(key, value string)      { http.Header(h).Set(key, value) }
func (h transportHeader) Add(key, value string)      { http.Header(h).Add(key, value) }
func (h transportHeader) Values(key string) []string { return http.Header(h).Values(key) }
func (h transportHeader) Keys() []string {
	keys := make([]string, 0, len(h))
	for key := range h {
		keys = append(keys, key)
	}
	return keys
}
