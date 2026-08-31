package config

import (
	"os"
	"strings"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"google.golang.org/protobuf/proto"
)

const DefaultAppVersion = "1.0.0"

// NewAppInfo 创建已补齐默认值且不共享可变字段的 AppInfo。
func NewAppInfo(info *conf.AppInfo) *conf.AppInfo {
	return NormalizeAppInfo(info)
}

// NormalizeAppInfo 克隆 AppInfo 并补齐运行时默认值。
// ServiceName 未提供时由 project 和 app_id 拼接生成；显式 ServiceName 会被保留。
func NormalizeAppInfo(info *conf.AppInfo) *conf.AppInfo {
	normalized := CloneAppInfo(info)
	if normalized == nil {
		normalized = &conf.AppInfo{}
	}

	if normalized.GetVersion() == "" {
		normalized.Version = DefaultAppVersion
	}
	if normalized.GetHostname() == "" {
		normalized.Hostname = ResolveHost()
	}
	if normalized.Metadata == nil {
		normalized.Metadata = make(map[string]string)
	}
	if normalized.GetServiceName() == "" {
		normalized.ServiceName = serviceName(normalized.GetProject(), normalized.GetAppId())
	}

	if normalized.GetInstanceId() == "" && normalized.GetAppId() != "" {
		normalized.InstanceId = normalized.GetHostname()
	}

	return normalized
}

// ResolveHost 返回当前运行环境的主机名。
//
// 容器环境优先使用显式注入的 Pod/hostname 环境变量；未注入时回退到
// 操作系统 hostname。该值同时作为默认的 instance ID。
func ResolveHost() string {
	for _, key := range []string{"POD_NAME", "HOSTNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}

	if hostname, err := os.Hostname(); err == nil {
		if hostname = strings.TrimSpace(hostname); hostname != "" {
			return hostname
		}
	}

	return "unknown-host"
}

// IsValidAppInfo 判断 AppInfo 是否包含可识别的应用标识。
func IsValidAppInfo(info *conf.AppInfo) bool {
	return info != nil && info.GetAppId() != ""
}

// ServiceName 返回用于 Kratos 服务注册和发现的完整服务名。
// 已标准化的 AppInfo 直接使用 ServiceName；未标准化的值按
// <project>-<app_id> 派生。
func ServiceName(info *conf.AppInfo) string {
	if info == nil {
		return ""
	}
	if info.GetServiceName() != "" {
		return info.GetServiceName()
	}
	return serviceName(info.GetProject(), info.GetAppId())
}

// serviceName 派生项目内应用的完整服务名称。
func serviceName(project, appID string) string {
	switch {
	case project == "":
		return appID
	case appID == "":
		return project
	default:
		return project + "-" + appID
	}
}

// CloneAppInfo 返回 AppInfo 的深拷贝。
func CloneAppInfo(info *conf.AppInfo) *conf.AppInfo {
	if info == nil {
		return nil
	}

	cloned, ok := proto.Clone(info).(*conf.AppInfo)
	if !ok {
		return nil
	}
	return cloned
}
