package metrics

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/alec404/kratos-bootstrap/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semConv "go.opentelemetry.io/otel/semconv/v1.20.0"
)

var (
	initOnce     sync.Once
	initErr      error
	initialized  bool
	otelProvider *sdkmetric.MeterProvider
	promHTTP     http.Handler
)

// NewMetricProvider 创建 Metrics Provider（主入口，与 tracer 保持一致）
func NewMetricProvider(cfg *conf.Metrics, serviceInfo *config.ServiceInfo) error {
	if cfg == nil {
		// 如果配置为空，使用默认配置
		return Init(serviceInfo)
	}

	config := NewConfigFromProto(cfg)
	return InitWithConfig(serviceInfo, config)
}

// Init 初始化 Metrics，使用默认配置
func Init(serviceInfo *config.ServiceInfo, opts ...Option) error {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	// 验证配置
	if err := ValidateConfig(cfg); err != nil {
		return fmt.Errorf("invalid metrics config: %w", err)
	}

	return InitWithConfig(serviceInfo, cfg)
}

// InitWithConfig 使用自定义配置初始化 Metrics
func InitWithConfig(serviceInfo *config.ServiceInfo, cfg *Config) error {
	// 验证配置
	if err := ValidateConfig(cfg); err != nil {
		return fmt.Errorf("invalid metrics config: %w", err)
	}

	initOnce.Do(func() {
		initErr = setupMetricProvider(serviceInfo, cfg)
		initialized = initErr == nil
	})
	return initErr
}

func setupMetricProvider(serviceInfo *config.ServiceInfo, cfg *Config) error {
	// 1. 创建 Readers（根据配置启用 Prometheus/OTLP）
	readers, err := NewMetricReaders(cfg.Exporters, cfg.Endpoint, cfg.Interval, cfg.Insecure)
	if err != nil {
		return fmt.Errorf("create metric readers: %w", err)
	}

	// 2. 资源标签（service/env/instance）
	res, err := resource.New(
		context.Background(),
		resource.WithFromEnv(),
		resource.WithHost(),
		resource.WithAttributes(
			semConv.ServiceName(serviceInfo.Name),
			semConv.ServiceInstanceID(serviceInfo.Id),
			semConv.DeploymentEnvironment(cfg.Env),
		),
	)
	if err != nil {
		return fmt.Errorf("create resource: %w", err)
	}

	// 3. 创建 MeterProvider
	var opts []sdkmetric.Option
	for _, reader := range readers {
		opts = append(opts, sdkmetric.WithReader(reader))
	}
	opts = append(opts, sdkmetric.WithResource(res))

	otelProvider = sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(otelProvider)

	// 4. 启用 Runtime Metrics（替代 Prometheus 原生 go/process collectors）
	if cfg.EnableRuntimeMetrics {
		if err := runtime.Start(
			runtime.WithMinimumReadMemStatsInterval(cfg.RuntimeReadMemStatsInterval),
			runtime.WithMeterProvider(otelProvider),
		); err != nil {
			fmt.Printf("[WARN] start otel runtime metrics failed: %v\n", err)
		}

	}

	// 5. 初始化业务指标
	if err := setupBusinessMetrics(otelProvider); err != nil {
		return fmt.Errorf("setup business metrics: %w", err)
	}

	//  6. 初始化网络指标
	if cfg.EnableHostMetrics {
		err = host.Start(host.WithMeterProvider(otelProvider))
		if err != nil {
			fmt.Printf("[WARN] start otel host metrics failed: %v\n", err)
		}
	}

	// 7. 设置 Prometheus HTTP Handler（仅在启用 Prometheus 时）
	if hasPrometheusExporter(cfg.Exporters) {
		promHTTP = promhttp.Handler()
	}

	return nil
}

// PromHandler 返回 Prometheus /metrics 的 HTTP Handler
func PromHandler() http.Handler {
	if !initialized || promHTTP == nil {
		return http.NotFoundHandler()
	}
	return promHTTP
}

// hasPrometheusExporter 检查是否启用了 Prometheus 导出器
func hasPrometheusExporter(exporters []ExporterType) bool {
	for _, exp := range exporters {
		if exp == ExporterTypePrometheus {
			return true
		}
	}
	return false
}
