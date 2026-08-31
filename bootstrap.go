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
	"google.golang.org/protobuf/proto"
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

// NewApp 使用启动 Context 创建应用程序。
func NewApp(ctx *Context, srv ...transport.Server) *kratos.App {
	return NewAppWithOptions(ctx, srv)
}

// NewAppWithOptions 创建应用程序，并允许调用方覆盖应用级选项。
func NewAppWithOptions(ctx *Context, srv []transport.Server, opts ...AppOption) *kratos.App {
	if ctx == nil {
		panic("bootstrap context is nil")
	}

	ll := LoggerFromContext(ctx)
	if ll == nil {
		ll = log.DefaultLogger
	}
	appInfo := AppInfoFromContext(ctx)
	if appInfo == nil {
		panic("bootstrap app info is nil")
	}

	o := newAppOptions(opts...)
	helper := log.NewHelper(ll)

	kratosOpts := []kratos.Option{
		kratos.ID(appInfo.GetInstanceId()),
		kratos.Name(config.ServiceName(appInfo)),
		kratos.Version(appInfo.GetVersion()),
		kratos.Metadata(appInfo.GetMetadata()),
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
func DoBootstrap(appInfo *conf.AppInfo, customConfigs ...proto.Message) (*conf.Bootstrap, log.Logger) {
	// inject command flags
	Flags := config.NewCommandFlags()
	Flags.Init()

	var err error

	// load configs
	bootstrapConfig, err := config.LoadBootstrapConfig(Flags.Conf, customConfigs...)
	if err != nil {
		panic(fmt.Sprintf("load config failed: %v", err))
	}

	// init logger
	ll := logger.NewLoggerProvider(bootstrapConfig.Logger, appInfo)

	// init tracer
	if err = tracer.NewTracerProvider(bootstrapConfig.Trace, appInfo); err != nil {
		panic(fmt.Sprintf("init tracer failed: %v", err))
	}

	// init metrics
	if err = metrics.NewMetricProvider(bootstrapConfig.Metrics, appInfo); err != nil {
		panic(fmt.Sprintf("init metrics failed: %v", err))
	}

	return bootstrapConfig, ll
}

type InitApp[T any] func(logger log.Logger, bootstrap *conf.Bootstrap, customCfg *T) (*kratos.App, func(), error)

// Bootstrap 应用引导启动。
//
// Deprecated: 使用 NewContext、RegisterCustomConfig 和 Context.Run。
func Bootstrap[T any](initApp InitApp[T], appID, version *string, customCfg *T) {
	info := &conf.AppInfo{}
	if appID != nil && len(*appID) != 0 {
		info.AppId = *appID
	}
	if version != nil && len(*version) != 0 {
		info.Version = *version
	}

	ctx := NewContext(info)
	if customCfg != nil {
		customConfig, ok := any(customCfg).(proto.Message)
		if !ok {
			panic(fmt.Sprintf("custom config %T must implement proto.Message", customCfg))
		}
		ctx.RegisterCustomConfig("custom", customConfig)
	}
	ctx.Run(func(ctx *Context) (*kratos.App, func(), error) {
		return initApp(LoggerFromContext(ctx), ConfigFromContext(ctx), customCfg)
	})
}
