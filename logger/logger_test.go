package logger

import (
	"io"
	"os"
	"strings"
	"testing"

	conf "github.com/alec404/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/go-kratos/kratos/v2/log"
)

func TestNewLoggerProviderIncludesAppIdentity(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	defer reader.Close()

	stdout := os.Stdout
	os.Stdout = writer
	logger := NewLoggerProvider(nil, &conf.AppInfo{
		Project:    "kratos-layout",
		AppId:      "core",
		Version:    "v1.4.0",
		Hostname:   "core-pod-1",
		InstanceId: "core-instance-1",
	})
	os.Stdout = stdout

	if err = logger.Log(log.LevelInfo, "msg", "test"); err != nil {
		t.Fatalf("write log: %v", err)
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close stdout pipe: %v", err)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	output := string(data)
	for _, field := range []string{
		"service.name=kratos-layout-core",
		"service.version=v1.4.0",
		"service.instance.id=core-instance-1",
	} {
		if !strings.Contains(output, field) {
			t.Fatalf("log output %q does not contain %q", output, field)
		}
	}
	for _, field := range []string{
		"project.name=",
		"service.id=",
		"host.name=",
		"service.namespace=",
	} {
		if strings.Contains(output, field) {
			t.Fatalf("log output %q unexpectedly contains %q", output, field)
		}
	}
	for _, field := range []string{"trace_id=", "span_id="} {
		if !strings.Contains(output, field) {
			t.Fatalf("log output %q does not contain %q", output, field)
		}
	}
}

func TestNewLoggerProviderOmitsInvalidAppIdentity(t *testing.T) {
	for _, appInfo := range []*conf.AppInfo{nil, {Project: "kratos-layout"}} {
		reader, writer, err := os.Pipe()
		if err != nil {
			t.Fatalf("create stdout pipe: %v", err)
		}

		stdout := os.Stdout
		os.Stdout = writer
		logger := NewLoggerProvider(nil, appInfo)
		os.Stdout = stdout

		if err = logger.Log(log.LevelInfo, "msg", "test"); err != nil {
			t.Fatalf("write log: %v", err)
		}
		if err = writer.Close(); err != nil {
			t.Fatalf("close stdout pipe: %v", err)
		}

		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		if err = reader.Close(); err != nil {
			t.Fatalf("close stdout reader: %v", err)
		}

		output := string(data)
		for _, field := range []string{"service.name=", "service.version=", "service.instance.id=", "project.name=", "service.id=", "host.name="} {
			if strings.Contains(output, field) {
				t.Fatalf("log output %q unexpectedly contains %q", output, field)
			}
		}
	}
}
