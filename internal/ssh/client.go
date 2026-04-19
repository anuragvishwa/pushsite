package ssh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type Client struct {
	config     *connection.Config
	client     *ssh.Client
	sftpClient *sftp.Client
}

func New(cfg *connection.Config) *Client {
	cfg.SetDefaults()
	return &Client{config: cfg}
}

func (c *Client) Connect() error {
	keyPath := expandPath(c.config.KeyPath)

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "connect",
			Host:    c.config.Host,
			Message: "failed to read SSH key",
			Err:     err,
		}
	}

	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "connect",
			Host:    c.config.Host,
			Message: "failed to parse SSH key",
			Err:     err,
		}
	}

	sshConfig := &ssh.ClientConfig{
		User: c.config.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	c.client, err = ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "connect",
			Host:    c.config.Host,
			Message: "failed to establish SSH connection",
			Err:     err,
		}
	}

	c.sftpClient, err = sftp.NewClient(c.client)
	if err != nil {
		c.client.Close()
		return &connection.ConnectionError{
			Op:      "connect",
			Host:    c.config.Host,
			Message: "failed to establish SFTP session",
			Err:     err,
		}
	}

	return nil
}

func (c *Client) Close() error {
	var errs []error

	if c.sftpClient != nil {
		if err := c.sftpClient.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (c *Client) Execute(cmd string) (string, error) {
	if c.client == nil {
		return "", connection.ErrNotConnected
	}

	session, err := c.client.NewSession()
	if err != nil {
		return "", &connection.ConnectionError{
			Op:      "execute",
			Host:    c.config.Host,
			Message: "failed to create session",
			Err:     err,
		}
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(cmd)
	output := stdout.String()
	if stderr.Len() > 0 {
		output += stderr.String()
	}

	if err != nil {
		return output, &connection.ConnectionError{
			Op:      "execute",
			Host:    c.config.Host,
			Message: fmt.Sprintf("command failed: %s", cmd),
			Err:     fmt.Errorf("%v: %s", err, stderr.String()),
		}
	}

	return output, nil
}

func (c *Client) Upload(localPath, remotePath string) error {
	if c.sftpClient == nil {
		return connection.ErrNotConnected
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "upload",
			Host:    c.config.Host,
			Message: fmt.Sprintf("failed to open local file: %s", localPath),
			Err:     err,
		}
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return err
	}

	return c.UploadReader(localFile, remotePath, stat.Size())
}

func (c *Client) UploadReader(reader io.Reader, remotePath string, size int64) error {
	if c.sftpClient == nil {
		return connection.ErrNotConnected
	}

	dir := filepath.Dir(remotePath)
	if err := c.MkdirAll(dir); err != nil {
		return err
	}

	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "upload",
			Host:    c.config.Host,
			Message: fmt.Sprintf("failed to create remote file: %s", remotePath),
			Err:     err,
		}
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, reader)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "upload",
			Host:    c.config.Host,
			Message: fmt.Sprintf("failed to write to remote file: %s", remotePath),
			Err:     err,
		}
	}

	return nil
}

func (c *Client) Download(remotePath, localPath string) error {
	if c.sftpClient == nil {
		return connection.ErrNotConnected
	}

	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "download",
			Host:    c.config.Host,
			Message: fmt.Sprintf("failed to open remote file: %s", remotePath),
			Err:     err,
		}
	}
	defer remoteFile.Close()

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}

	localFile, err := os.Create(localPath)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "download",
			Host:    c.config.Host,
			Message: fmt.Sprintf("failed to create local file: %s", localPath),
			Err:     err,
		}
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "download",
			Host:    c.config.Host,
			Message: fmt.Sprintf("failed to read from remote file: %s", remotePath),
			Err:     err,
		}
	}

	return nil
}

func (c *Client) MkdirAll(path string) error {
	if c.sftpClient == nil {
		return connection.ErrNotConnected
	}

	return c.sftpClient.MkdirAll(path)
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
