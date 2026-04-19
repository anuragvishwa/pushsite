package connection

import "fmt"

type ConnectionError struct {
	Op      string
	Host    string
	Message string
	Err     error
}

func (e *ConnectionError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s %s: %s: %v", e.Op, e.Host, e.Message, e.Err)
	}
	return fmt.Sprintf("%s %s: %s", e.Op, e.Host, e.Message)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

var (
	ErrNotConnected    = fmt.Errorf("not connected")
	ErrKeyNotFound     = fmt.Errorf("SSH key file not found")
	ErrInvalidKey      = fmt.Errorf("invalid SSH key format")
	ErrAuthFailed      = fmt.Errorf("authentication failed")
	ErrTimeout         = fmt.Errorf("connection timeout")
	ErrCommandFailed   = fmt.Errorf("command execution failed")
	ErrUploadFailed    = fmt.Errorf("file upload failed")
	ErrDownloadFailed  = fmt.Errorf("file download failed")
	ErrInstanceNotFound = fmt.Errorf("SSM instance not found")
)
