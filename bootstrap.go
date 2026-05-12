package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/alec404/kratos-bootstrap/config"
	"github.com/alec404/kratos-bootstrap/logger"
	"github.com/alec404/kratos-bootstrap/metrics"
	"github.com/alec404/kratos-bootstrap/tracer"
)

var (
	Service = config.NewServiceInfo(
		"",
		"1.0.0",
		"",
	)
)

const DefaultBeforeStopDelay time.Duration = 0

type appOptions struct {
	beforeStopDelay time.Duration
}

// AppOption 配置应用启动/停止行为。
type AppOption func(*appOptions)

func newAppOptions(opts ...AppOption) *appOptions {
	o := &appOptions{
		beforeStopDelay: DefaultBeforeStopDelay,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// WithBeforeStopDelay 设置停止前等待时间。
//
// 该等待可用于给网关、负载均衡或服务发现摘流留出缓冲时间。
// delay <= 0 时不注册 BeforeStop 等待逻辑。
func WithBeforeStopDelay(delay time.Duration) AppOption {
	return func(o *appOptions) {
		o.beforeStopDelay = delay
	}
}

// NewApp 创建应用程序
func NewApp(ll log.Logger, srv ...transport.Server) *kratos.App {
	return NewAppWithOptions(ll, srv)
}

// NewAppWithOptions 创建应用程序，并允许调用方覆盖应用级选项。
func NewAppWithOptions(ll log.Logger, srv []transport.Server, opts ...AppOption) *kratos.App {
	if ll == nil {
		ll = log.DefaultLogger
	}
	o := newAppOptions(opts...)
	helper := log.NewHelper(ll)

	kratosOpts := []kratos.Option{
		kratos.ID(Service.GetInstanceId()),
		kratos.Name(Service.Name),
		kratos.Version(Service.Version),
		kratos.Metadata(Service.Metadata),
		kratos.Logger(ll),
	}

	if o.beforeStopDelay > 0 {
		kratosOpts = append(kratosOpts, kratos.BeforeStop(func(_ context.Context) error {
			helper.Infow("msg", "app stopping, waiting before stop", "delay", o.beforeStopDelay.String())
			time.Sleep(o.beforeStopDelay)
			return nil
		}))
	}

	kratosOpts = append(kratosOpts,
		kratos.Server(
			srv...,
		),
	)

	return kratos.New(kratosOpts...)
}

// DoBootstrap 执行引导
func DoBootstrap(serviceInfo *config.ServiceInfo) (*conf.Bootstrap, log.Logger) {
	// inject command flags
	Flags := config.NewCommandFlags()
	Flags.Init()

	var err error

	// load configs
	if err = config.LoadBootstrapConfig(Flags.Conf); err != nil {
		panic(fmt.Sprintf("load config failed: %v", err))
	}

	// init logger
	ll := logger.NewLoggerProvider(config.GetBootstrapConfig().Logger, serviceInfo)

	// init tracer
	if err = tracer.NewTracerProvider(config.GetBootstrapConfig().Trace, serviceInfo); err != nil {
		panic(fmt.Sprintf("init tracer failed: %v", err))
	}

	// init metrics
	if err = metrics.NewMetricProvider(config.GetBootstrapConfig().Metrics, serviceInfo); err != nil {
		panic(fmt.Sprintf("init metrics failed: %v", err))
	}

	return config.GetBootstrapConfig(), ll
}

type InitApp[T any] func(logger log.Logger, bootstrap *conf.Bootstrap, customCfg *T) (*kratos.App, func(), error)

// Bootstrap 应用引导启动
func Bootstrap[T any](initApp InitApp[T], serviceName, version *string, customCfg *T) {
	if serviceName != nil && len(*serviceName) != 0 {
		Service.Name = *serviceName
	}
	if version != nil && len(*version) != 0 {
		Service.Version = *version
	}

	if customCfg != nil {
		config.RegisterConfig(customCfg)
	}

	// bootstrap
	cfg, ll := DoBootstrap(Service)

	// init app
	app, cleanup, err := initApp(ll, cfg, customCfg)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// run the app.
	if err = app.Run(); err != nil {
		panic(err)
	}
}
