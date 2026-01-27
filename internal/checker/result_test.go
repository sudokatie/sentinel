package checker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

type mockAlerter struct {
	downAlerts     int
	recoveryAlerts int
	lastCheck      *storage.Check
	lastIncident   *storage.Incident
}

func (m *mockAlerter) SendDownAlert(check *storage.Check, incident *storage.Incident, errorMsg string) error {
	m.downAlerts++
	m.lastCheck = check
	m.lastIncident = incident
	return nil
}

func (m *mockAlerter) SendRecoveryAlert(check *storage.Check, incident *storage.Incident) error {
	m.recoveryAlerts++
	m.lastCheck = check
	m.lastIncident = incident
	return nil
}

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

func TestDetermineStatus(t *testing.T) {
	tests := []struct {
		name           string
		response       *CheckResponse
		expectedStatus int
		want           string
	}{
		{
			name:           "success 200",
			response:       &CheckResponse{StatusCode: 200},
			expectedStatus: 200,
			want:           "up",
		},
		{
			name:           "success 201",
			response:       &CheckResponse{StatusCode: 201},
			expectedStatus: 201,
			want:           "up",
		},
		{
			name:           "wrong status code",
			response:       &CheckResponse{StatusCode: 500},
			expectedStatus: 200,
			want:           "down",
		},
		{
			name:           "error response",
			response:       &CheckResponse{Error: errors.New("timeout")},
			expectedStatus: 200,
			want:           "down",
		},
		{
			name:           "default expected status",
			response:       &CheckResponse{StatusCode: 200},
			expectedStatus: 0, // Should default to 200
			want:           "up",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetermineStatus(tt.response, tt.expectedStatus)
			if got != tt.want {
				t.Errorf("DetermineStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessResultSavesResult(t *testing.T) {
	store := setupTestStorage(t)

	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Status:         "pending", // First check
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	response := &CheckResponse{
		StatusCode:     200,
		ResponseTimeMs: 50,
	}

	if err := ProcessResult(store, nil, check, response, 2); err != nil {
		t.Fatalf("ProcessResult failed: %v", err)
	}

	// Verify result was saved
	result, err := store.GetLatestResult(check.ID)
	if err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result == nil {
		t.Fatal("expected result to be saved")
	}
	if result.Status != "up" {
		t.Errorf("expected status up, got %s", result.Status)
	}
	if result.ResponseTimeMs != 50 {
		t.Errorf("expected response time 50, got %d", result.ResponseTimeMs)
	}
}

func TestShouldAlert(t *testing.T) {
	store := setupTestStorage(t)

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

	// No results yet - should not alert
	should, err := ShouldAlert(store, check.ID, 2)
	if err != nil {
		t.Fatalf("ShouldAlert failed: %v", err)
	}
	if should {
		t.Error("should not alert with no results")
	}

	// Add one down result - not enough for threshold of 2
	store.SaveResult(&storage.CheckResult{CheckID: check.ID, Status: "down"})
	time.Sleep(10 * time.Millisecond)

	should, err = ShouldAlert(store, check.ID, 2)
	if err != nil {
		t.Fatalf("ShouldAlert failed: %v", err)
	}
	if should {
		t.Error("should not alert with only 1 down result (threshold 2)")
	}

	// Add second down result - now should alert
	store.SaveResult(&storage.CheckResult{CheckID: check.ID, Status: "down"})
	time.Sleep(10 * time.Millisecond)

	should, err = ShouldAlert(store, check.ID, 2)
	if err != nil {
		t.Fatalf("ShouldAlert failed: %v", err)
	}
	if !should {
		t.Error("should alert with 2 consecutive down results")
	}
}

func TestShouldAlertWithExistingIncident(t *testing.T) {
	store := setupTestStorage(t)

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

	// Add consecutive down results
	store.SaveResult(&storage.CheckResult{CheckID: check.ID, Status: "down"})
	time.Sleep(10 * time.Millisecond)
	store.SaveResult(&storage.CheckResult{CheckID: check.ID, Status: "down"})
	time.Sleep(10 * time.Millisecond)

	// Create an active incident
	store.CreateIncident(&storage.Incident{CheckID: check.ID, StartedAt: time.Now()})

	// Should not alert again - already have an incident
	should, err := ShouldAlert(store, check.ID, 2)
	if err != nil {
		t.Fatalf("ShouldAlert failed: %v", err)
	}
	if should {
		t.Error("should not alert when incident already exists")
	}
}

func TestProcessResultCreatesIncident(t *testing.T) {
	store := setupTestStorage(t)
	alerter := &mockAlerter{}

	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Status:         "up", // Was up before
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	// Add previous down result to meet threshold
	store.SaveResult(&storage.CheckResult{CheckID: check.ID, Status: "down"})
	time.Sleep(10 * time.Millisecond)

	// Process another down result
	response := &CheckResponse{
		Error: errors.New("connection refused"),
	}

	if err := ProcessResult(store, alerter, check, response, 2); err != nil {
		t.Fatalf("ProcessResult failed: %v", err)
	}

	// Verify incident was created
	incident, err := store.GetActiveIncident(check.ID)
	if err != nil {
		t.Fatalf("failed to get incident: %v", err)
	}
	if incident == nil {
		t.Fatal("expected incident to be created")
	}

	// Verify alert was sent
	if alerter.downAlerts != 1 {
		t.Errorf("expected 1 down alert, got %d", alerter.downAlerts)
	}
}

func TestProcessResultClosesIncident(t *testing.T) {
	store := setupTestStorage(t)
	alerter := &mockAlerter{}

	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Status:         "down", // Was down before
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	// Create an active incident
	incident := &storage.Incident{CheckID: check.ID, StartedAt: time.Now().Add(-5 * time.Minute)}
	if err := store.CreateIncident(incident); err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}

	// Process a successful result (recovery)
	response := &CheckResponse{
		StatusCode:     200,
		ResponseTimeMs: 50,
	}

	if err := ProcessResult(store, alerter, check, response, 2); err != nil {
		t.Fatalf("ProcessResult failed: %v", err)
	}

	// Verify incident was closed
	active, err := store.GetActiveIncident(check.ID)
	if err != nil {
		t.Fatalf("failed to get incident: %v", err)
	}
	if active != nil {
		t.Error("expected no active incident after recovery")
	}

	// Verify recovery alert was sent
	if alerter.recoveryAlerts != 1 {
		t.Errorf("expected 1 recovery alert, got %d", alerter.recoveryAlerts)
	}
}

func TestProcessResultNoAlertOnFirstCheck(t *testing.T) {
	store := setupTestStorage(t)
	alerter := &mockAlerter{}

	check := &storage.Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Status:         "pending", // First check
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	// Process a down result - but it's the first check
	response := &CheckResponse{
		Error: errors.New("connection refused"),
	}

	if err := ProcessResult(store, alerter, check, response, 2); err != nil {
		t.Fatalf("ProcessResult failed: %v", err)
	}

	// Should not create incident on first check
	incident, err := store.GetActiveIncident(check.ID)
	if err != nil {
		t.Fatalf("failed to get incident: %v", err)
	}
	if incident != nil {
		t.Error("should not create incident on first check")
	}

	if alerter.downAlerts != 0 {
		t.Errorf("expected 0 down alerts on first check, got %d", alerter.downAlerts)
	}
}
