package config

import (
	"testing"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"google.golang.org/protobuf/proto"
)

var _ proto.Message = (*conf.AppInfo)(nil)

func TestAppInfoServiceName(t *testing.T) {
	tests := []struct {
		name string
		info *conf.AppInfo
		want string
	}{
		{name: "project and app", info: &conf.AppInfo{Project: "layout", AppId: "core"}, want: "layout-core"},
		{name: "explicit service name", info: &conf.AppInfo{Project: "layout", AppId: "core", ServiceName: "layout-core"}, want: "layout-core"},
		{name: "app only", info: &conf.AppInfo{AppId: "core"}, want: "core"},
		{name: "project only", info: &conf.AppInfo{Project: "layout"}, want: "layout"},
		{name: "empty", info: &conf.AppInfo{}, want: ""},
		{name: "nil", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ServiceName(tt.info); got != tt.want {
				t.Fatalf("ServiceName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeAppInfo(t *testing.T) {
	metadata := map[string]string{"zone": "cn"}
	info := NormalizeAppInfo(&conf.AppInfo{
		Project:  "kratos-layout",
		AppId:    "core",
		Hostname: "core-pod-1",
		Metadata: metadata,
	})

	if info.Version != DefaultAppVersion {
		t.Fatalf("Version = %q, want %q", info.Version, DefaultAppVersion)
	}
	if info.ServiceName != "kratos-layout-core" {
		t.Fatalf("ServiceName = %q, want %q", info.ServiceName, "kratos-layout-core")
	}
	if info.InstanceId != "core-pod-1" {
		t.Fatalf("InstanceId = %q, want %q", info.InstanceId, "core-pod-1")
	}

	metadata["zone"] = "us"
	if info.Metadata["zone"] != "cn" {
		t.Fatalf("Metadata shares caller map: got %q, want %q", info.Metadata["zone"], "cn")
	}
}

func TestNormalizeAppInfoUsesResolvedHostnameAsInstanceID(t *testing.T) {
	t.Setenv("POD_NAME", "core-pod-1")

	info := NormalizeAppInfo(&conf.AppInfo{AppId: "core"})
	if info.Hostname != "core-pod-1" {
		t.Fatalf("Hostname = %q, want %q", info.Hostname, "core-pod-1")
	}
	if info.InstanceId != info.Hostname {
		t.Fatalf("InstanceId = %q, want hostname %q", info.InstanceId, info.Hostname)
	}
}

func TestNormalizeAppInfoDoesNotGenerateInstanceIDWithoutApp(t *testing.T) {
	info := NormalizeAppInfo(&conf.AppInfo{Project: "kratos-layout"})
	if info.InstanceId != "" {
		t.Fatalf("InstanceId = %q, want empty for invalid app info", info.InstanceId)
	}
}

func TestResolveHostPrefersInjectedEnvironment(t *testing.T) {
	t.Setenv("POD_NAME", "pod-from-env")
	t.Setenv("HOSTNAME", "hostname-from-env")
	if got := ResolveHost(); got != "pod-from-env" {
		t.Fatalf("ResolveHost() = %q, want %q", got, "pod-from-env")
	}

	t.Setenv("POD_NAME", " ")
	if got := ResolveHost(); got != "hostname-from-env" {
		t.Fatalf("ResolveHost() = %q, want %q", got, "hostname-from-env")
	}
}

func TestNormalizeAppInfoPreservesExplicitRuntimeIdentity(t *testing.T) {
	info := NormalizeAppInfo(&conf.AppInfo{
		Project:     "kratos-layout",
		AppId:       "core",
		ServiceName: "custom-service-name",
		Version:     "v1.4.0",
		Hostname:    "core-pod-1",
		InstanceId:  "core-instance-1",
	})

	if info.Version != "v1.4.0" {
		t.Fatalf("Version = %q, want %q", info.Version, "v1.4.0")
	}
	if info.InstanceId != "core-instance-1" {
		t.Fatalf("InstanceId = %q, want %q", info.InstanceId, "core-instance-1")
	}
	if info.ServiceName != "custom-service-name" {
		t.Fatalf("ServiceName = %q, want %q", info.ServiceName, "custom-service-name")
	}
	if info.Metadata == nil {
		t.Fatal("Metadata should be initialized")
	}
}

func TestIsValidAppInfo(t *testing.T) {
	tests := []struct {
		name string
		info *conf.AppInfo
		want bool
	}{
		{name: "nil", want: false},
		{name: "empty", info: &conf.AppInfo{}, want: false},
		{name: "project only", info: &conf.AppInfo{Project: "layout"}, want: false},
		{name: "app", info: &conf.AppInfo{AppId: "core"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidAppInfo(tt.info); got != tt.want {
				t.Fatalf("IsValidAppInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
