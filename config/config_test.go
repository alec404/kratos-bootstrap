package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
)

func TestNewConfigProviderParsesGRPCKeepalive(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "transport.yaml")
	configYAML := []byte(`server:
  grpc:
    keepalive_enforcement_policy:
      enable: true
      min_time: 30s
      permit_without_stream: true
client:
  grpc:
    keepalive:
      enable: true
      time: 60s
      timeout: 10s
      permit_without_stream: true
`)
	if err := os.WriteFile(configPath, configYAML, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	provider := NewConfigProvider(configPath)
	t.Cleanup(func() {
		if err := provider.Close(); err != nil {
			t.Errorf("close config provider: %v", err)
		}
	})
	if err := provider.Load(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	var got conf.Bootstrap
	if err := provider.Scan(&got); err != nil {
		t.Fatalf("scan config: %v", err)
	}

	clientKeepalive := got.GetClient().GetGrpc().GetKeepalive()
	if clientKeepalive == nil {
		t.Fatal("client gRPC keepalive config is nil")
	}
	if !clientKeepalive.GetEnable() {
		t.Fatal("client gRPC keepalive should be enabled")
	}
	if clientKeepalive.GetTime() == nil {
		t.Fatal("client keepalive time is nil")
	}
	if duration := clientKeepalive.GetTime().AsDuration(); duration != time.Minute {
		t.Fatalf("client keepalive time = %s, want %s", duration, time.Minute)
	}
	if clientKeepalive.GetTimeout() == nil {
		t.Fatal("client keepalive timeout is nil")
	}
	if duration := clientKeepalive.GetTimeout().AsDuration(); duration != 10*time.Second {
		t.Fatalf("client keepalive timeout = %s, want %s", duration, 10*time.Second)
	}
	if !clientKeepalive.GetPermitWithoutStream() {
		t.Fatal("client keepalive should permit pings without streams")
	}

	policy := got.GetServer().GetGrpc().GetKeepaliveEnforcementPolicy()
	if policy == nil {
		t.Fatal("server gRPC keepalive enforcement policy is nil")
	}
	if !policy.GetEnable() {
		t.Fatal("server gRPC keepalive enforcement policy should be enabled")
	}
	if policy.GetMinTime() == nil {
		t.Fatal("server keepalive minimum time is nil")
	}
	if duration := policy.GetMinTime().AsDuration(); duration != 30*time.Second {
		t.Fatalf("server keepalive minimum time = %s, want %s", duration, 30*time.Second)
	}
	if !policy.GetPermitWithoutStream() {
		t.Fatal("server keepalive enforcement policy should permit pings without streams")
	}
}

func TestLoadBootstrapConfigReturnsFreshConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "transport.yaml")
	if err := os.WriteFile(configPath, []byte("server:\n  http:\n    addr: 127.0.0.1:8080\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	first, err := LoadBootstrapConfig(configPath)
	if err != nil {
		t.Fatalf("load first bootstrap config: %v", err)
	}
	first.Server.Http.Addr = "changed"

	second, err := LoadBootstrapConfig(configPath)
	if err != nil {
		t.Fatalf("load second bootstrap config: %v", err)
	}
	if first == second {
		t.Fatal("LoadBootstrapConfig returned a shared Bootstrap instance")
	}
	if got := second.GetServer().GetHttp().GetAddr(); got != "127.0.0.1:8080" {
		t.Fatalf("second HTTP addr = %q, want %q", got, "127.0.0.1:8080")
	}
}
