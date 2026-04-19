package connection

import (
	"testing"
)

func TestConfigSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()

	if cfg.User != "ubuntu" {
		t.Errorf("expected user 'ubuntu', got '%s'", cfg.User)
	}
	if cfg.Port != 22 {
		t.Errorf("expected port 22, got %d", cfg.Port)
	}
	if cfg.Method != "ssh" {
		t.Errorf("expected method 'ssh', got '%s'", cfg.Method)
	}
	if cfg.Region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got '%s'", cfg.Region)
	}
}

func TestConfigSetDefaultsPreservesValues(t *testing.T) {
	cfg := &Config{
		User:   "admin",
		Port:   2222,
		Method: "ssm",
		Region: "eu-west-1",
	}
	cfg.SetDefaults()

	if cfg.User != "admin" {
		t.Errorf("expected user 'admin', got '%s'", cfg.User)
	}
	if cfg.Port != 2222 {
		t.Errorf("expected port 2222, got %d", cfg.Port)
	}
	if cfg.Method != "ssm" {
		t.Errorf("expected method 'ssm', got '%s'", cfg.Method)
	}
	if cfg.Region != "eu-west-1" {
		t.Errorf("expected region 'eu-west-1', got '%s'", cfg.Region)
	}
}

func TestConnectionError(t *testing.T) {
	err := &ConnectionError{
		Op:      "connect",
		Host:    "example.com",
		Message: "connection refused",
	}

	expected := "connect example.com: connection refused"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

func TestConnectionErrorWithWrapped(t *testing.T) {
	inner := ErrAuthFailed
	err := &ConnectionError{
		Op:      "connect",
		Host:    "example.com",
		Message: "auth error",
		Err:     inner,
	}

	if err.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}

	contains := "connect example.com: auth error: authentication failed"
	if err.Error() != contains {
		t.Errorf("expected '%s', got '%s'", contains, err.Error())
	}
}

func TestSentinelErrors(t *testing.T) {
	errors := []error{
		ErrNotConnected,
		ErrKeyNotFound,
		ErrInvalidKey,
		ErrAuthFailed,
		ErrTimeout,
		ErrCommandFailed,
		ErrUploadFailed,
		ErrDownloadFailed,
		ErrInstanceNotFound,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("sentinel error should not be nil")
		}
		if err.Error() == "" {
			t.Error("sentinel error should have a message")
		}
	}
}
