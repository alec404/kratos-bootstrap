package zap

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

type Config struct {
	EnableConsole   bool   // 是否启用控制台输出
	EnableFile      bool   // 是否启用文件输出
	EnableErrorFile bool   // 是否启用错误文件输出
	FileName        string // 日志文件名
	ErrorFileName   string // 错误日志文件名
	Level           zapcore.Level
	MaxSize         int  // 日志文件的最大大小（MB）
	MaxBackups      int  // 日志文件数量
	MaxAge          int  // 文件最多保存多少天
	Compress        bool // 是否压缩
}

func newConfigByOpts(opts ...Option) *Config {

	defaultLevel := zap.InfoLevel
	if os.Getenv("GO_ENV") != "production" {
		defaultLevel = zap.DebugLevel
	}

	cfg := &Config{
		EnableConsole:   true,               // 是否启用控制台输出
		EnableFile:      true,               // 是否启用文件输出
		EnableErrorFile: true,               // 是否启用错误文件输出
		FileName:        "./logs/myapp.log", // 日志文件名
		ErrorFileName:   "./logs/error.log", // 日志文件名
		Level:           defaultLevel,
		MaxSize:         100,  // 日志文件的最大大小（MB）
		MaxBackups:      10,   // 日志文件数量
		MaxAge:          3,    // 文件最多保存多少天
		Compress:        true, // 是否压缩
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// Option represents the optional function.
type Option func(c *Config)

func WithEnableConsole(enable bool) Option {
	return func(c *Config) {
		c.EnableConsole = enable
	}
}

func WithEnableFile(enable bool) Option {
	return func(c *Config) {
		c.EnableFile = enable
	}
}

func WithFileName(filename string) Option {
	return func(c *Config) {
		c.FileName = filename
	}
}

func WithLevel(level zapcore.Level) Option {
	return func(c *Config) {
		c.Level = level
	}
}

func WithMaxSize(maxSize int) Option {
	return func(c *Config) {
		c.MaxSize = maxSize
	}
}

func WithMaxBackups(maxBackups int) Option {
	return func(c *Config) {
		c.MaxBackups = maxBackups
	}
}

func WithMaxAge(maxAge int) Option {
	return func(c *Config) {
		c.MaxAge = maxAge
	}
}

func WithCompress(compress bool) Option {
	return func(c *Config) {
		c.Compress = compress
	}
}

func WithEnableErrorFile(enable bool) Option {
	return func(c *Config) {
		c.EnableErrorFile = enable
	}
}

func WithErrorFileName(filename string) Option {
	return func(c *Config) {
		c.ErrorFileName = filename
	}
}
