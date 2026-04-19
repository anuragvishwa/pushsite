package ssl

import (
	"fmt"
	"strings"

	"github.com/anuragvishwa/pushsite/internal/connection"
)

// Manager handles SSL certificate management via certbot
type Manager struct {
	conn   connection.Connection
	domain string
}

// New creates a new SSL Manager
func New(conn connection.Connection, domain string) *Manager {
	return &Manager{conn: conn, domain: domain}
}

// Obtain gets a new SSL certificate using certbot
func (m *Manager) Obtain(email string) error {
	cmd := fmt.Sprintf(
		"sudo certbot --nginx -d %s --non-interactive --agree-tos --email %s --redirect",
		m.domain, email,
	)
	_, err := m.conn.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}
	return nil
}

// ObtainStaging gets a staging certificate (for testing)
func (m *Manager) ObtainStaging(email string) error {
	cmd := fmt.Sprintf(
		"sudo certbot --nginx -d %s --non-interactive --agree-tos --email %s --redirect --staging",
		m.domain, email,
	)
	_, err := m.conn.Execute(cmd)
	if err != nil {
		return fmt.Errorf("failed to obtain staging certificate: %w", err)
	}
	return nil
}

// Renew attempts to renew all certificates
func (m *Manager) Renew() error {
	_, err := m.conn.Execute("sudo certbot renew --dry-run 2>&1")
	if err != nil {
		return fmt.Errorf("certificate renewal check failed: %w", err)
	}

	_, err = m.conn.Execute("sudo certbot renew 2>&1")
	if err != nil {
		return fmt.Errorf("certificate renewal failed: %w", err)
	}

	return nil
}

// Status checks the certificate status for the domain
func (m *Manager) Status() (*CertStatus, error) {
	output, err := m.conn.Execute("sudo certbot certificates 2>&1")
	if err != nil {
		return nil, fmt.Errorf("failed to check certificates: %w", err)
	}

	status := &CertStatus{
		Domain:    m.domain,
		RawOutput: output,
	}

	if strings.Contains(output, m.domain) {
		status.HasCert = true

		// Parse expiry from output
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Expiry Date:") {
				status.Expiry = strings.TrimPrefix(line, "Expiry Date:")
				status.Expiry = strings.TrimSpace(strings.Split(status.Expiry, "(")[0])
			}
		}
	}

	return status, nil
}

// CertStatus holds information about SSL certificate status
type CertStatus struct {
	Domain    string
	HasCert   bool
	Expiry    string
	RawOutput string
}
