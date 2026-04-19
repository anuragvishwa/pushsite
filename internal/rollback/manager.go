package rollback

import (
	"fmt"
	"strings"
	"time"

	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Manager handles rollback operations
type Manager struct {
	conn         connection.Connection
	webRoot      string
	keepReleases int
}

// RollbackInfo contains info about a rollback operation
type RollbackInfo struct {
	FromRelease string
	ToRelease   string
	Timestamp   time.Time
}

// New creates a new rollback Manager
func New(conn connection.Connection, webRoot string, keepReleases int) *Manager {
	return &Manager{
		conn:         conn,
		webRoot:      webRoot,
		keepReleases: keepReleases,
	}
}

// GetCurrentRelease returns the currently active release name
func (m *Manager) GetCurrentRelease() (string, error) {
	output, err := m.conn.Execute(fmt.Sprintf("readlink %s/current 2>/dev/null || echo ''", m.webRoot))
	if err != nil {
		return "", err
	}

	target := strings.TrimSpace(output)
	if target == "" {
		return "", fmt.Errorf("no current release found")
	}

	// Extract the release name from the path
	parts := strings.Split(target, "/")
	return parts[len(parts)-1], nil
}

// GetPreviousRelease returns the release before the current one
func (m *Manager) GetPreviousRelease() (string, error) {
	output, err := m.conn.Execute(fmt.Sprintf("ls -1t %s/releases 2>/dev/null", m.webRoot))
	if err != nil {
		return "", err
	}

	releases := strings.Split(strings.TrimSpace(output), "\n")
	if len(releases) < 2 {
		return "", fmt.Errorf("no previous release available")
	}

	current, err := m.GetCurrentRelease()
	if err != nil {
		return "", err
	}

	for i, r := range releases {
		if strings.TrimSpace(r) == current && i+1 < len(releases) {
			return strings.TrimSpace(releases[i+1]), nil
		}
	}

	// If current isn't in the list, return the most recent
	return strings.TrimSpace(releases[0]), nil
}

// SwitchTo switches the current symlink to a specific release
func (m *Manager) SwitchTo(releaseName string) (*RollbackInfo, error) {
	// Verify the release exists
	releasePath := fmt.Sprintf("%s/releases/%s", m.webRoot, releaseName)
	_, err := m.conn.Execute(fmt.Sprintf("test -d %s", releasePath))
	if err != nil {
		return nil, fmt.Errorf("release %s does not exist", releaseName)
	}

	currentRelease, _ := m.GetCurrentRelease()

	// Update the symlink
	cmd := fmt.Sprintf("ln -sfn %s %s/current", releasePath, m.webRoot)
	if _, err := m.conn.Execute(cmd); err != nil {
		return nil, fmt.Errorf("failed to switch release: %w", err)
	}

	return &RollbackInfo{
		FromRelease: currentRelease,
		ToRelease:   releaseName,
		Timestamp:   time.Now(),
	}, nil
}

// ToPrevious rolls back to the previous release
func (m *Manager) ToPrevious() (*RollbackInfo, error) {
	prev, err := m.GetPreviousRelease()
	if err != nil {
		return nil, err
	}
	return m.SwitchTo(prev)
}
