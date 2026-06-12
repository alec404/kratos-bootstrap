package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/alec404/kratos-bootstrap/metrics"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metadata"
	kratosMetrics "github.com/go-kratos/kratos/v2/middleware/metrics"

	"github.com/go-kratos/aegis/ratelimit"
	"github.com/go-kratos/aegis/ratelimit/bbr"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	midRateLimit "github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/validate"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	kratosGrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

const (
	defaultTimeout        = 5 * time.Second
	defaultCaFile         = "pkg/cert/ca/ca.crt"
	defaultServerCertFile = "pkg/cert/server/server.crt"
	defaultClientCertFile = "pkg/cert/client/client.crt"
	defaultServerName     = "localhost"
	defaultMsgSize        = 16 * 1024 * 1024
)

// CreateGrpcClient 创建GRPC客户端
func CreateGrpcClient(ctx context.Context, serviceName string, cfg *conf.Bootstrap, m ...middleware.Middleware) grpc.ClientConnInterface {
	endpoint := cfg.Client.Grpc.Addr

	var opts []kratosGrpc.ClientOption
	var ms []middleware.Middleware
	var conn *grpc.ClientConn
	var connErr error
	enableTls := false
	timeout := defaultTimeout
	serverName := defaultServerName
	caFile := defaultCaFile
	certFile := defaultClientCertFile
	msgSize := defaultMsgSize

	if cfg.Client != nil && cfg.Client.Grpc != nil {
		if cfg.Client.Grpc.Timeout != nil {
			timeout = cfg.Client.Grpc.Timeout.AsDuration()
		}

		if cfg.Client.Grpc.MaxMsgSize > 0 {
			msgSize = int(cfg.Client.Grpc.MaxMsgSize * 1024 * 1024)
		}

		if cfg.Client.Grpc.Middleware != nil {
			if cfg.Client.Grpc.Middleware.GetEnableRecovery() {
				ms = append(ms, recovery.Recovery())
			}
			if cfg.Client.Grpc.Middleware.GetEnableTracing() {
				ms = append(ms, tracingClient(cfg.Client.Grpc.Middleware.GetTracing()))
			}
			if cfg.Client.Grpc.Middleware.GetEnableValidate() {
				ms = append(ms, validate.Validator())
			}

			if cfg.Client.Grpc.Middleware.GetEnableMetadata() {
				ms = append(ms, metadata.Client())
			}

			if cfg.Client.Grpc.Middleware.Auth != nil && cfg.Client.Grpc.Middleware.Auth.GetKey() != "" {
				signingMethod := jwtv5.SigningMethodHS256
				switch authMethod := cfg.Client.Grpc.Middleware.Auth.GetMethod(); authMethod {
				case "HS384":
					signingMethod = jwtv5.SigningMethodHS384
				case "HS512":
					signingMethod = jwtv5.SigningMethodHS512
				}

				ms = append(ms, jwt.Client(func(token *jwtv5.Token) (interface{}, error) {
					return []byte(cfg.Client.Grpc.Middleware.Auth.GetKey()), nil
				}, jwt.WithSigningMethod(signingMethod)))
			}

			if cfg.Client.Grpc.Middleware.Metrics != nil {
				var options []kratosMetrics.Option

				if cfg.Client.Grpc.Middleware.Metrics.GetHistogram() {
					options = append(options, kratosMetrics.WithSeconds(metrics.MetricClientSeconds))
				}

				if cfg.Client.Grpc.Middleware.Metrics.GetCounter() {
					options = append(options, kratosMetrics.WithRequests(metrics.MetricClientRequests))
				}
				if len(options) > 0 {
					ms = append(ms, kratosMetrics.Client(options...))
				}
			}

		}
	}

	ms = append(ms, m...)

	// 配置健康检查，对于K8s环境中的Headless Service很有用
	healthCheckConfig := `,"healthCheckConfig":{"serviceName":""}`

	opts = append(opts, kratosGrpc.WithEndpoint(endpoint))
	opts = append(opts, kratosGrpc.WithTimeout(timeout))
	opts = append(opts, kratosGrpc.WithMiddleware(ms...))

	defaultCallOpts := []grpc.CallOption{
		grpc.MaxCallRecvMsgSize(msgSize),
		grpc.MaxCallSendMsgSize(msgSize),
	}

	// 使用round_robin负载均衡策略，适用于K8s Headless Service
	opts = append(opts, kratosGrpc.WithOptions(
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingConfig": [{"%s":{}}]%s}`,
			"round_robin", healthCheckConfig)),
		grpc.WithDefaultCallOptions(defaultCallOpts...),
	))

	if cfg.Client.Grpc.Tls != nil && cfg.Client.Grpc.Tls.Enable {
		enableTls = true
		keyFile := cfg.Client.Grpc.Tls.KeyFile

		if cfg.Client.Grpc.Tls.CaFile != "" {
			caFile = cfg.Client.Grpc.Tls.CaFile
		}

		if cfg.Client.Grpc.Tls.CertFile != "" {
			certFile = cfg.Client.Grpc.Tls.CertFile
		}

		if cfg.Client.Grpc.Tls.ServerName != "" {
			serverName = cfg.Client.Grpc.Tls.ServerName
		}

		tlsConfig, err := newTLSConfig(certFile, keyFile, caFile)
		if err != nil {
			log.Fatal(err)
		}
		tlsConfig.ServerName = serverName

		tlsConf := kratosGrpc.WithTLSConfig(tlsConfig)

		opts = append(opts, tlsConf)
		conn, connErr = kratosGrpc.Dial(
			ctx,
			opts...,
		)
	} else {
		conn, connErr = kratosGrpc.DialInsecure(
			ctx,
			opts...,
		)
	}

	if connErr != nil {
		log.Fatalf("dial grpc client [%s] failed: %s", serviceName, connErr.Error())
	}
	log.Infof("endpoint: %s, initialize %s, tls: %t, client success", endpoint, serviceName, enableTls)
	return conn
}

