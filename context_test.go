package bootstrap

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/alec404/kratos-bootstrap/config"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{
		Project:  "example",
		AppId:    "service",
		Version:  "v1.0.0",
		Hostname: "pod-1",
	})

	if got := config.ServiceName(ctx.appInfo); got != "example-service" {
		t.Fatalf("unexpected service name: got %q", got)
	}
	if ctx.appInfo.ServiceName != "example-service" {
		t.Fatalf("unexpected service name: got %q", ctx.appInfo.ServiceName)
	}
	if ctx.appInfo.Version != "v1.0.0" {
		t.Fatalf("unexpected version: got %q", ctx.appInfo.Version)
	}
	if ctx.appInfo.InstanceId != "pod-1" {
		t.Fatalf("unexpected instance ID: got %q, want %q", ctx.appInfo.InstanceId, "pod-1")
	}
	if ctx.customConfigs == nil {
		t.Fatal("custom config map is nil")
	}
}

func TestAppInfoFromContextReturnsClone(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{
		Project:  "example",
		AppId:    "service",
		Hostname: "pod-1",
		Metadata: map[string]string{"zone": "cn"},
	})

	got := AppInfoFromContext(ctx)
	got.Project = "changed"
	got.Metadata["zone"] = "us"

	again := AppInfoFromContext(ctx)
	if again.Project != "example" {
		t.Fatalf("context app info was mutated: got project %q", again.Project)
	}
	if again.Metadata["zone"] != "cn" {
		t.Fatalf("context metadata was mutated: got zone %q", again.Metadata["zone"])
	}
	if AppInfoFromContext(nil) != nil {
		t.Fatal("nil context should return nil app info")
	}
}

func TestContextProviders(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{})
	cfg := &conf.Bootstrap{}
	ctx.config = cfg
	ctx.logger = log.DefaultLogger

	if got := ConfigFromContext(ctx); got != cfg {
		t.Fatalf("unexpected config: got %p, want %p", got, cfg)
	}
	if got := LoggerFromContext(ctx); got != log.DefaultLogger {
		t.Fatalf("unexpected logger: got %T, want %T", got, log.DefaultLogger)
	}
	if ConfigFromContext(nil) != nil {
		t.Fatal("nil context should return nil config")
	}
	if LoggerFromContext(nil) != nil {
		t.Fatal("nil context should return nil logger")
	}
}

func TestContextPrintsStartupInfo(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{
		Project:  "example",
		AppId:    "core",
		Hostname: "core-pod-1",
	})

	output := captureStdout(t, ctx.PrintAppInfo)
	for _, want := range []string{
		"example-core",
		"Project: example",
		"AppId: core",
		"Version: 1.0.0",
		"InstanceId: core-pod-1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("startup output missing %q: %q", want, output)
		}
	}
}

func TestContextSkipsStartupInfoForInvalidApp(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{})
	output := captureStdout(t, ctx.PrintAppInfo)
	if output != "" {
		t.Fatalf("unexpected startup info for invalid app: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = old
		_ = reader.Close()
	}()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(output)
}

func TestContextCustomConfigs(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{})
	first := &conf.AppInfo{Project: "first"}
	second := &conf.Bootstrap{}

	ctx.RegisterCustomConfig("first", first)
	ctx.RegisterCustomConfig("second", second)

	gotFirst, err := GetCustomConfig[*conf.AppInfo](ctx, "first")
	if err != nil {
		t.Fatalf("get first custom config: %v", err)
	}
	if gotFirst != first {
		t.Fatalf("unexpected first custom config: got %p, want %p", gotFirst, first)
	}

	gotSecond, err := GetCustomConfig[*conf.Bootstrap](ctx, "second")
	if err != nil {
		t.Fatalf("get second custom config: %v", err)
	}
	if gotSecond != second {
		t.Fatalf("unexpected second custom config: got %p, want %p", gotSecond, second)
	}
}

