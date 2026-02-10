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

func TestSendDownAlertNoEmail(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		ConsecutiveFailures: 2,
		CooldownMinutes:     5,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		ID:   1,
		Name: "Test Check",
		URL:  "https://test.com",
	}

	incident := &storage.Incident{
		ID:        1,
		CheckID:   1,
		StartedAt: time.Now(),
	}

	// Should not error when email is disabled
	err := manager.SendDownAlert(check, incident, "connection refused")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendRecoveryAlertNoEmail(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		RecoveryNotification: true,
		CooldownMinutes:      5,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		ID:   1,
		Name: "Test Check",
		URL:  "https://test.com",
	}

	incident := &storage.Incident{
		ID:              1,
		CheckID:         1,
		StartedAt:       time.Now().Add(-5 * time.Minute),
		DurationSeconds: 300,
	}

	// Should not error when email is disabled
	err := manager.SendRecoveryAlert(check, incident)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
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

func TestBuildRecoveryEmailNoDuration(t *testing.T) {
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

	// No incident means no duration
	alert := &Alert{
		Type:      "recovery",
		Check:     check,
		Incident:  nil,
		Timestamp: time.Now(),
	}

	subject, body := sender.buildRecoveryEmail(alert)

	if subject != "[SENTINEL] RECOVERED: Test API" {
		t.Errorf("unexpected subject: %s", subject)
	}

	if !contains(body, "unknown") {
		t.Error("body should contain 'unknown' for missing duration")
	}
}

func TestBuildEmail(t *testing.T) {
	sender := &EmailSender{
		config: &config.EmailConfig{
			FromAddress: "sentinel@example.com",
			ToAddresses: []string{"alert@example.com"},
		},
	}

	check := &storage.Check{
		Name: "Test",
		URL:  "https://test.com",
	}

	// Test down alert
	downAlert := &Alert{
		Type:      "down",
		Check:     check,
		Error:     "error",
		Timestamp: time.Now(),
	}

	subject, _ := sender.buildEmail(downAlert)
	if !contains(subject, "DOWN") {
		t.Error("down alert should have DOWN in subject")
	}

	// Test recovery alert
	recoveryAlert := &Alert{
		Type:      "recovery",
		Check:     check,
		Timestamp: time.Now(),
	}

	subject, _ = sender.buildEmail(recoveryAlert)
	if !contains(subject, "RECOVERED") {
		t.Error("recovery alert should have RECOVERED in subject")
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

func TestShouldSendAlertNoIncident(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		CooldownMinutes: 5,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		Name: "Test",
		URL:  "https://test.com",
	}

	// Alert without incident should be allowed
	alert := &Alert{
		Type:     "down",
		Check:    check,
		Incident: nil,
	}

	if !manager.shouldSendAlert(alert) {
		t.Error("alert without incident should be allowed")
	}
}

func TestShouldSendAlertFailedPreviousAlert(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		CooldownMinutes: 5,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	incident := &storage.Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
	}
	store.CreateIncident(incident)

	// Log a failed alert
	store.LogAlert(&storage.AlertLog{
		IncidentID:   incident.ID,
		Channel:      "email",
		Success:      false,
		ErrorMessage: "SMTP error",
	})

	alert := &Alert{
		Type:     "down",
		Check:    check,
		Incident: incident,
	}

	// Alert should be allowed because previous one failed
	if !manager.shouldSendAlert(alert) {
		t.Error("alert should be allowed after failed previous alert")
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

func TestLogAlert(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	incident := &storage.Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
	}
	store.CreateIncident(incident)

	alert := &Alert{
		Type:     "down",
		Check:    check,
		Incident: incident,
	}

	// Log alert
	manager.logAlert(alert, "email", true, "")

	// Verify logged
	last, err := store.GetLastAlertForIncident(incident.ID, "email")
	if err != nil {
		t.Fatalf("failed to get last alert: %v", err)
	}
	if last == nil {
		t.Fatal("expected alert to be logged")
	}
	if !last.Success {
		t.Error("expected success to be true")
	}
}

func TestLogAlertNoIncident(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		Name: "Test",
		URL:  "https://test.com",
	}

	// Alert without incident - should not panic
	alert := &Alert{
		Type:     "down",
		Check:    check,
		Incident: nil,
	}

	// This should not panic or error
	manager.logAlert(alert, "email", true, "")
}

func TestNewEmailSender(t *testing.T) {
	cfg := &config.EmailConfig{
		Enabled:      true,
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUser:     "user@example.com",
		SMTPPassword: "password",
		SMTPTLS:      true,
		FromAddress:  "sender@example.com",
		ToAddresses:  []string{"recipient@example.com"},
	}

	sender := NewEmailSender(cfg)

	if sender == nil {
		t.Fatal("expected email sender to be created")
	}
	if sender.config != cfg {
		t.Error("expected config to be set")
	}
}

func TestSendAlertNoChannels(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		CooldownMinutes: 5,
		Email: config.EmailConfig{
			Enabled: false, // No email configured
		},
	}

	manager := NewManager(cfg, store)

	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	incident := &storage.Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
	}
	store.CreateIncident(incident)

	alert := &Alert{
		Type:      "down",
		Check:     check,
		Incident:  incident,
		Error:     "test error",
		Timestamp: time.Now(),
	}

	// Should not error even with no channels
	err := manager.sendAlert(alert)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAlertStructure(t *testing.T) {
	check := &storage.Check{
		ID:   1,
		Name: "Test Check",
		URL:  "https://test.com",
	}

	incident := &storage.Incident{
		ID:        1,
		CheckID:   1,
		StartedAt: time.Now(),
	}

	alert := &Alert{
		Type:      "down",
		Check:     check,
		Incident:  incident,
		Error:     "connection refused",
		Timestamp: time.Now(),
	}

	if alert.Type != "down" {
		t.Error("expected Type to be 'down'")
	}
	if alert.Check != check {
		t.Error("expected Check to be set")
	}
	if alert.Incident != incident {
		t.Error("expected Incident to be set")
	}
	if alert.Error != "connection refused" {
		t.Error("expected Error to be set")
	}
}

func TestManagerConfig(t *testing.T) {
	store := setupTestStorage(t)

	cfg := &config.AlertsConfig{
		ConsecutiveFailures:  3,
		RecoveryNotification: true,
		CooldownMinutes:      10,
		Email: config.EmailConfig{
			Enabled: false,
		},
	}

	manager := NewManager(cfg, store)

	if manager.config.ConsecutiveFailures != 3 {
		t.Errorf("expected consecutive failures 3, got %d", manager.config.ConsecutiveFailures)
	}
	if !manager.config.RecoveryNotification {
		t.Error("expected recovery notification to be true")
	}
	if manager.config.CooldownMinutes != 10 {
		t.Errorf("expected cooldown 10, got %d", manager.config.CooldownMinutes)
	}
}

func TestEmailConfigStructure(t *testing.T) {
	cfg := &config.EmailConfig{
		Enabled:      true,
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUser:     "user@example.com",
		SMTPPassword: "password",
		SMTPTLS:      true,
		FromAddress:  "sender@example.com",
		ToAddresses:  []string{"recipient1@example.com", "recipient2@example.com"},
	}

	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if cfg.SMTPHost != "smtp.example.com" {
		t.Error("expected SMTPHost to be set")
	}
	if cfg.SMTPPort != 587 {
		t.Error("expected SMTPPort to be 587")
	}
	if len(cfg.ToAddresses) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(cfg.ToAddresses))
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
