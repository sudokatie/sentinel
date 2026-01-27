package alerter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
)

func setupTestStorage(t *testing.T) storage.Storage {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		os.Remove(dbPath)
	})

	return store
}

func TestNewManager(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		ConsecutiveFailures:  2,
		RecoveryNotification: true,
		CooldownMinutes:      5,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	if manager == nil {
		t.Fatal("expected manager to be created")
	}
	if manager.email != nil {
		t.Error("expected email sender to be nil when disabled")
	}
}

func TestNewManagerWithEmail(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		Email: config.EmailConfig{
			Enabled:     true,
			SMTPHost:    "smtp.example.com",
			SMTPPort:    587,
			FromAddress: "test@example.com",
			ToAddresses: []string{"alert@example.com"},
		},
	}

	manager := NewManager(cfg, store)

	if manager.email == nil {
		t.Error("expected email sender to be created when enabled")
	}
}

func TestBuildDownEmail(t *testing.T) {
	sender := &EmailSender{
		config: &config.EmailConfig{
			FromAddress: "sentinel@example.com",
			ToAddresses: []string{"alert@example.com"},
		},
	}

	check := &storage.Check{
		Name: "Test API",
		URL:  "https://api.example.com/health",
	}

	alert := &Alert{
		Type:      "down",
		Check:     check,
		Error:     "connection refused",
		Timestamp: time.Now(),
	}

	subject, body := sender.buildDownEmail(alert)

	if subject != "[SENTINEL] DOWN: Test API" {
		t.Errorf("unexpected subject: %s", subject)
	}

	if !contains(body, "Test API") {
		t.Error("body should contain check name")
	}
	if !contains(body, "https://api.example.com/health") {
		t.Error("body should contain URL")
	}
	if !contains(body, "connection refused") {
		t.Error("body should contain error message")
	}
	if !contains(body, "DOWN") {
		t.Error("body should contain status DOWN")
	}
}

func TestBuildRecoveryEmail(t *testing.T) {
	sender := &EmailSender{
		config: &config.EmailConfig{
			FromAddress: "sentinel@example.com",
			ToAddresses: []string{"alert@example.com"},
		},
	}

	check := &storage.Check{
		Name: "Test API",
		URL:  "https://api.example.com/health",
	}

	incident := &storage.Incident{
		ID:              1,
		CheckID:         1,
		StartedAt:       time.Now().Add(-5 * time.Minute),
		DurationSeconds: 300,
	}

	alert := &Alert{
		Type:      "recovery",
		Check:     check,
		Incident:  incident,
		Timestamp: time.Now(),
	}

	subject, body := sender.buildRecoveryEmail(alert)

	if subject != "[SENTINEL] RECOVERED: Test API" {
		t.Errorf("unexpected subject: %s", subject)
	}

	if !contains(body, "Test API") {
		t.Error("body should contain check name")
	}
	if !contains(body, "UP") {
		t.Error("body should contain status UP")
	}
	if !contains(body, "5m") {
		t.Error("body should contain downtime duration")
	}
}

func TestShouldSendAlertCooldown(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		CooldownMinutes: 5,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	// Create check and incident
	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	incident := &storage.Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
	}
	if err := store.CreateIncident(incident); err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}

	alert := &Alert{
		Type:     "down",
		Check:    check,
		Incident: incident,
	}

	// First alert should be allowed
	if !manager.shouldSendAlert(alert) {
		t.Error("first alert should be allowed")
	}

	// Log a successful alert
	store.LogAlert(&storage.AlertLog{
		IncidentID: incident.ID,
		Channel:    "email",
		Success:    true,
	})

	// Second alert within cooldown should be blocked
	if manager.shouldSendAlert(alert) {
		t.Error("second alert within cooldown should be blocked")
	}
}

func TestRecoveryNotificationDisabled(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		RecoveryNotification: false,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		Name: "Test",
		URL:  "https://test.com",
	}

	incident := &storage.Incident{
		ID:        1,
		CheckID:   1,
		StartedAt: time.Now(),
	}

	// Should return nil immediately when recovery notifications are disabled
	err := manager.SendRecoveryAlert(check, incident)
	if err != nil {
		t.Errorf("expected no error when recovery disabled, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
