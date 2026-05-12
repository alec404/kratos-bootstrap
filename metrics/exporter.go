package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	otlpmetricgrpc "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otlpmetrichttp "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	exporterPrometheus "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// ExporterType 定义导出器类型
type ExporterType string

const (
	ExporterTypePrometheus ExporterType = "prometheus"
	ExporterTypeOTLPGRPC   ExporterType = "otlp-grpc"
	ExporterTypeOTLPHTTP   ExporterType = "otlp-http"
	// 兼容别名
	ExporterTypeOTLP ExporterType = "otlp" // 默认使用 gRPC
)

// NewMetricReaders 根据配置创建 metric readers
func NewMetricReaders(exporters []ExporterType, endpoint string, interval time.Duration, insecure bool) ([]sdkmetric.Reader, error) {
	if len(exporters) == 0 {
		return nil, errors.New("at least one exporter must be specified")
	}

	var readers []sdkmetric.Reader

	for _, exp := range exporters {
		reader, err := newMetricReader(exp, endpoint, interval, insecure)
		if err != nil {
			return nil, fmt.Errorf("create %s reader: %w", exp, err)
		}
		readers = append(readers, reader)
	}

	return readers, nil
}

// newMetricReader 创建单个 metric reader
func newMetricReader(exporterType ExporterType, endpoint string, interval time.Duration, insecure bool) (sdkmetric.Reader, error) {
	switch exporterType {
	case ExporterTypePrometheus:
		return NewPrometheusReader()
	case ExporterTypeOTLP, ExporterTypeOTLPGRPC:
		return NewOTLPGRPCReader(context.Background(), endpoint, interval, insecure)
	case ExporterTypeOTLPHTTP:
		return NewOTLPHTTPReader(context.Background(), endpoint, interval, insecure)
	default:
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}
}

// NewPrometheusReader 创建 Prometheus 导出器（使用默认 Registry）
func NewPrometheusReader() (sdkmetric.Reader, error) {
	return exporterPrometheus.New()
}

// NewOTLPGRPCReader 创建 OTLP gRPC 导出器（默认端口：4317）
func NewOTLPGRPCReader(ctx context.Context, endpoint string, interval time.Duration, insecure bool) (sdkmetric.Reader, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(endpoint),
	}

	if insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(interval)), nil
}

// NewOTLPHTTPReader 创建 OTLP HTTP 导出器（默认端口：4318）
func NewOTLPHTTPReader(ctx context.Context, endpoint string, interval time.Duration, insecure bool) (sdkmetric.Reader, error) {
	opts := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(endpoint),
	}

	if insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}

	exporter, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(interval)), nil
}
