package zap

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

var _ log.Logger = (*Logger)(nil)

type Logger struct {
	log *zap.Logger
}

// getLogWriter 设置日志文件的路径和轮转相关属性。
func getLogWriter(cfg *Config) zapcore.WriteSyncer {
	lumberjackLogger := &lumberjack.Logger{
		Filename:   cfg.FileName,   // 日志文件的位置
		MaxSize:    cfg.MaxSize,    // 日志文件的最大大小（MB）
		MaxBackups: cfg.MaxBackups, // 最多保留多少个备份
		MaxAge:     cfg.MaxAge,     // 文件最多保存多少天
		Compress:   cfg.Compress,   // 是否压缩
	}
	return zapcore.AddSync(lumberjackLogger)
}

// getErrorLogWriter 设置日志文件的路径和轮转相关属性。
func getErrorLogWriter(cfg *Config) zapcore.WriteSyncer {
	lumberjackLogger := &lumberjack.Logger{
		Filename:   cfg.ErrorFileName, // 日志文件的位置
		MaxSize:    cfg.MaxSize,       // 日志文件的最大大小（MB）
		MaxBackups: cfg.MaxBackups,    // 最多保留多少个备份
		MaxAge:     cfg.MaxAge,        // 文件最多保存多少天
		Compress:   cfg.Compress,      // 是否压缩
	}
	return zapcore.AddSync(lumberjackLogger)
}

// getEncoder 设置日志的输出格式。
func getEncoderConfig() zapcore.EncoderConfig {
	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		NameKey:        "logger",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}

	return encoderCfg
}

func (l Logger) Log(level log.Level, keyvals ...interface{}) error {
	keylen := len(keyvals)
	if keylen == 0 || keylen%2 != 0 {
		l.log.Warn(fmt.Sprint("Keyvalues must appear in pairs: ", keyvals))
		return nil
	}

	data := make([]zap.Field, 0, (keylen/2)+1)
	for i := 0; i < keylen; i += 2 {
		data = append(data, zap.Any(fmt.Sprint(keyvals[i]), keyvals[i+1]))
	}

	switch level {
	case log.LevelDebug:
		l.log.Debug("", data...)
	case log.LevelInfo:
		l.log.Info("", data...)
	case log.LevelWarn:
		l.log.Warn("", data...)
	case log.LevelError:
		l.log.Error("", data...)
	case log.LevelFatal:
		l.log.Fatal("", data...)
	}
	return nil
}

// 创建一个zap logger
func NewZapLogger(opts ...Option) *zap.Logger {

	cfg := newConfigByOpts(opts...)

	// 设置日志级别
	atomicLevel := zap.NewAtomicLevelAt(cfg.Level)

	var cores []zapcore.Core

	if cfg.EnableConsole {
		cores = append(cores, zapcore.NewCore(zapcore.NewConsoleEncoder(getEncoderConfig()), zapcore.AddSync(zapcore.Lock(os.Stdout)), atomicLevel))
	}

	if cfg.EnableFile {
		cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(getEncoderConfig()), zapcore.AddSync(getLogWriter(cfg)), atomicLevel))
	}

	if cfg.EnableErrorFile {
		cores = append(cores, zapcore.NewCore(zapcore.NewJSONEncoder(getEncoderConfig()), zapcore.AddSync(getErrorLogWriter(cfg)), zapcore.ErrorLevel))
	}

	core := zapcore.NewTee(cores...)
	// 添加调用者信息
	caller := zap.AddCaller()

	// 添加堆栈跟踪
	stacktrace := zap.AddStacktrace(zap.ErrorLevel)

	// 构建日志
	logger := zap.New(core, caller, stacktrace)
	return logger
}

func NewLogger(zlog *zap.Logger) *Logger {
	return &Logger{zlog}
}

func (l *Logger) Sync() error {
	return l.log.Sync()
}

func (l *Logger) Close() error {
	return l.Sync()
}
