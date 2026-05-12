package bootstrap

import (
	"testing"
	"time"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
)

type CustomConfig struct {
	Cfg string `protobuf:"bytes,1,opt,name=cfg,proto3" json:"cfg,omitempty"`
}

func initApp(logger log.Logger, _ *conf.Bootstrap, _ *CustomConfig) (*kratos.App, func(), error) {
	app := NewApp(logger)
	return app, func() {
	}, nil
}

func TestNewApp(t *testing.T) {
	serviceName := "test"
	version := "v0.0.1"
	oldService := Service
	Service = Service.Clone()
	t.Cleanup(func() {
		Service = oldService
	})

	Service.SetName(serviceName)
	Service.SetVersion(version)
	app, cleanup, err := initApp(log.DefaultLogger, &conf.Bootstrap{}, &CustomConfig{})
	if err != nil {
		t.Fatalf("init app: %v", err)
	}
	if app == nil {
		t.Fatal("app is nil")
	}
	cleanup()
}

func TestNewAppWithOptions(t *testing.T) {
	app := NewAppWithOptions(
		log.DefaultLogger,
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
