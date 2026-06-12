# kratos-bootstrap

`kratos-bootstrap` 是一个面向 [go-kratos/kratos](https://github.com/go-kratos/kratos) 的 Go 服务启动辅助库。它封装了 Kratos 服务常见的启动、配置加载、日志、链路追踪、指标、HTTP/gRPC 服务初始化，以及部分常用数据客户端的基础接入。

## 功能特性

- 基于 `kratos.App` 的应用启动辅助方法。
- 统一的 protobuf 配置定义，位于 `api/proto/conf/v1`。
- 支持从环境变量和本地配置文件加载配置。
- 支持 Kratos 标准日志和 zap 日志。
- 支持 OpenTelemetry 链路追踪，包含 stdout、OTLP/gRPC、OTLP/HTTP exporter。
- 支持 Prometheus 和 OTLP 指标导出。
- 提供 HTTP/gRPC Server 和 Client 的快速创建方法。
- 提供可选的 Redis、Elasticsearch、OpenSearch、Ent 客户端辅助模块。

## 安装

```bash
go get github.com/alec404/kratos-bootstrap
```

可选的数据客户端辅助模块是独立 Go module，需要时单独安装：

```bash
go get github.com/alec404/kratos-bootstrap/cache/redis
go get github.com/alec404/kratos-bootstrap/data/elasticsearch
go get github.com/alec404/kratos-bootstrap/data/opensearch
go get github.com/alec404/kratos-bootstrap/data/ent
```

## 目录结构

```text
api/                  protobuf 配置定义和生成后的 Go 类型
bootstrap.go          应用启动辅助方法
config/               配置加载和服务元信息
logger/               Kratos logger provider
metrics/              OpenTelemetry metrics 初始化
rpc/                  HTTP/gRPC Server 和 Client 辅助方法
tracer/               OpenTelemetry tracing 初始化
cache/redis/          可选 Redis 辅助模块
data/elasticsearch/   可选 Elasticsearch 辅助模块
data/opensearch/      可选 OpenSearch 辅助模块
data/ent/             可选 Ent 辅助模块
```

## 生成 protobuf 代码

本仓库使用 `buf` 和 `protoc-gen-go` 生成 Go 代码。

```bash
make api
```

该命令会进入 `api/` 目录执行 `buf generate`，生成结果输出到 `api/gen/go`。

## 基础用法

```go
package main

import (
	"github.com/alec404/kratos-bootstrap"
	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/alec404/kratos-bootstrap/config"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
)

type CustomConfig struct{}

func initApp(logger log.Logger, cfg *conf.Bootstrap, custom *CustomConfig) (*kratos.App, func(), error) {
	app := bootstrap.NewApp(logger)
	return app, func() {}, nil
}

func main() {
	serviceName := "example-service"
	version := "v0.1.0"

	bootstrap.Service = config.NewServiceInfo(serviceName, version, "")
	bootstrap.Bootstrap(initApp, &serviceName, &version, &CustomConfig{})
}
```

实际项目通常会在 `initApp` 中创建 HTTP/gRPC server，然后传给 `bootstrap.NewApp`。

## 应用停止前等待

`NewApp` 默认不注册停机前等待：

```go
const DefaultBeforeStopDelay = 0
```

如需给网关、负载均衡或服务发现系统留出摘流时间，可以显式使用 `NewAppWithOptions` 注入等待窗口：

```go
app := bootstrap.NewAppWithOptions(
	logger,
	[]transport.Server{httpServer, grpcServer},
	bootstrap.WithBeforeStopDelay(10*time.Second),
)
```

如需保持默认不等待，可以继续使用：

```go
app := bootstrap.NewApp(logger, httpServer)
```

显式配置正数等待时间后，等待逻辑会使用 `time.Sleep`，确保进程在配置的 drain window 内持续等待。

## HTTP Server

```go
httpServer := rpc.CreateHTTPServer(cfg, logger)
```

`CreateHTTPServer` 支持通过配置启用以下能力：

- recovery
- tracing
- tracing operation exclude（按 transport.Operation() 精确/前缀排除，HTTP/gRPC 通用）
- validation
- logging
- BBR rate limit
- Kratos metrics middleware
- CORS
- pprof

同时会注册：

- `/ready`：返回 `200 OK`
- `/metrics`：当 HTTP server metrics 配置启用时注册

如果需要注入原生 Kratos HTTP ServerOption，可以使用 `CreateHTTPServerWithOptions`：

```go
httpServer := rpc.CreateHTTPServerWithOptions(
	cfg,
	logger,
	[]kratosHttp.ServerOption{
		kratosHttp.ErrorEncoder(customErrorEncoder),
	},
)
```

如果部分 HTTP/gRPC operation 不需要创建 span，可以在对应 `middleware.tracing` 下配置排除规则：

```yaml
server:
  http:
    middleware:
      enable_tracing: true
      tracing:
        exclude_operations:
          - /api/items/{id}
          - /helloworld.Greeter/SayHello
        exclude_operation_prefixes:
          - /debug/pprof/
          - /helloworld.InternalService/
```

`exclude_operations` 和 `exclude_operation_prefixes` 统一匹配 Kratos `transport.Operation()`：HTTP 通常是 path template（如 `/api/items/{id}`），gRPC 通常是 full method（如 `/helloworld.Greeter/SayHello`）。

命中排除规则后，不只跳过当前 server/client span；还会把当前 context 标记为 unsampled parent，使后续 child span 和下游服务调用在 `ParentBased` sampler 下继续保持不采样。

跨服务抑制依赖 client tracing middleware 注入 unsampled `traceparent` 到下游请求；因此如果希望“当前接口及其下游调用都不产生 trace”，对应下游 client 的 `enable_tracing` 仍需保持开启。

## gRPC Server

```go
grpcServer := rpc.CreateGrpcServer(cfg, logger)
```

`CreateGrpcServer` 支持通过配置启用以下能力：

- recovery
- tracing
- validation
- metadata
- logging
- JWT auth
- BBR rate limit
- Kratos metrics middleware

客户端和服务端的最大消息大小可通过 `max_msg_size` 配置，单位为 MB。

## 配置结构

根配置消息是 `conf.Bootstrap`：

```proto
message Bootstrap {
  Server server = 1;
  Client client = 2;
  Data data = 3;
  Tracer trace = 4;
  Logger logger = 5;
  Notification notify = 9;
  Metrics metrics = 10;
}
```

主要配置块：

- `server`：HTTP/gRPC 监听地址、超时、CORS、TLS、中间件。
- `client`：HTTP/gRPC 客户端 endpoint、超时、TLS、中间件。
- `data`：数据库、Redis、Elasticsearch、OpenSearch、RabbitMQ 连接配置。
- `trace`：OpenTelemetry tracing exporter 配置。
- `logger`：标准日志或 zap 日志配置。
- `metrics`：Prometheus/OTLP 指标配置。
- `notify`：通用通知开关，具体通知通道由上层适配库实现。

配置加载基于 Kratos config source：

```go
cfg := config.NewConfigProvider("../../configs")
```

`DoBootstrap` 会读取 `-conf` 命令行参数作为配置路径，默认值为 `../../configs`。

## Logger

支持的日志类型：

- `std`
- `zap`

zap 日志支持控制台输出、文件输出、错误文件输出、日志级别、文件大小轮转、保留天数、备份数量和压缩。

## Tracing

链路追踪支持：

- `stdout`
- `otlp-grpc`
- `otlp-http`

不直接支持 `jaeger` exporter；如需接入 Jaeger，建议通过 OTLP Collector 转发。

## Metrics

指标导出支持：

- `prometheus`
- `otlp-grpc`
- `otlp-http`

`otlp-grpc` 和 `otlp-http` 不能同时启用。Prometheus 可以和其中一个 OTLP exporter 同时启用。

## 校验

运行主模块测试：

```bash
go test ./...
```

运行可选辅助模块测试：

```bash
cd cache/redis && go test ./...
cd ../../data/elasticsearch && go test ./...
cd ../ent && go test ./...
```

运行 protobuf lint：

```bash
cd api && buf lint
```

## License

MIT
