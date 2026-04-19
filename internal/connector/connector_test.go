package connector

import (
	"testing"

	"github.com/anuragvishwa/pushsite/internal/connection"
)

func TestNewSSH(t *testing.T) {
	cfg := &connection.Config{
		Host:    "1.2.3.4",
		User:    "ubuntu",
		KeyPath: "~/.ssh/id_rsa",
		Method:  "ssh",
	}

	conn, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if conn == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewSSM(t *testing.T) {
	cfg := &connection.Config{
		Method:     "ssm",
		InstanceID: "i-12345",
		Region:     "us-east-1",
	}

	conn, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if conn == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewUnknownMethod(t *testing.T) {
	cfg := &connection.Config{
		Method: "telnet",
	}

	_, err := New(cfg)
	if err == nil {
		t.Fatal("New() should error for unknown method")
	}
}

func TestFromServerConfig(t *testing.T) {
	cfg := FromServerConfig("1.2.3.4", "ubuntu", "~/.ssh/key", "ssh", "", 22)

	if cfg.Host != "1.2.3.4" {
		t.Errorf("expected host '1.2.3.4', got '%s'", cfg.Host)
	}
	if cfg.User != "ubuntu" {
		t.Errorf("expected user 'ubuntu', got '%s'", cfg.User)
	}
	if cfg.Method != "ssh" {
		t.Errorf("expected method 'ssh', got '%s'", cfg.Method)
	}
	if cfg.Port != 22 {
		t.Errorf("expected port 22, got %d", cfg.Port)
	}
}

func TestFromServerConfigSSM(t *testing.T) {
	cfg := FromServerConfig("", "", "", "ssm", "i-12345", 0)

	if cfg.Method != "ssm" {
		t.Errorf("expected method 'ssm', got '%s'", cfg.Method)
	}
	if cfg.InstanceID != "i-12345" {
		t.Errorf("expected instance id 'i-12345', got '%s'", cfg.InstanceID)
	}
}
