package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"time"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/alec404/kratos-bootstrap/config"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"
)

var (
	ErrInvalidCustomConfig      = errors.New("invalid custom config")
	ErrDuplicateCustomConfig    = errors.New("duplicate custom config")
	ErrCustomConfigNotFound     = errors.New("custom config not found")
	ErrCustomConfigTypeMismatch = errors.New("custom config type mismatch")
)

// InitAppWithContext 使用启动 Context 构造应用。
type InitAppWithContext func(ctx *Context) (*kratos.App, func(), error)

// Context 保存一次应用启动所需的信息和配置。
type Context struct {
	appInfo       *conf.AppInfo
	customConfigs map[string]proto.Message
	config        *conf.Bootstrap
	logger        log.Logger
}

// NewContext 使用应用信息创建启动 Context。
func NewContext(info *conf.AppInfo) *Context {
	return &Context{
		appInfo:       config.NewAppInfo(info),
		customConfigs: make(map[string]proto.Message),
	}
}

// RegisterCustomConfig 注册一个由 Bootstrap 配置源填充的 Proto 配置。
// 注册必须在 Run 之前完成。
func (c *Context) RegisterCustomConfig(key string, target proto.Message) {
	if c == nil {
		panic("bootstrap context is nil")
	}
	if key == "" {
		panic(fmt.Errorf("%w: key is empty", ErrInvalidCustomConfig))
	}
	if err := validateCustomConfigTarget(target); err != nil {
		panic(fmt.Errorf("%w for key %q: %v", ErrInvalidCustomConfig, key, err))
	}
	if c.config != nil {
		panic(fmt.Errorf("%w for key %q: registration must happen before Run", ErrInvalidCustomConfig, key))
	}
	if c.customConfigs == nil {
		c.customConfigs = make(map[string]proto.Message)
	}
	if _, exists := c.customConfigs[key]; exists {
		panic(fmt.Errorf("%w: %q", ErrDuplicateCustomConfig, key))
	}

	c.customConfigs[key] = target
}

// GetCustomConfig 获取并校验 Context 中的自定义 Proto 配置类型。
func GetCustomConfig[T proto.Message](c *Context, key string) (T, error) {
	var zero T
	if c == nil {
		return zero, fmt.Errorf("%w: context is nil", ErrCustomConfigNotFound)
	}
	value, ok := c.customConfigs[key]
	if !ok {
		return zero, fmt.Errorf("%w: %q", ErrCustomConfigNotFound, key)
	}
	typed, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf(
			"%w for key %q: stored %T",
			ErrCustomConfigTypeMismatch,
			key,
			value,
		)
	}
	return typed, nil
}

func validateCustomConfigTarget(target proto.Message) error {
	if target == nil {
		return errors.New("target is nil")
	}
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer, got %T", target)
	}
	return nil
}

func (c *Context) customConfigTargets() []proto.Message {
	if c == nil || len(c.customConfigs) == 0 {
		return nil
	}

	keys := make([]string, 0, len(c.customConfigs))
	for key := range c.customConfigs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	targets := make([]proto.Message, 0, len(keys))
	for _, key := range keys {
		targets = append(targets, c.customConfigs[key])
	}
	return targets
}

// Run 加载配置、构造应用并启动。
func (c *Context) Run(initApp InitAppWithContext) {
	if c == nil {
		panic("bootstrap context is nil")
	}

	// 先输出一次不依赖配置的应用身份，确保配置加载或 Logger 初始化失败时
	// 仍能确认当前进程对应的服务和运行环境。
	c.PrintAppInfo()

	c.config, c.logger = DoBootstrap(c.appInfo, c.customConfigTargets()...)

	app, cleanup, err := initApp(c)
	if err != nil {
		panic(err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	if err = app.Run(); err != nil {
		panic(err)
	}
}

// PrintAppInfo 在配置加载前直接输出一次应用身份信息。
// 该输出不依赖 Bootstrap 配置或业务 Logger，即使 DoBootstrap 失败也可见。
func (c *Context) PrintAppInfo() {
	if c == nil || !config.IsValidAppInfo(c.appInfo) {
		return
	}

	ai := c.appInfo
	host := ai.GetHostname()
	if host == "" {
		host = config.ResolveHost()
	}

	fmt.Printf("[%s] %s (pid:%d@%s)\n", time.Now().Format(time.RFC3339), config.ServiceName(ai), os.Getpid(), host)
	fmt.Printf("  Project: %s\n", ai.GetProject())
	fmt.Printf("  AppId: %s\n", ai.GetAppId())
	fmt.Printf("  Version: %s\n", ai.GetVersion())
	fmt.Printf("  InstanceId: %s\n", ai.GetInstanceId())
}

// AppInfoFromContext 为 Wire 提供标准化后的应用身份信息。
func AppInfoFromContext(c *Context) *conf.AppInfo {
	if c == nil || c.appInfo == nil {
		return nil
	}
	return config.CloneAppInfo(c.appInfo)
}

// LoggerFromContext 为 Wire 提供启动时创建的 Logger。
func LoggerFromContext(c *Context) log.Logger {
	if c == nil {
		return nil
	}
	return c.logger
}

// ConfigFromContext 为 Wire 提供已加载的 Bootstrap 配置。
func ConfigFromContext(c *Context) *conf.Bootstrap {
	if c == nil {
		return nil
	}
	return c.config
}