// CreateGrpcServer 创建GRPC服务端
func CreateGrpcServer(cfg *conf.Bootstrap, logger log.Logger, m ...middleware.Middleware) *kratosGrpc.Server {
	var opts []kratosGrpc.ServerOption

	var ms []middleware.Middleware
	bTls := false
	caFile := defaultCaFile
	certFile := defaultServerCertFile
	msgSize := defaultMsgSize

	if cfg.Server != nil && cfg.Server.Grpc != nil && cfg.Server.Grpc.Middleware != nil {
		if cfg.Server.Grpc.Middleware.GetEnableRecovery() {
			ms = append(ms, recovery.Recovery())
		}
		if cfg.Server.Grpc.Middleware.GetEnableTracing() {
			ms = append(ms, tracingServer(cfg.Server.Grpc.Middleware.GetTracing()))
		}
		if cfg.Server.Grpc.Middleware.GetEnableValidate() {
			ms = append(ms, validate.Validator())
		}
		if cfg.Server.Grpc.Middleware.GetEnableMetadata() {
			ms = append(ms, metadata.Server())
		}

		if cfg.Server.Grpc.Middleware.GetEnableCircuitBreaker() {
		}
		if cfg.Server.Grpc.Middleware.Limiter != nil {
			var limiter ratelimit.Limiter
			switch cfg.Server.Grpc.Middleware.Limiter.GetName() {
			case "bbr":
				limiter = bbr.NewLimiter()
			}
			ms = append(ms, midRateLimit.Server(midRateLimit.WithLimiter(limiter)))
		}
		if cfg.Server.Grpc.Middleware.Auth != nil && cfg.Server.Grpc.Middleware.Auth.GetKey() != "" {
			signingMethod := jwtv5.SigningMethodHS256
			switch authMethod := cfg.Server.Grpc.Middleware.Auth.GetMethod(); authMethod {
			case "HS384":
				signingMethod = jwtv5.SigningMethodHS384
			case "HS512":
				signingMethod = jwtv5.SigningMethodHS512
			}

			ms = append(ms, jwt.Server(func(token *jwtv5.Token) (interface{}, error) {
				return []byte(cfg.Server.Grpc.Middleware.Auth.GetKey()), nil
			}, jwt.WithSigningMethod(signingMethod)))
		}

		if cfg.Server.Grpc.Middleware.GetEnableLogging() {
			ms = append(ms, logging.Server(logger))
		}

		if cfg.Server.Grpc.Middleware.Metrics != nil {
			var options []kratosMetrics.Option

			if cfg.Server.Grpc.Middleware.Metrics.GetHistogram() {
				options = append(options, kratosMetrics.WithSeconds(metrics.MetricServerSeconds))
			}

			if cfg.Server.Grpc.Middleware.Metrics.GetCounter() {
				options = append(options, kratosMetrics.WithRequests(metrics.MetricServerRequests))
			}
			if len(options) > 0 {
				ms = append(ms, kratosMetrics.Server(options...))
			}
		}

	}
	ms = append(ms, m...)
	opts = append(opts, kratosGrpc.Middleware(ms...))

	if cfg.Server.Grpc.Network != "" {
		opts = append(opts, kratosGrpc.Network(cfg.Server.Grpc.Network))
	}
	if cfg.Server.Grpc.Addr != "" {
		opts = append(opts, kratosGrpc.Address(cfg.Server.Grpc.Addr))
	}
	if cfg.Server.Grpc.Timeout != nil {
		opts = append(opts, kratosGrpc.Timeout(cfg.Server.Grpc.Timeout.AsDuration()))
	}

	if cfg.Server.Grpc.MaxMsgSize > 0 {
		msgSize = int(cfg.Server.Grpc.MaxMsgSize * 1024 * 1024)
	}

	grpcServerOpts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(msgSize),
		grpc.MaxSendMsgSize(msgSize),
	}
	opts = append(opts, kratosGrpc.Options(grpcServerOpts...))

	if cfg.Server.Grpc.Tls != nil && cfg.Server.Grpc.Tls.Enable {
		bTls = true

		keyFile := cfg.Server.Grpc.Tls.KeyFile

		if cfg.Server.Grpc.Tls.CaFile != "" {
			caFile = cfg.Server.Grpc.Tls.CaFile
		}

		if cfg.Server.Grpc.Tls.CertFile != "" {
			certFile = cfg.Server.Grpc.Tls.CertFile
		}

		tlsConfig, err := newTLSConfig(certFile, keyFile, caFile)
		if err != nil {
			log.Fatal(err)
		}

		tlsConf := kratosGrpc.TLSConfig(tlsConfig)

		opts = append(opts, tlsConf)
	}

	log.Infof("[gRPC] server listening on %s, TLS: %t", cfg.Server.Grpc.Addr, bTls)

	srv := kratosGrpc.NewServer(opts...)

	return srv
}

func newTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %v", err)
	}

	certPool := x509.NewCertPool()
	ca, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %v", err)
	}
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		return nil, fmt.Errorf("failed to append certs: %v", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		RootCAs:      certPool,
		ClientCAs:    certPool,
	}, nil
}
