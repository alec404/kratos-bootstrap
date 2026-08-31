package tracer

import (
	"strings"
	"testing"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
)

func TestNewTracerProviderAllowsOptionalInputs(t *testing.T) {
	appInfo := &conf.AppInfo{AppId: "core", ServiceName: "core"}

	if err := NewTracerProvider(nil, appInfo); err != nil {
		t.Fatalf("nil tracer config should be ignored: %v", err)
	}
	if err := NewTracerProvider(&conf.Tracer{}, nil); err != nil {
		t.Fatalf("nil app info should be ignored: %v", err)
	}
	if err := NewTracerProvider(&conf.Tracer{}, appInfo); err != nil {
		t.Fatalf("valid tracer inputs should initialize: %v", err)
	}
}

func TestNewTracerProviderReturnsExporterError(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("exporter errors must be returned, not panicked: %v", recovered)
		}
	}()

	err := NewTracerProvider(
		&conf.Tracer{Batcher: "jaeger", Endpoint: "collector:4317"},
		&conf.AppInfo{AppId: "core", ServiceName: "core"},
	)
	if err == nil {
		t.Fatal("expected unsupported exporter error")
	}
	if !strings.Contains(err.Error(), "jaeger") {
		t.Fatalf("unexpected exporter error: %v", err)
	}
}
