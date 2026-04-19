package env

import (
	"fmt"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Manager handles environment variables on the remote server
type Manager struct {
	conn    connection.Connection
	webRoot string
}

// New creates a new env Manager
func New(conn connection.Connection, webRoot string) *Manager {
	return &Manager{conn: conn, webRoot: webRoot}
}

// envFilePath returns the path to the shared .env file
func (m *Manager) envFilePath() string {
	return m.webRoot + "/shared/.env"
}

// Push writes environment variables to the server
func (m *Manager) Push(envVars map[string]string) error {
	if len(envVars) == 0 {
		return nil
	}

	var lines []string
	for k, v := range envVars {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}

	content := strings.Join(lines, "\n") + "\n"
	cmd := fmt.Sprintf("cat > %s << 'PUSHSITE_ENV'\n%sPUSHSITE_ENV", m.envFilePath(), content)
	_, err := m.conn.Execute(cmd)
	return err
}

// Pull reads the current .env file from the server
func (m *Manager) Pull() (map[string]string, error) {
	output, err := m.conn.Execute(fmt.Sprintf("cat %s 2>/dev/null || echo ''", m.envFilePath()))
	if err != nil {
		return nil, err
	}

	envVars := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		}
	}

	return envVars, nil
}

// Set sets a single environment variable on the server
func (m *Manager) Set(key, value string) error {
	existing, err := m.Pull()
	if err != nil {
		existing = make(map[string]string)
	}
	existing[key] = value
	return m.Push(existing)
}

// Remove removes an environment variable from the server
func (m *Manager) Remove(key string) error {
	existing, err := m.Pull()
	if err != nil {
		return err
	}
	delete(existing, key)
	return m.Push(existing)
}