func TestContextCustomConfigLoadedFromBootstrapSource(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{})
	target := &conf.AppInfo{}
	ctx.RegisterCustomConfig("app", target)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.yaml"), []byte("project: loaded\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := config.LoadBootstrapConfig(dir, ctx.customConfigTargets()...); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if target.Project != "loaded" {
		t.Fatalf("unexpected custom config value: got %q, want %q", target.Project, "loaded")
	}
}

func TestContextCustomConfigsAreIsolated(t *testing.T) {
	firstContext := NewContext(&conf.AppInfo{})
	firstTarget := &conf.AppInfo{}
	firstContext.RegisterCustomConfig("app", firstTarget)

	secondContext := NewContext(&conf.AppInfo{})
	secondTarget := &conf.AppInfo{}
	secondContext.RegisterCustomConfig("app", secondTarget)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.yaml"), []byte("project: first\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := config.LoadBootstrapConfig(dir, firstContext.customConfigTargets()...); err != nil {
		t.Fatalf("load first context config: %v", err)
	}

	if firstTarget.Project != "first" {
		t.Fatalf("first target project = %q, want %q", firstTarget.Project, "first")
	}
	if secondTarget.Project != "" {
		t.Fatalf("second context target was mutated: got project %q", secondTarget.Project)
	}
}

func TestContextRejectsDuplicateCustomConfig(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{})
	ctx.RegisterCustomConfig("app", &conf.AppInfo{})

	assertContextPanicIs(t, ErrDuplicateCustomConfig, func() {
		ctx.RegisterCustomConfig("app", &conf.AppInfo{})
	})
}

func TestContextRejectsInvalidCustomConfig(t *testing.T) {
	var nilTarget *conf.AppInfo
	tests := []struct {
		name   string
		key    string
		target proto.Message
	}{
		{name: "empty key", target: &conf.AppInfo{}},
		{name: "nil target", key: "app"},
		{name: "typed nil target", key: "app", target: nilTarget},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext(&conf.AppInfo{})
			assertContextPanicIs(t, ErrInvalidCustomConfig, func() {
				ctx.RegisterCustomConfig(tt.key, tt.target)
			})
		})
	}
}

func TestContextRejectsCustomConfigRegistrationAfterStartup(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{})
	ctx.config = &conf.Bootstrap{}

	assertContextPanicIs(t, ErrInvalidCustomConfig, func() {
		ctx.RegisterCustomConfig("app", &conf.AppInfo{})
	})
}

func TestGetCustomConfigErrors(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{})
	ctx.RegisterCustomConfig("app", &conf.AppInfo{})

	if _, err := GetCustomConfig[*conf.AppInfo](ctx, "missing"); !errors.Is(err, ErrCustomConfigNotFound) {
		t.Fatalf("unexpected missing error: got %v, want %v", err, ErrCustomConfigNotFound)
	}
	if _, err := GetCustomConfig[*conf.Bootstrap](ctx, "app"); !errors.Is(err, ErrCustomConfigTypeMismatch) {
		t.Fatalf("unexpected type mismatch error: got %v, want %v", err, ErrCustomConfigTypeMismatch)
	}
	if _, err := GetCustomConfig[*conf.AppInfo](nil, "app"); !errors.Is(err, ErrCustomConfigNotFound) {
		t.Fatalf("unexpected nil context error: got %v, want %v", err, ErrCustomConfigNotFound)
	}
}

func TestNilContextRejectsCustomConfig(t *testing.T) {
	var ctx *Context
	defer func() {
		if recover() == nil {
			t.Fatal("expected nil context registration to panic")
		}
	}()
	ctx.RegisterCustomConfig("app", &conf.AppInfo{})
}

func assertContextPanicIs(t *testing.T, target error, fn func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		err, ok := recovered.(error)
		if !ok || !errors.Is(err, target) {
			t.Fatalf("unexpected panic: got %v, want error matching %v", recovered, target)
		}
	}()
	fn()
}
