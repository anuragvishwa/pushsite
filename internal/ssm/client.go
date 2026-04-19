package ssm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anuragvishwa/pushsite/internal/connection"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type Client struct {
	cfg       *connection.Config
	ssmClient *ssm.Client
	s3Client  *s3.Client
	bucket    string
}

func New(cfg *connection.Config) *Client {
	cfg.SetDefaults()
	return &Client{cfg: cfg}
}

func (c *Client) Connect() error {
	ctx := context.Background()

	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(c.cfg.Region))
	if err != nil {
		return &connection.ConnectionError{
			Op:      "connect",
			Host:    c.cfg.InstanceID,
			Message: "failed to load AWS config",
			Err:     err,
		}
	}

	c.ssmClient = ssm.NewFromConfig(awsCfg)
	c.s3Client = s3.NewFromConfig(awsCfg)

	// Verify instance is managed by SSM
	input := &ssm.DescribeInstanceInformationInput{
		InstanceInformationFilterList: []types.InstanceInformationFilter{
			{
				Key:      types.InstanceInformationFilterKeyInstanceIds,
				ValueSet: []string{c.cfg.InstanceID},
			},
		},
	}

	resp, err := c.ssmClient.DescribeInstanceInformation(ctx, input)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "connect",
			Host:    c.cfg.InstanceID,
			Message: "failed to describe instance",
			Err:     err,
		}
	}

	if len(resp.InstanceInformationList) == 0 {
		return &connection.ConnectionError{
			Op:      "connect",
			Host:    c.cfg.InstanceID,
			Message: "instance not found or not managed by SSM",
		}
	}

	return nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) Execute(cmd string) (string, error) {
	if c.ssmClient == nil {
		return "", connection.ErrNotConnected
	}

	ctx := context.Background()

	input := &ssm.SendCommandInput{
		InstanceIds:  []string{c.cfg.InstanceID},
		DocumentName: aws.String("AWS-RunShellScript"),
		Parameters: map[string][]string{
			"commands": {cmd},
		},
		TimeoutSeconds: aws.Int32(3600),
	}

	sendResp, err := c.ssmClient.SendCommand(ctx, input)
	if err != nil {
		return "", &connection.ConnectionError{
			Op:      "execute",
			Host:    c.cfg.InstanceID,
			Message: "failed to send command",
			Err:     err,
		}
	}

	commandID := *sendResp.Command.CommandId

	// Poll for command completion
	var output string
	for {
		time.Sleep(1 * time.Second)

		invocationInput := &ssm.GetCommandInvocationInput{
			CommandId:  aws.String(commandID),
			InstanceId: aws.String(c.cfg.InstanceID),
		}

		invResp, err := c.ssmClient.GetCommandInvocation(ctx, invocationInput)
		if err != nil {
			if strings.Contains(err.Error(), "InvocationDoesNotExist") {
				continue
			}
			return "", &connection.ConnectionError{
				Op:      "execute",
				Host:    c.cfg.InstanceID,
				Message: "failed to get command invocation",
				Err:     err,
			}
		}

		switch invResp.Status {
		case types.CommandInvocationStatusPending,
			types.CommandInvocationStatusInProgress,
			types.CommandInvocationStatusDelayed:
			continue
		case types.CommandInvocationStatusSuccess:
			output = aws.ToString(invResp.StandardOutputContent)
			return output, nil
		case types.CommandInvocationStatusCancelled,
			types.CommandInvocationStatusTimedOut,
			types.CommandInvocationStatusFailed:
			output = aws.ToString(invResp.StandardErrorContent)
			return output, &connection.ConnectionError{
				Op:      "execute",
				Host:    c.cfg.InstanceID,
				Message: fmt.Sprintf("command %s: %s", invResp.Status, output),
			}
		}
	}
}

func (c *Client) Upload(localPath, remotePath string) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "upload",
			Host:    c.cfg.InstanceID,
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
	if c.ssmClient == nil {
		return connection.ErrNotConnected
	}

	// For small files, use base64 encoding through SSM
	if size < 256*1024 {
		return c.uploadSmall(reader, remotePath)
	}

	// For larger files, use S3 as intermediary
	return c.uploadViaS3(reader, remotePath)
}

func (c *Client) uploadSmall(reader io.Reader, remotePath string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Create directory first
	dir := filepath.Dir(remotePath)
	if _, err := c.Execute(fmt.Sprintf("mkdir -p %s", dir)); err != nil {
		return err
	}

	// Write file using base64 encoding to handle binary data
	encoded := base64Encode(data)
	cmd := fmt.Sprintf("echo '%s' | base64 -d > %s", encoded, remotePath)
	_, err = c.Execute(cmd)
	return err
}

func (c *Client) uploadViaS3(reader io.Reader, remotePath string) error {
	if c.bucket == "" {
		return &connection.ConnectionError{
			Op:      "upload",
			Host:    c.cfg.InstanceID,
			Message: "S3 bucket not configured for large file transfers",
		}
	}

	ctx := context.Background()
	key := fmt.Sprintf("pushsite-transfers/%s/%d", c.cfg.InstanceID, time.Now().UnixNano())

	// Upload to S3
	_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		return &connection.ConnectionError{
			Op:      "upload",
			Host:    c.cfg.InstanceID,
			Message: "failed to upload to S3",
			Err:     err,
		}
	}

	// Download from S3 on the instance
	dir := filepath.Dir(remotePath)
	cmd := fmt.Sprintf("mkdir -p %s && aws s3 cp s3://%s/%s %s", dir, c.bucket, key, remotePath)
	_, err = c.Execute(cmd)
	if err != nil {
		return err
	}

	// Clean up S3 object
	_, _ = c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})

	return nil
}

func (c *Client) Download(remotePath, localPath string) error {
	if c.ssmClient == nil {
		return connection.ErrNotConnected
	}

	// Read file content via SSM
	cmd := fmt.Sprintf("base64 %s", remotePath)
	output, err := c.Execute(cmd)
	if err != nil {
		return &connection.ConnectionError{
			Op:      "download",
			Host:    c.cfg.InstanceID,
			Message: fmt.Sprintf("failed to read remote file: %s", remotePath),
			Err:     err,
		}
	}

	data, err := base64Decode(strings.TrimSpace(output))
	if err != nil {
		return err
	}

	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(localPath, data, 0644)
}

func (c *Client) MkdirAll(path string) error {
	_, err := c.Execute(fmt.Sprintf("mkdir -p %s", path))
	return err
}

func (c *Client) SetBucket(bucket string) {
	c.bucket = bucket
}

func base64Encode(data []byte) string {
	var buf bytes.Buffer
	encoder := base64.NewEncoder(base64.StdEncoding, &buf)
	encoder.Write(data)
	encoder.Close()
	return buf.String()
}

func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
