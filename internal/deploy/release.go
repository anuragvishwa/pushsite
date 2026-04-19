package deploy

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Release represents a single deployment release
type Release struct {
	Name      string // timestamp-based directory name
	Path      string // full path on server
	Timestamp time.Time
}

// ReleaseManager handles release directory management on the server
type ReleaseManager struct {
	conn         connection.Connection
	webRoot      string
	keepReleases int
}

// NewReleaseManager creates a new ReleaseManager
func NewReleaseManager(conn connection.Connection, webRoot string, keepReleases int) *ReleaseManager {
	if keepReleases <= 0 {
		keepReleases = 5
	}
	return &ReleaseManager{
		conn:         conn,
		webRoot:      webRoot,
		keepReleases: keepReleases,
	}
}

// releasesDir returns the path to the releases directory
func (rm *ReleaseManager) releasesDir() string {
	return filepath.Join(rm.webRoot, "releases")
}

// currentLink returns the path to the current symlink
func (rm *ReleaseManager) currentLink() string {
	return filepath.Join(rm.webRoot, "current")
}

// sharedDir returns the path to the shared directory
func (rm *ReleaseManager) sharedDir() string {
	return filepath.Join(rm.webRoot, "shared")
}

// CreateRelease creates a new timestamped release directory
func (rm *ReleaseManager) CreateRelease() (*Release, error) {
	now := time.Now().UTC()
	name := now.Format("20060102150405")
	releasePath := filepath.Join(rm.releasesDir(), name)

	if err := rm.conn.MkdirAll(releasePath); err != nil {
		return nil, fmt.Errorf("failed to create release directory: %w", err)
	}

	return &Release{
		Name:      name,
		Path:      releasePath,
		Timestamp: now,
	}, nil
}

// SetupDirectories ensures the base directory structure exists
func (rm *ReleaseManager) SetupDirectories() error {
	dirs := []string{
		rm.releasesDir(),
		rm.sharedDir(),
	}

	for _, dir := range dirs {
		if err := rm.conn.MkdirAll(dir); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// Symlink updates the current symlink to point to the given release
func (rm *ReleaseManager) Symlink(release *Release) error {
	currentLink := rm.currentLink()

	// Remove existing symlink
	cmd := fmt.Sprintf("rm -f %s && ln -sfn %s %s", currentLink, release.Path, currentLink)
	_, err := rm.conn.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to update symlink: %w", err)
	}

	return nil
}

// ListReleases returns all releases sorted by timestamp (newest first)
func (rm *ReleaseManager) ListReleases() ([]Release, error) {
	output, err := rm.conn.Execute(fmt.Sprintf("ls -1 %s 2>/dev/null || true", rm.releasesDir()))
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	var releases []Release
	for _, name := range strings.Split(strings.TrimSpace(output), "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		t, err := time.Parse("20060102150405", name)
		if err != nil {
			continue // skip non-release directories
		}

		releases = append(releases, Release{
			Name:      name,
			Path:      filepath.Join(rm.releasesDir(), name),
			Timestamp: t,
		})
	}

	sort.Slice(releases, func(i, j int) bool {
		return releases[i].Timestamp.After(releases[j].Timestamp)
	})

	return releases, nil
}

// CurrentRelease returns the release that current points to
func (rm *ReleaseManager) CurrentRelease() (*Release, error) {
	output, err := rm.conn.Execute(fmt.Sprintf("readlink %s 2>/dev/null || echo ''", rm.currentLink()))
	if err != nil {
		return nil, err
	}

	target := strings.TrimSpace(output)
	if target == "" {
		return nil, nil // no current release
	}

	name := filepath.Base(target)
	t, err := time.Parse("20060102150405", name)
	if err != nil {
		return nil, fmt.Errorf("current release has invalid name: %s", name)
	}

	return &Release{
		Name:      name,
		Path:      target,
		Timestamp: t,
	}, nil
}

// Cleanup removes old releases beyond the keepReleases limit
func (rm *ReleaseManager) Cleanup() error {
	releases, err := rm.ListReleases()
	if err != nil {
		return err
	}

	if len(releases) <= rm.keepReleases {
		return nil
	}

	// Get current release to avoid removing it
	current, _ := rm.CurrentRelease()

	for _, release := range releases[rm.keepReleases:] {
		if current != nil && release.Name == current.Name {
			continue // don't remove current
		}

		_, err := rm.conn.Execute(fmt.Sprintf("rm -rf %s", release.Path))
		if err != nil {
			return fmt.Errorf("failed to remove old release %s: %w", release.Name, err)
		}
	}

	return nil
}

// Rollback switches the current symlink to a previous release
func (rm *ReleaseManager) Rollback(targetName string) (*Release, error) {
	releases, err := rm.ListReleases()
	if err != nil {
		return nil, err
	}

	if targetName == "" {
		// Rollback to previous release
		if len(releases) < 2 {
			return nil, fmt.Errorf("no previous release to rollback to")
		}
		target := releases[1]
		if err := rm.Symlink(&target); err != nil {
			return nil, err
		}
		return &target, nil
	}

	// Find specific release
	for _, r := range releases {
		if r.Name == targetName {
			if err := rm.Symlink(&r); err != nil {
				return nil, err
			}
			return &r, nil
		}
	}

	return nil, fmt.Errorf("release not found: %s", targetName)
}
