package logger

import (
	"os"

	"go.uber.org/zap/zapcore"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"

	"github.com/alec404/kratos-bootstrap/config"
	"github.com/alec404/kratos-bootstrap/logger/zap"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
)

// NewLogger 创建一个新的日志记录器
func NewLogger(cfg *conf.Logger) log.Logger {
	if cfg == nil {
		return NewStdLogger()
	}

	switch Type(cfg.Type) {
	default:
		fallthrough
	case Std:
		return NewStdLogger()
	case Zap:
		var zapOptions []zap.Option
		if cfg.Zap != nil {

			if !cfg.Zap.EnableConsole { // 默认开启
				zapOptions = append(zapOptions, zap.WithEnableConsole(cfg.Zap.EnableConsole))
			}
			if !cfg.Zap.EnableFile { // 默认开启
				zapOptions = append(zapOptions, zap.WithEnableFile(cfg.Zap.EnableFile))
			}
			if !cfg.Zap.EnableErrorFile { // 默认开启
				zapOptions = append(zapOptions, zap.WithEnableErrorFile(cfg.Zap.EnableErrorFile))
			}
			if !cfg.Zap.EnableCompress { // 默认开启
				zapOptions = append(zapOptions, zap.WithCompress(cfg.Zap.EnableCompress))
			}

			if cfg.Zap.Filename != "" {
				zapOptions = append(zapOptions, zap.WithFileName(cfg.Zap.Filename))
			}
			if cfg.Zap.ErrorFilename != "" {
				zapOptions = append(zapOptions, zap.WithErrorFileName(cfg.Zap.ErrorFilename))
			}
			if cfg.Zap.Level != "" {
				var lvl = new(zapcore.Level)
				if err := lvl.UnmarshalText([]byte(cfg.Zap.Level)); err != nil {
					return nil
				}
				zapOptions = append(zapOptions, zap.WithLevel(*lvl))
			}
			if cfg.Zap.MaxSize > 0 {
				zapOptions = append(zapOptions, zap.WithMaxSize(int(cfg.Zap.MaxSize)))
			}
			if cfg.Zap.MaxAge > 0 {
				zapOptions = append(zapOptions, zap.WithMaxAge(int(cfg.Zap.MaxAge)))
			}
			if cfg.Zap.MaxBackups > 0 {
				zapOptions = append(zapOptions, zap.WithMaxBackups(int(cfg.Zap.MaxBackups)))
			}
		}

		return zap.NewLogger(zap.NewZapLogger(zapOptions...))
	}
}

// NewLoggerProvider 创建一个新的日志记录器提供者。
// 仅当 AppInfo 包含有效的 AppId 时追加应用身份字段。
func NewLoggerProvider(cfg *conf.Logger, appInfo *conf.AppInfo) log.Logger {
	l := NewLogger(cfg)
	fields := []any{
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
	}

	if config.IsValidAppInfo(appInfo) {
		identityFields := make([]any, 0, 6)
		identityFields = appendStringField(identityFields, "service.name", config.ServiceName(appInfo))
		identityFields = appendStringField(identityFields, "service.version", appInfo.GetVersion())
		identityFields = appendStringField(identityFields, "service.instance.id", appInfo.GetInstanceId())
		fields = append(identityFields, fields...)
	}

	fields = append(fields,
		"trace_id", tracing.TraceID(),
		"span_id", tracing.SpanID(),
	)

	return log.With(l, fields...)
}

func appendStringField(fields []any, key, value string) []any {
	if value == "" {
		return fields
	}
	return append(fields, key, value)
}

// NewStdLogger 创建一个新的日志记录器 - Kratos内置，控制台输出
func NewStdLogger() log.Logger {
	l := log.NewStdLogger(os.Stdout)
	return l
}
