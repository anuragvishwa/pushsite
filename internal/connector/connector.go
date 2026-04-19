package connector

import (
	"fmt"

	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/anuragvishwa/pushsite/internal/ssh"
	"github.com/anuragvishwa/pushsite/internal/ssm"
)

func New(cfg *connection.Config) (connection.Connection, error) {
	cfg.SetDefaults()

	switch cfg.Method {
	case "ssh":
		return ssh.New(cfg), nil
	case "ssm":
		return ssm.New(cfg), nil
	default:
		return nil, fmt.Errorf("unknown connection method: %s", cfg.Method)
	}
}

func FromServerConfig(host, user, key, method, instanceID string, port int) *connection.Config {
	return &connection.Config{
		Host:       host,
		User:       user,
		KeyPath:    key,
		Port:       port,
		Method:     method,
		InstanceID: instanceID,
	}
}
