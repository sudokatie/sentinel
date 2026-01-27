package storage

import (
	"testing"
	"time"
)

func TestCheckIsUp(t *testing.T) {
	check := &Check{Status: "up"}
	if !check.IsUp() {
		t.Error("expected IsUp() to return true")
	}

	check.Status = "down"
	if check.IsUp() {
		t.Error("expected IsUp() to return false")
	}
}

func TestCheckIsDown(t *testing.T) {
	check := &Check{Status: "down"}
	if !check.IsDown() {
		t.Error("expected IsDown() to return true")
	}

	check.Status = "up"
	if check.IsDown() {
		t.Error("expected IsDown() to return false")
	}
}

func TestCheckIsPending(t *testing.T) {
	check := &Check{Status: "pending"}
	if !check.IsPending() {
		t.Error("expected IsPending() to return true for 'pending'")
	}

	check.Status = ""
	if !check.IsPending() {
		t.Error("expected IsPending() to return true for empty string")
	}

	check.Status = "up"
	if check.IsPending() {
		t.Error("expected IsPending() to return false for 'up'")
	}
}

func TestCheckResultIsUp(t *testing.T) {
	result := &CheckResult{Status: "up"}
	if !result.IsUp() {
		t.Error("expected IsUp() to return true")
	}

	result.Status = "down"
	if result.IsUp() {
		t.Error("expected IsUp() to return false")
	}
}

func TestIncidentIsActive(t *testing.T) {
	incident := &Incident{EndedAt: nil}
	if !incident.IsActive() {
		t.Error("expected IsActive() to return true when EndedAt is nil")
	}

	now := time.Now()
	incident.EndedAt = &now
	if incident.IsActive() {
		t.Error("expected IsActive() to return false when EndedAt is set")
	}
}

func TestIncidentDuration(t *testing.T) {
	// Test with DurationSeconds set
	incident := &Incident{DurationSeconds: 300}
	d := incident.Duration()
	if d != 300*time.Second {
		t.Errorf("expected 300s, got %v", d)
	}

	// Test with EndedAt set but not DurationSeconds
	start := time.Now().Add(-10 * time.Minute)
	end := time.Now()
	incident = &Incident{
		StartedAt:       start,
		EndedAt:         &end,
		DurationSeconds: 0,
	}
	d = incident.Duration()
	// Should be approximately 10 minutes
	if d < 9*time.Minute || d > 11*time.Minute {
		t.Errorf("expected ~10m, got %v", d)
	}

	// Test active incident (no EndedAt)
	incident = &Incident{
		StartedAt:       time.Now().Add(-5 * time.Minute),
		EndedAt:         nil,
		DurationSeconds: 0,
	}
	d = incident.Duration()
	// Should be approximately 5 minutes
	if d < 4*time.Minute || d > 6*time.Minute {
		t.Errorf("expected ~5m, got %v", d)
	}
}

func TestIncidentDurationString(t *testing.T) {
	// Less than a minute
	incident := &Incident{DurationSeconds: 45}
	s := incident.DurationString()
	if s != "45s" {
		t.Errorf("expected 45s, got %s", s)
	}

	// Minutes
	incident = &Incident{DurationSeconds: 300}
	s = incident.DurationString()
	if s != "5m0s" {
		t.Errorf("expected 5m0s, got %s", s)
	}

	// Hours
	incident = &Incident{DurationSeconds: 7200}
	s = incident.DurationString()
	if s != "2h0m0s" {
		t.Errorf("expected 2h0m0s, got %s", s)
	}
}

func TestCreateCheckInputToCheck(t *testing.T) {
	// Test defaults
	input := &CreateCheckInput{
		Name: "Test",
		URL:  "https://test.com",
	}

	check := input.ToCheck()

	if check.Name != "Test" {
		t.Errorf("expected name Test, got %s", check.Name)
	}
	if check.URL != "https://test.com" {
		t.Errorf("expected URL https://test.com, got %s", check.URL)
	}
	if check.IntervalSecs != 3600 {
		t.Errorf("expected default interval 3600, got %d", check.IntervalSecs)
	}
	if check.TimeoutSecs != 10 {
		t.Errorf("expected default timeout 10, got %d", check.TimeoutSecs)
	}
	if check.ExpectedStatus != 200 {
		t.Errorf("expected default status 200, got %d", check.ExpectedStatus)
	}
	if !check.Enabled {
		t.Error("expected enabled to default to true")
	}

	// Test with custom values
	enabled := false
	input = &CreateCheckInput{
		Name:           "Custom",
		URL:            "https://custom.com",
		IntervalSecs:   30,
		TimeoutSecs:    5,
		ExpectedStatus: 201,
		Enabled:        &enabled,
		Tags:           []string{"tag1", "tag2"},
	}

	check = input.ToCheck()

	if check.IntervalSecs != 30 {
		t.Errorf("expected interval 30, got %d", check.IntervalSecs)
	}
	if check.TimeoutSecs != 5 {
		t.Errorf("expected timeout 5, got %d", check.TimeoutSecs)
	}
	if check.ExpectedStatus != 201 {
		t.Errorf("expected status 201, got %d", check.ExpectedStatus)
	}
	if check.Enabled {
		t.Error("expected enabled to be false")
	}
	if len(check.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(check.Tags))
	}
}

func TestCreateCheckInputToCheckEnabledTrue(t *testing.T) {
	enabled := true
	input := &CreateCheckInput{
		Name:    "Test",
		URL:     "https://test.com",
		Enabled: &enabled,
	}

	check := input.ToCheck()

	if !check.Enabled {
		t.Error("expected enabled to be true")
	}
}
