package metrics

import (
	"fmt"

	kmetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// 业务指标（Kratos middleware 使用）
var (
	MetricServerRequests metric.Int64Counter
	MetricServerSeconds  metric.Float64Histogram

	MetricClientRequests metric.Int64Counter
	MetricClientSeconds  metric.Float64Histogram
)

// setupBusinessMetrics 初始化业务指标
func setupBusinessMetrics(provider *sdkmetric.MeterProvider) error {
	serverMeter := provider.Meter("server")
	clientMeter := provider.Meter("client")

	var err error
	if MetricServerRequests, err = kmetrics.DefaultRequestsCounter(serverMeter, kmetrics.DefaultServerRequestsCounterName); err != nil {
		return fmt.Errorf("create server requests counter: %w", err)
	}

	if MetricServerSeconds, err = kmetrics.DefaultSecondsHistogram(serverMeter, kmetrics.DefaultServerSecondsHistogramName); err != nil {
		return fmt.Errorf("create server seconds histogram: %w", err)
	}

	if MetricClientRequests, err = kmetrics.DefaultRequestsCounter(clientMeter, kmetrics.DefaultClientRequestsCounterName); err != nil {
		return fmt.Errorf("create client requests counter: %w", err)
	}

	if MetricClientSeconds, err = kmetrics.DefaultSecondsHistogram(clientMeter, kmetrics.DefaultClientSecondsHistogramName); err != nil {
		return fmt.Errorf("create client seconds histogram: %w", err)
	}

	return nil
}
