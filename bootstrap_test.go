package bootstrap

import (
	"testing"
	"time"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
)

func TestNewApp(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{
		Project:    "example",
		AppId:      "worker",
		Version:    "v0.0.1",
		Hostname:   "pod-1",
		InstanceId: "instance-1",
		Metadata:   map[string]string{"zone": "cn"},
	})
	ctx.logger = log.DefaultLogger

	app := NewApp(ctx)
	if got := app.Name(); got != "example-worker" {
		t.Fatalf("unexpected app name: got %q, want %q", got, "example-worker")
	}
	if got := app.ID(); got != "instance-1" {
		t.Fatalf("unexpected app ID: got %q, want %q", got, "instance-1")
	}
	if got := app.Version(); got != "v0.0.1" {
		t.Fatalf("unexpected app version: got %q, want %q", got, "v0.0.1")
	}
	if got := app.Metadata()["zone"]; got != "cn" {
		t.Fatalf("unexpected app metadata: got %q, want %q", got, "cn")
	}
}

func TestNewAppWithOptions(t *testing.T) {
	ctx := NewContext(&conf.AppInfo{Project: "example", AppId: "worker", Hostname: "pod-1"})
	ctx.logger = log.DefaultLogger
	app := NewAppWithOptions(
		ctx,
		nil,
		WithBeforeStopDelay(0),
	)
	if app == nil {
		t.Fatal("app is nil")
	}
}

func TestNewAppOptions(t *testing.T) {
	opts := newAppOptions()
	if opts.beforeStopDelay != DefaultBeforeStopDelay {
		t.Fatalf("unexpected default before stop delay: got %s, want %s", opts.beforeStopDelay, DefaultBeforeStopDelay)
	}

	customDelay := 3 * time.Second
	opts = newAppOptions(WithBeforeStopDelay(customDelay))
	if opts.beforeStopDelay != customDelay {
		t.Fatalf("unexpected custom before stop delay: got %s, want %s", opts.beforeStopDelay, customDelay)
	}
}
