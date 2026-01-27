package checker

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

func setupSchedulerTest(t *testing.T) (storage.Storage, *httptest.Server) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Cleanup(func() {
		store.Close()
		server.Close()
		os.Remove(dbPath)
	})

	return store, server
}

func TestSchedulerStartStop(t *testing.T) {
	store, server := setupSchedulerTest(t)

	// Create a check
	check := &storage.Check{
		Name:           "Test Check",
		URL:            server.URL,
		IntervalSecs:   1,
		TimeoutSecs:    5,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	scheduler := NewScheduler(store, nil, SchedulerConfig{ConsecutiveFailures: 2})

	if err := scheduler.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}

	if scheduler.GetCheckCount() != 1 {
		t.Errorf("expected 1 check, got %d", scheduler.GetCheckCount())
	}

	// Wait for at least one check to execute
	time.Sleep(500 * time.Millisecond)

	scheduler.Stop()

	// Verify at least one result was saved
	results, err := store.GetResults(check.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get results: %v", err)
	}
	if len(results) < 1 {
		t.Error("expected at least one result after scheduler ran")
	}
}

func TestSchedulerDisabledCheck(t *testing.T) {
	store, server := setupSchedulerTest(t)

	// Create a disabled check
	check := &storage.Check{
		Name:           "Disabled Check",
		URL:            server.URL,
		IntervalSecs:   1,
		TimeoutSecs:    5,
		ExpectedStatus: 200,
		Enabled:        false, // Disabled
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	scheduler := NewScheduler(store, nil, SchedulerConfig{ConsecutiveFailures: 2})

	if err := scheduler.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}

	if scheduler.GetCheckCount() != 0 {
		t.Errorf("expected 0 checks (disabled), got %d", scheduler.GetCheckCount())
	}

	scheduler.Stop()
}

func TestSchedulerAddRemoveCheck(t *testing.T) {
	store, server := setupSchedulerTest(t)

	scheduler := NewScheduler(store, nil, SchedulerConfig{ConsecutiveFailures: 2})

	if err := scheduler.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}

	if scheduler.GetCheckCount() != 0 {
		t.Errorf("expected 0 checks initially, got %d", scheduler.GetCheckCount())
	}

	// Add a check
	check := &storage.Check{
		ID:             1,
		Name:           "Dynamic Check",
		URL:            server.URL,
		IntervalSecs:   60,
		TimeoutSecs:    5,
		ExpectedStatus: 200,
		Enabled:        true,
	}

	if err := scheduler.AddCheck(check); err != nil {
		t.Fatalf("failed to add check: %v", err)
	}

	if scheduler.GetCheckCount() != 1 {
		t.Errorf("expected 1 check after add, got %d", scheduler.GetCheckCount())
	}

	// Remove the check
	scheduler.RemoveCheck(check.ID)

	// Give goroutine time to clean up
	time.Sleep(100 * time.Millisecond)

	if scheduler.GetCheckCount() != 0 {
		t.Errorf("expected 0 checks after remove, got %d", scheduler.GetCheckCount())
	}

	scheduler.Stop()
}

func TestSchedulerTriggerCheck(t *testing.T) {
	store, server := setupSchedulerTest(t)

	// Create a check
	check := &storage.Check{
		Name:           "Trigger Test",
		URL:            server.URL,
		IntervalSecs:   3600, // Long interval so it won't auto-run
		TimeoutSecs:    5,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	scheduler := NewScheduler(store, nil, SchedulerConfig{ConsecutiveFailures: 2})

	// Trigger without starting scheduler
	resp, err := scheduler.TriggerCheck(check.ID)
	if err != nil {
		t.Fatalf("failed to trigger check: %v", err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify result was saved
	result, err := store.GetLatestResult(check.ID)
	if err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	if result == nil {
		t.Fatal("expected result to be saved after trigger")
	}
	if result.Status != "up" {
		t.Errorf("expected status up, got %s", result.Status)
	}
}

func TestSchedulerTriggerCheckNotFound(t *testing.T) {
	store, _ := setupSchedulerTest(t)

	scheduler := NewScheduler(store, nil, SchedulerConfig{ConsecutiveFailures: 2})

	_, err := scheduler.TriggerCheck(999)
	if err == nil {
		t.Error("expected error for non-existent check")
	}
}

func TestSchedulerUpdateCheck(t *testing.T) {
	store, server := setupSchedulerTest(t)

	// Create a check
	check := &storage.Check{
		Name:           "Update Test",
		URL:            server.URL,
		IntervalSecs:   60,
		TimeoutSecs:    5,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	scheduler := NewScheduler(store, nil, SchedulerConfig{ConsecutiveFailures: 2})

	if err := scheduler.Start(); err != nil {
		t.Fatalf("failed to start scheduler: %v", err)
	}

	if scheduler.GetCheckCount() != 1 {
		t.Errorf("expected 1 check, got %d", scheduler.GetCheckCount())
	}

	// Update check to disabled
	check.Enabled = false
	if err := scheduler.UpdateCheck(check); err != nil {
		t.Fatalf("failed to update check: %v", err)
	}

	if scheduler.GetCheckCount() != 0 {
		t.Errorf("expected 0 checks after disabling, got %d", scheduler.GetCheckCount())
	}

	// Update check back to enabled
	check.Enabled = true
	if err := scheduler.UpdateCheck(check); err != nil {
		t.Fatalf("failed to update check: %v", err)
	}

	if scheduler.GetCheckCount() != 1 {
		t.Errorf("expected 1 check after re-enabling, got %d", scheduler.GetCheckCount())
	}

	scheduler.Stop()
}
