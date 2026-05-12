package config

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	fileKratos "github.com/go-kratos/kratos/v2/config/file"
)

// NewFileConfigSource 创建一个本地文件配置源
func NewFileConfigSource(filePath string) config.Source {
	return fileKratos.NewSource(filePath)
}

// NewConfigProvider 创建一个配置
func NewConfigProvider(configPath string) config.Config {
	return config.New(
		config.WithSource(
			env.NewSource(),
			NewFileConfigSource(configPath),
		),
	)
}

// LoadBootstrapConfig 加载程序引导配置
func LoadBootstrapConfig(configPath string) error {
	cfg := NewConfigProvider(configPath)

	var err error

	if err = cfg.Load(); err != nil {
		return err
	}

	initBootstrapConfig()

	if err = scanConfigs(cfg); err != nil {
		return err
	}

	return nil
}

func scanConfigs(cfg config.Config) error {
	initBootstrapConfig()

	for _, c := range configList {
		if err := cfg.Scan(c); err != nil {
			return err
		}
	}
	return nil
}
