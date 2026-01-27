package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *SQLiteStorage {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	s, err := NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	t.Cleanup(func() {
		s.Close()
		os.Remove(dbPath)
	})

	return s
}

func TestCreateAndGetCheck(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "Test Check",
		URL:            "https://example.com",
		IntervalSecs:   30,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Tags:           []string{"api", "production"},
	}

	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	if check.ID == 0 {
		t.Error("expected check ID to be set")
	}

	// Get by ID
	got, err := s.GetCheck(check.ID)
	if err != nil {
		t.Fatalf("failed to get check: %v", err)
	}

	if got.Name != check.Name {
		t.Errorf("expected name %s, got %s", check.Name, got.Name)
	}
	if got.URL != check.URL {
		t.Errorf("expected url %s, got %s", check.URL, got.URL)
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}

	// Get by URL
	got, err = s.GetCheckByURL(check.URL)
	if err != nil {
		t.Fatalf("failed to get check by url: %v", err)
	}
	if got.ID != check.ID {
		t.Error("expected same check")
	}
}

func TestGetCheckNotFound(t *testing.T) {
	s := setupTestDB(t)

	got, err := s.GetCheck(999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil for non-existent check")
	}
}

func TestListChecks(t *testing.T) {
	s := setupTestDB(t)

	// Create a few checks
	checks := []*Check{
		{Name: "Check A", URL: "https://a.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true},
		{Name: "Check B", URL: "https://b.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true},
		{Name: "Check C", URL: "https://c.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: false},
	}

	for _, c := range checks {
		if err := s.CreateCheck(c); err != nil {
			t.Fatalf("failed to create check: %v", err)
		}
	}

	// List all
	all, err := s.ListChecks()
	if err != nil {
		t.Fatalf("failed to list checks: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 checks, got %d", len(all))
	}

	// List enabled only
	enabled, err := s.ListEnabledChecks()
	if err != nil {
		t.Fatalf("failed to list enabled checks: %v", err)
	}
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled checks, got %d", len(enabled))
	}
}

func TestUpdateCheck(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "Original",
		URL:            "https://original.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}

	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	check.Name = "Updated"
	check.URL = "https://updated.com"
	check.Enabled = false

	if err := s.UpdateCheck(check); err != nil {
		t.Fatalf("failed to update check: %v", err)
	}

	got, err := s.GetCheck(check.ID)
	if err != nil {
		t.Fatalf("failed to get check: %v", err)
	}

	if got.Name != "Updated" {
		t.Errorf("expected name Updated, got %s", got.Name)
	}
	if got.URL != "https://updated.com" {
		t.Errorf("expected url https://updated.com, got %s", got.URL)
	}
	if got.Enabled {
		t.Error("expected check to be disabled")
	}
}

func TestDeleteCheck(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "To Delete",
		URL:            "https://delete.me",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}

	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	if err := s.DeleteCheck(check.ID); err != nil {
		t.Fatalf("failed to delete check: %v", err)
	}

	got, err := s.GetCheck(check.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestSaveAndGetResults(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "Test",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	// Save some results
	results := []*CheckResult{
		{CheckID: check.ID, Status: "up", StatusCode: 200, ResponseTimeMs: 100},
		{CheckID: check.ID, Status: "up", StatusCode: 200, ResponseTimeMs: 150},
		{CheckID: check.ID, Status: "down", StatusCode: 0, ErrorMessage: "timeout"},
	}

	for _, r := range results {
		if err := s.SaveResult(r); err != nil {
			t.Fatalf("failed to save result: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Get latest
	latest, err := s.GetLatestResult(check.ID)
	if err != nil {
		t.Fatalf("failed to get latest result: %v", err)
	}
	if latest.Status != "down" {
		t.Errorf("expected latest status down, got %s", latest.Status)
	}

	// Get all with pagination
	all, err := s.GetResults(check.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 results, got %d", len(all))
	}
}

func TestGetStats(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "Stats Test",
		URL:            "https://stats.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	// Save mixed results
	for i := 0; i < 10; i++ {
		status := "up"
		if i%5 == 0 {
			status = "down"
		}
		result := &CheckResult{
			CheckID:        check.ID,
			Status:         status,
			StatusCode:     200,
			ResponseTimeMs: 100 + i*10,
		}
		if err := s.SaveResult(result); err != nil {
			t.Fatalf("failed to save result: %v", err)
		}
	}

	stats, err := s.GetStats(check.ID)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	// 8 up out of 10 = 80%
	if stats.UptimePercent24h < 79 || stats.UptimePercent24h > 81 {
		t.Errorf("expected ~80%% uptime, got %.2f%%", stats.UptimePercent24h)
	}
}

func TestIncidents(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "Incident Test",
		URL:            "https://incident.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	// Create incident
	incident := &Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
		Cause:     "Connection timeout",
	}
	if err := s.CreateIncident(incident); err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}

	if incident.ID == 0 {
		t.Error("expected incident ID to be set")
	}

	// Get active incident
	active, err := s.GetActiveIncident(check.ID)
	if err != nil {
		t.Fatalf("failed to get active incident: %v", err)
	}
	if active == nil {
		t.Fatal("expected active incident")
	}
	if active.ID != incident.ID {
		t.Error("expected same incident")
	}

	// Close incident
	endTime := time.Now().Add(5 * time.Minute)
	if err := s.CloseIncident(incident.ID, endTime); err != nil {
		t.Fatalf("failed to close incident: %v", err)
	}

	// Verify closed
	closed, err := s.GetIncident(incident.ID)
	if err != nil {
		t.Fatalf("failed to get incident: %v", err)
	}
	if closed.EndedAt == nil {
		t.Error("expected ended_at to be set")
	}
	if closed.DurationSeconds < 299 || closed.DurationSeconds > 301 {
		t.Errorf("expected ~300 seconds duration, got %d", closed.DurationSeconds)
	}

	// No active incident now
	active, err = s.GetActiveIncident(check.ID)
	if err != nil {
		t.Fatalf("failed to get active incident: %v", err)
	}
	if active != nil {
		t.Error("expected no active incident after close")
	}
}

func TestAlertLog(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "Alert Test",
		URL:            "https://alert.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	incident := &Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
		Cause:     "Test",
	}
	if err := s.CreateIncident(incident); err != nil {
		t.Fatalf("failed to create incident: %v", err)
	}

	log := &AlertLog{
		IncidentID: incident.ID,
		Channel:    "email",
		Success:    true,
	}
	if err := s.LogAlert(log); err != nil {
		t.Fatalf("failed to log alert: %v", err)
	}

	last, err := s.GetLastAlertForIncident(incident.ID, "email")
	if err != nil {
		t.Fatalf("failed to get last alert: %v", err)
	}
	if last == nil {
		t.Fatal("expected alert log")
	}
	if !last.Success {
		t.Error("expected success to be true")
	}
}

func TestCleanupOldResults(t *testing.T) {
	s := setupTestDB(t)

	check := &Check{
		Name:           "Cleanup Test",
		URL:            "https://cleanup.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := s.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	// Save result
	result := &CheckResult{
		CheckID:        check.ID,
		Status:         "up",
		StatusCode:     200,
		ResponseTimeMs: 100,
	}
	if err := s.SaveResult(result); err != nil {
		t.Fatalf("failed to save result: %v", err)
	}

	// Cleanup future results (should delete our result)
	if err := s.CleanupOldResults(time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	results, err := s.GetResults(check.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after cleanup, got %d", len(results))
	}
}
