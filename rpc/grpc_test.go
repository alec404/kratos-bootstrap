package rpc

import (
	"strings"
	"testing"
	"time"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestClientKeepaliveParameters(t *testing.T) {
	validConfig := func() *conf.Client_Keepalive {
		return &conf.Client_Keepalive{
			Enable:              true,
			Time:                durationpb.New(time.Minute),
			Timeout:             durationpb.New(10 * time.Second),
			PermitWithoutStream: true,
		}
	}

	tests := []struct {
		name        string
		config      *conf.Client_Keepalive
		want        keepalive.ClientParameters
		wantEnabled bool
		wantErr     string
	}{
		{name: "missing config remains disabled"},
		{
			name: "disabled config ignores durations",
			config: &conf.Client_Keepalive{
				Time:    durationpb.New(-time.Second),
				Timeout: durationpb.New(0),
			},
		},
		{
			name:        "enabled config maps parameters",
			config:      validConfig(),
			wantEnabled: true,
			want: keepalive.ClientParameters{
				Time:                time.Minute,
				Timeout:             10 * time.Second,
				PermitWithoutStream: true,
			},
		},
		{
			name: "enabled config requires time",
			config: &conf.Client_Keepalive{
				Enable:  true,
				Timeout: durationpb.New(10 * time.Second),
			},
			wantErr: "client.grpc.keepalive.time is required",
		},
		{
			name: "enabled config rejects short time",
			config: &conf.Client_Keepalive{
				Enable:  true,
				Time:    durationpb.New(9 * time.Second),
				Timeout: durationpb.New(10 * time.Second),
			},
			wantErr: "client.grpc.keepalive.time must be at least 10s",
		},
		{
			name: "enabled config rejects invalid time",
			config: &conf.Client_Keepalive{
				Enable:  true,
				Time:    &durationpb.Duration{Seconds: 315576000001},
				Timeout: durationpb.New(10 * time.Second),
			},
			wantErr: "client.grpc.keepalive.time is invalid",
		},
		{
			name: "enabled config requires timeout",
			config: &conf.Client_Keepalive{
				Enable: true,
				Time:   durationpb.New(time.Minute),
			},
			wantErr: "client.grpc.keepalive.timeout is required",
		},
		{
			name: "enabled config rejects non-positive timeout",
			config: &conf.Client_Keepalive{
				Enable:  true,
				Time:    durationpb.New(time.Minute),
				Timeout: durationpb.New(0),
			},
			wantErr: "client.grpc.keepalive.timeout must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, enabled, err := clientKeepaliveParameters(tt.config)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("clientKeepaliveParameters() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("clientKeepaliveParameters() unexpected error: %v", err)
			}
			if enabled != tt.wantEnabled {
				t.Fatalf("clientKeepaliveParameters() enabled = %t, want %t", enabled, tt.wantEnabled)
			}
			if got != tt.want {
				t.Fatalf("clientKeepaliveParameters() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestServerKeepaliveEnforcementPolicy(t *testing.T) {
	tests := []struct {
		name        string
		config      *conf.Server_KeepaliveEnforcementPolicy
		want        keepalive.EnforcementPolicy
		wantEnabled bool
		wantErr     string
	}{
		{name: "missing config remains disabled"},
		{
			name: "disabled config ignores min time",
			config: &conf.Server_KeepaliveEnforcementPolicy{
				MinTime: durationpb.New(-time.Second),
			},
		},
		{
			name: "enabled config maps policy",
			config: &conf.Server_KeepaliveEnforcementPolicy{
				Enable:              true,
				MinTime:             durationpb.New(30 * time.Second),
				PermitWithoutStream: true,
			},
			wantEnabled: true,
			want: keepalive.EnforcementPolicy{
				MinTime:             30 * time.Second,
				PermitWithoutStream: true,
			},
		},
		{
			name: "enabled config requires min time",
			config: &conf.Server_KeepaliveEnforcementPolicy{
				Enable: true,
			},
			wantErr: "server.grpc.keepalive_enforcement_policy.min_time is required",
		},
		{
			name: "enabled config rejects non-positive min time",
			config: &conf.Server_KeepaliveEnforcementPolicy{
				Enable:  true,
				MinTime: durationpb.New(0),
			},
			wantErr: "server.grpc.keepalive_enforcement_policy.min_time must be greater than zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, enabled, err := serverKeepaliveEnforcementPolicy(tt.config)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("serverKeepaliveEnforcementPolicy() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("serverKeepaliveEnforcementPolicy() unexpected error: %v", err)
			}
			if enabled != tt.wantEnabled {
				t.Fatalf("serverKeepaliveEnforcementPolicy() enabled = %t, want %t", enabled, tt.wantEnabled)
			}
			if got != tt.want {
				t.Fatalf("serverKeepaliveEnforcementPolicy() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGRPCTransportOptionsKeepBaselineOptions(t *testing.T) {
	clientConfig := &conf.Client_GRPC{}
	clientOpts, err := grpcClientTransportOptions(clientConfig, defaultMsgSize)
	if err != nil {
		t.Fatalf("grpcClientTransportOptions() disabled error: %v", err)
	}
	disabledClientOptionCount := len(clientOpts)
	if disabledClientOptionCount == 0 {
		t.Fatal("disabled client config unexpectedly removed all baseline options")
	}

	clientConfig.Keepalive = &conf.Client_Keepalive{
		Enable:              true,
		Time:                durationpb.New(time.Minute),
		Timeout:             durationpb.New(10 * time.Second),
		PermitWithoutStream: true,
	}
	clientOpts, err = grpcClientTransportOptions(clientConfig, defaultMsgSize)
	if err != nil {
		t.Fatalf("grpcClientTransportOptions() enabled error: %v", err)
	}
	if got, want := len(clientOpts), disabledClientOptionCount+1; got != want {
		t.Fatalf("enabled client option count = %d, want %d", got, want)
	}

	serverConfig := &conf.Server_GRPC{}
	serverOpts, err := grpcServerTransportOptions(serverConfig, defaultMsgSize)
	if err != nil {
		t.Fatalf("grpcServerTransportOptions() disabled error: %v", err)
	}
	disabledServerOptionCount := len(serverOpts)
	if disabledServerOptionCount == 0 {
		t.Fatal("disabled server config unexpectedly removed all baseline options")
	}

	serverConfig.KeepaliveEnforcementPolicy = &conf.Server_KeepaliveEnforcementPolicy{
		Enable:              true,
		MinTime:             durationpb.New(30 * time.Second),
		PermitWithoutStream: true,
	}
	serverOpts, err = grpcServerTransportOptions(serverConfig, defaultMsgSize)
	if err != nil {
		t.Fatalf("grpcServerTransportOptions() enabled error: %v", err)
	}
	if got, want := len(serverOpts), disabledServerOptionCount+1; got != want {
		t.Fatalf("enabled server option count = %d, want %d", got, want)
	}
}
