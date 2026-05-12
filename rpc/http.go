package rpc

import (
	"context"
	"net/http"
	"net/http/pprof"

	"github.com/alec404/kratos-bootstrap/metrics"
	"github.com/go-kratos/aegis/ratelimit"
	"github.com/go-kratos/aegis/ratelimit/bbr"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	kratosMetrics "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/gorilla/handlers"

	"github.com/go-kratos/kratos/v2/middleware"
	midRateLimit "github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"

	kratosHttp "github.com/go-kratos/kratos/v2/transport/http"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
)

// CreateHTTPClient 创建HTTP客户端
func CreateHTTPClient(ctx context.Context, serviceName string, cfg *conf.Bootstrap, m ...middleware.Middleware) *kratosHttp.Client {

	endpoint := cfg.Client.Http.Addr

	var opts []kratosHttp.ClientOption

	var ms []middleware.Middleware
	if cfg.Client != nil && cfg.Client.Http != nil && cfg.Client.Http.Middleware != nil {
		if cfg.Client.Http.Middleware.GetEnableRecovery() {
			ms = append(ms, recovery.Recovery())
		}
		if cfg.Client.Http.Middleware.GetEnableTracing() {
			ms = append(ms, tracing.Client())
		}
		if cfg.Client.Http.Middleware.GetEnableValidate() {
			ms = append(ms, validate.Validator())
		}

		if cfg.Client.Http.Middleware.Metrics != nil {
			var options []kratosMetrics.Option

			if cfg.Client.Http.Middleware.Metrics.GetHistogram() {
				options = append(options, kratosMetrics.WithSeconds(metrics.MetricClientSeconds))
			}

			if cfg.Client.Http.Middleware.Metrics.GetCounter() {
				options = append(options, kratosMetrics.WithRequests(metrics.MetricClientRequests))
			}
			if len(options) > 0 {
				ms = append(ms, kratosMetrics.Client(options...))
			}
		}

	}
	ms = append(ms, m...)
	opts = append(opts, kratosHttp.WithMiddleware(ms...))

	if cfg.Client.Http.Timeout != nil {
		opts = append(opts, kratosHttp.WithTimeout(cfg.Client.Http.Timeout.AsDuration()))
	}

	opts = append(opts, kratosHttp.WithEndpoint(endpoint))

	client, connErr := kratosHttp.NewClient(ctx, opts...)

	if connErr != nil {
		log.Fatalf("dial http client [%s] failed: %s", serviceName, connErr.Error())
	}
	log.Infof("endpoint: %s, initialize %s, client success", endpoint, serviceName)

	return client
}

// CreateHTTPServer 创建HTTP服务端
func CreateHTTPServer(cfg *conf.Bootstrap, logger log.Logger, m ...middleware.Middleware) *kratosHttp.Server {
	return CreateHTTPServerWithOptions(cfg, logger, nil, m...)
}

// CreateHTTPServerWithOptions 创建HTTP服务端，并允许调用方注入 Kratos HTTP ServerOption。
func CreateHTTPServerWithOptions(cfg *conf.Bootstrap, logger log.Logger, serverOpts []kratosHttp.ServerOption, m ...middleware.Middleware) *kratosHttp.Server {
	var opts []kratosHttp.ServerOption

	var ms []middleware.Middleware
	if cfg.Server != nil && cfg.Server.Http != nil && cfg.Server.Http.Middleware != nil {
		if cfg.Server.Http.Cors != nil {
			opts = append(opts, kratosHttp.Filter(handlers.CORS(
				handlers.AllowedHeaders(cfg.Server.Http.Cors.Headers),
				handlers.AllowedMethods(cfg.Server.Http.Cors.Methods),
				handlers.AllowedOrigins(cfg.Server.Http.Cors.Origins),
			)))
		}

		if cfg.Server.Http.Middleware.GetEnableRecovery() {
			ms = append(ms, recovery.Recovery())
		}
		if cfg.Server.Http.Middleware.GetEnableTracing() {
			ms = append(ms, tracing.Server())
		}
		if cfg.Server.Http.Middleware.GetEnableValidate() {
			ms = append(ms, validate.Validator())
		}
		if cfg.Server.Http.Middleware.GetEnableCircuitBreaker() {
		}
		if cfg.Server.Http.Middleware.Limiter != nil {
			var limiter ratelimit.Limiter
			switch cfg.Server.Http.Middleware.Limiter.GetName() {
			case "bbr":
				limiter = bbr.NewLimiter()
			}
			ms = append(ms, midRateLimit.Server(midRateLimit.WithLimiter(limiter)))
		}

		if cfg.Server.Http.Middleware.GetEnableLogging() {
			ms = append(ms, logging.Server(logger))
		}

		if cfg.Server.Http.Middleware.Metrics != nil {
			var options []kratosMetrics.Option

			if cfg.Server.Http.Middleware.Metrics.GetHistogram() {
				options = append(options, kratosMetrics.WithSeconds(metrics.MetricServerSeconds))
			}

			if cfg.Server.Http.Middleware.Metrics.GetCounter() {
				options = append(options, kratosMetrics.WithRequests(metrics.MetricServerRequests))
			}
			if len(options) > 0 {
				ms = append(ms, kratosMetrics.Server(options...))
			}
		}

	}
	ms = append(ms, m...)
	opts = append(opts, kratosHttp.Middleware(ms...))

	if cfg.Server.Http.Network != "" {
		opts = append(opts, kratosHttp.Network(cfg.Server.Http.Network))
	}
	if cfg.Server.Http.Addr != "" {
		opts = append(opts, kratosHttp.Address(cfg.Server.Http.Addr))
	}
	if cfg.Server.Http.Timeout != nil {
		opts = append(opts, kratosHttp.Timeout(cfg.Server.Http.Timeout.AsDuration()))
	}

	// 调用方选项最后应用，方便业务侧注入或覆盖 HTTP Server 行为，例如 ErrorEncoder。
	opts = append(opts, serverOpts...)

	srv := kratosHttp.NewServer(opts...)

	if cfg.Server.Http.GetEnablePprof() {
		registerHttpPprof(srv)
	}

	srv.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	if cfg.GetServer().GetHttp().GetMiddleware().GetMetrics() != nil {
		srv.Handle("/metrics", metrics.PromHandler())
	}

	return srv
}

func registerHttpPprof(s *kratosHttp.Server) {
	s.HandleFunc("/debug/pprof", pprof.Index)
	s.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.HandleFunc("/debug/pprof/allocs", pprof.Handler("allocs").ServeHTTP)
	s.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	s.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	s.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	s.HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)
	s.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
}
