package config

import (
	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/env"
	fileKratos "github.com/go-kratos/kratos/v2/config/file"
	"google.golang.org/protobuf/proto"
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

// LoadBootstrapConfig 加载程序引导配置和本次启动显式注册的自定义配置。
func LoadBootstrapConfig(configPath string, customConfigs ...proto.Message) (*conf.Bootstrap, error) {
	cfg := NewConfigProvider(configPath)
	if err := cfg.Load(); err != nil {
		return nil, err
	}

	bootstrapConfig := &conf.Bootstrap{}
	targets := make([]proto.Message, 0, len(customConfigs)+1)
	targets = append(targets, bootstrapConfig)
	targets = append(targets, customConfigs...)
	if err := scanConfigs(cfg, targets); err != nil {
		return nil, err
	}

	return bootstrapConfig, nil
}

func scanConfigs(cfg config.Config, targets []proto.Message) error {
	for _, target := range targets {
		if err := cfg.Scan(target); err != nil {
			return err
		}
	}
	return nil
}
