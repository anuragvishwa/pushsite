package connection

import "io"

type Connection interface {
	Connect() error
	Close() error
	Execute(cmd string) (string, error)
	Upload(localPath, remotePath string) error
	UploadReader(reader io.Reader, remotePath string, size int64) error
	Download(remotePath, localPath string) error
	MkdirAll(path string) error
}

type Config struct {
	Host       string
	User       string
	KeyPath    string
	Port       int
	Method     string // "ssh" or "ssm"
	InstanceID string // for SSM
	Region     string // for SSM
}

func (c *Config) SetDefaults() {
	if c.User == "" {
		c.User = "ubuntu"
	}
	if c.Port == 0 {
		c.Port = 22
	}
	if c.Method == "" {
		c.Method = "ssh"
	}
	if c.Region == "" {
		c.Region = "us-east-1"
	}
}
