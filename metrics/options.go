package metrics

import (
	"fmt"
	"os"
	"strings"
	"time"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
)

// Config 指标配置
type Config struct {
	// Exporters 启用的导出器类型
	Exporters []ExporterType

	// OTLP 配置
	Endpoint string
	Interval time.Duration
	Insecure bool

	// 环境配置
	Env string

	// Runtime metrics 配置
	EnableRuntimeMetrics        bool
	RuntimeReadMemStatsInterval time.Duration

	// Host metrics 配置
	EnableHostMetrics bool
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Exporters:                   []ExporterType{ExporterTypePrometheus},
		Endpoint:                    getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		Interval:                    15 * time.Second,
		Insecure:                    true,
		Env:                         getEnvOrDefault("GO_ENV", "test"),
		EnableRuntimeMetrics:        true,
		RuntimeReadMemStatsInterval: 5 * time.Second, // 默认 5 秒
		EnableHostMetrics:           true,
	}
}

// NewConfigFromProto 从 protobuf 配置创建 Config
func NewConfigFromProto(cfg *conf.Metrics) *Config {
	if cfg == nil {
		return DefaultConfig()
	}

	config := DefaultConfig()

	// 解析导出器类型
	if len(cfg.Exporters) > 0 {
		config.Exporters = parseExporters(cfg.Exporters)
	}

	// OTLP 配置
	if cfg.Endpoint != "" {
		config.Endpoint = cfg.Endpoint
	}
	if cfg.Interval > 0 {
		config.Interval = time.Duration(cfg.Interval) * time.Second
	}
	config.Insecure = cfg.Insecure

	// 环境配置
	if cfg.Env != "" {
		config.Env = cfg.Env
	} else {
		config.Env = getEnvOrDefault("GO_ENV", "test")
	}

	// 网络接口
	config.EnableHostMetrics = cfg.GetEnableHostMetrics()

	// Runtime metrics
	config.EnableRuntimeMetrics = cfg.GetEnableRuntimeMetrics()

	if cfg.RuntimeReadMemstatsInterval > 0 {
		config.RuntimeReadMemStatsInterval = time.Duration(cfg.RuntimeReadMemstatsInterval) * time.Second
	}

	return config
}

// parseExporters 解析导出器类型字符串
func parseExporters(exporters []string) []ExporterType {
	var result []ExporterType
	for _, exp := range exporters {
		switch strings.ToLower(strings.TrimSpace(exp)) {
		case "prometheus":
			result = append(result, ExporterTypePrometheus)
		case "otlp", "otlp-grpc":
			result = append(result, ExporterTypeOTLPGRPC)
		case "otlp-http":
			result = append(result, ExporterTypeOTLPHTTP)
		}
	}
	if len(result) == 0 {
		result = []ExporterType{ExporterTypePrometheus}
	}
	return result
}

// getEnvOrDefault 获取环境变量或返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// Option 配置选项函数
type Option func(*Config)

// WithExporters 设置导出器类型（会自动验证 OTLP 导出器互斥）
func WithExporters(exporters ...ExporterType) Option {
	return func(c *Config) {
		c.Exporters = exporters
	}
}

// WithEndpoint 设置 OTLP endpoint
func WithEndpoint(endpoint string) Option {
	return func(c *Config) {
		c.Endpoint = endpoint
	}
}

// WithInterval 设置 OTLP 推送间隔
func WithInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.Interval = interval
	}
}

// WithEnv 设置环境
func WithEnv(env string) Option {
	return func(c *Config) {
		c.Env = env
	}
}

// WithRuntimeMetrics 设置是否启用 runtime metrics
func WithRuntimeMetrics(enable bool) Option {
	return func(c *Config) {
		c.EnableRuntimeMetrics = enable
	}
}

// ValidateConfig 验证配置（确保 OTLP gRPC 和 HTTP 不同时启用）
func ValidateConfig(cfg *Config) error {
	hasGRPC := false
	hasHTTP := false

	for _, exp := range cfg.Exporters {
		if exp == ExporterTypeOTLPGRPC || exp == ExporterTypeOTLP {
			hasGRPC = true
		}
		if exp == ExporterTypeOTLPHTTP {
			hasHTTP = true
		}
	}

	if hasGRPC && hasHTTP {
		return fmt.Errorf("otlp-grpc and otlp-http cannot be enabled at the same time, please choose one")
	}

	return nil
}
