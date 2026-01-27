package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()

	if c.Server.Port != 3000 {
		t.Errorf("expected port 3000, got %d", c.Server.Port)
	}
	if c.Server.Host != "0.0.0.0" {
		t.Errorf("expected host 0.0.0.0, got %s", c.Server.Host)
	}
	if c.Database.Path != "./sentinel.db" {
		t.Errorf("expected db path ./sentinel.db, got %s", c.Database.Path)
	}
	if c.Alerts.ConsecutiveFailures != 2 {
		t.Errorf("expected consecutive_failures 2, got %d", c.Alerts.ConsecutiveFailures)
	}
	if !c.Alerts.RecoveryNotification {
		t.Error("expected recovery_notification to be true")
	}
	if c.Alerts.Email.Enabled {
		t.Error("expected email to be disabled by default")
	}
}

func TestLoadMissingFile(t *testing.T) {
	c, err := Load("/nonexistent/path/sentinel.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if c.Server.Port != 3000 {
		t.Errorf("expected default port, got %d", c.Server.Port)
	}
}

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sentinel.yaml")

	content := `
server:
  port: 8080
  host: 127.0.0.1
database:
  path: /tmp/test.db
alerts:
  consecutive_failures: 3
  email:
    enabled: false
checks:
  - name: Test Check
    url: https://example.com
    interval: 30s
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	c, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if c.Server.Port != 8080 {
		t.Errorf("expected port 8080, got %d", c.Server.Port)
	}
	if c.Server.Host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %s", c.Server.Host)
	}
	if c.Database.Path != "/tmp/test.db" {
		t.Errorf("expected db path /tmp/test.db, got %s", c.Database.Path)
	}
	if c.Alerts.ConsecutiveFailures != 3 {
		t.Errorf("expected consecutive_failures 3, got %d", c.Alerts.ConsecutiveFailures)
	}
	if len(c.Checks) != 1 {
		t.Fatalf("expected 1 check, got %d", len(c.Checks))
	}
	if c.Checks[0].Name != "Test Check" {
		t.Errorf("expected check name 'Test Check', got %s", c.Checks[0].Name)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sentinel.yaml")

	content := `
server:
  port: [invalid
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "sentinel.yaml")

	content := `
server:
  port: 3000
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	os.Setenv("SENTINEL_PORT", "9999")
	os.Setenv("SENTINEL_DB_PATH", "/custom/path.db")
	defer func() {
		os.Unsetenv("SENTINEL_PORT")
		os.Unsetenv("SENTINEL_DB_PATH")
	}()

	c, err := LoadWithEnv(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if c.Server.Port != 9999 {
		t.Errorf("expected port 9999 from env, got %d", c.Server.Port)
	}
	if c.Database.Path != "/custom/path.db" {
		t.Errorf("expected db path /custom/path.db from env, got %s", c.Database.Path)
	}
}

func TestValidateInvalidPort(t *testing.T) {
	c := DefaultConfig()
	c.Server.Port = 0
	if err := c.Validate(); err == nil {
		t.Error("expected error for port 0")
	}

	c.Server.Port = 70000
	if err := c.Validate(); err == nil {
		t.Error("expected error for port 70000")
	}
}

func TestValidateEmptyDBPath(t *testing.T) {
	c := DefaultConfig()
	c.Database.Path = ""
	if err := c.Validate(); err == nil {
		t.Error("expected error for empty db path")
	}
}

func TestValidateEmailEnabled(t *testing.T) {
	c := DefaultConfig()
	c.Alerts.Email.Enabled = true

	if err := c.Validate(); err == nil {
		t.Error("expected error for email enabled without smtp_host")
	}

	c.Alerts.Email.SMTPHost = "smtp.example.com"
	if err := c.Validate(); err == nil {
		t.Error("expected error for email enabled without from_address")
	}

	c.Alerts.Email.FromAddress = "test@example.com"
	if err := c.Validate(); err == nil {
		t.Error("expected error for email enabled without to_addresses")
	}

	c.Alerts.Email.ToAddresses = []string{"alert@example.com"}
	if err := c.Validate(); err != nil {
		t.Errorf("expected no error with valid email config, got %v", err)
	}
}

func TestValidateCheckConfig(t *testing.T) {
	c := DefaultConfig()
	c.Checks = []CheckConfig{
		{Name: "", URL: "https://example.com"},
	}
	if err := c.Validate(); err == nil {
		t.Error("expected error for check without name")
	}

	c.Checks = []CheckConfig{
		{Name: "Test", URL: ""},
	}
	if err := c.Validate(); err == nil {
		t.Error("expected error for check without url")
	}

	c.Checks = []CheckConfig{
		{Name: "Test", URL: "https://example.com", Interval: "invalid"},
	}
	if err := c.Validate(); err == nil {
		t.Error("expected error for check with invalid interval")
	}
}

func TestCheckConfigHelpers(t *testing.T) {
	check := CheckConfig{
		Name: "Test",
		URL:  "https://example.com",
	}

	// Test defaults
	if check.GetInterval() != time.Hour {
		t.Errorf("expected default interval 1h, got %v", check.GetInterval())
	}
	if check.GetTimeout() != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", check.GetTimeout())
	}
	if check.GetExpectedStatus() != 200 {
		t.Errorf("expected default status 200, got %d", check.GetExpectedStatus())
	}
	if !check.IsEnabled() {
		t.Error("expected check to be enabled by default")
	}

	// Test custom values
	check.Interval = "30s"
	check.Timeout = "5s"
	check.ExpectedStatus = 201
	enabled := false
	check.Enabled = &enabled

	if check.GetInterval() != 30*time.Second {
		t.Errorf("expected interval 30s, got %v", check.GetInterval())
	}
	if check.GetTimeout() != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", check.GetTimeout())
	}
	if check.GetExpectedStatus() != 201 {
		t.Errorf("expected status 201, got %d", check.GetExpectedStatus())
	}
	if check.IsEnabled() {
		t.Error("expected check to be disabled")
	}
}
