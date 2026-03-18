package checker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSyntheticChecker(t *testing.T) {
	checker := NewSyntheticChecker("/tmp/screenshots")

	if checker.ScreenshotDir != "/tmp/screenshots" {
		t.Errorf("expected screenshot dir /tmp/screenshots, got %s", checker.ScreenshotDir)
	}
	if checker.NodePath != "node" {
		t.Errorf("expected node path 'node', got %s", checker.NodePath)
	}
	if checker.PlaywrightPath != "npx" {
		t.Errorf("expected playwright path 'npx', got %s", checker.PlaywrightPath)
	}
}

func TestSyntheticChecker_Execute_ScriptNotFound(t *testing.T) {
	checker := NewSyntheticChecker("")

	req := &SyntheticRequest{
		ScriptPath: "/nonexistent/script.spec.ts",
		Timeout:    10 * time.Second,
		Name:       "test-check",
	}

	response := checker.Execute(req)

	if response.Error == nil {
		t.Error("expected error for nonexistent script")
	}
	if response.Success {
		t.Error("expected failure for nonexistent script")
	}
}

func TestStepResult_Fields(t *testing.T) {
	step := StepResult{
		Name:       "Navigate to homepage",
		DurationMs: 1500,
		Status:     "passed",
	}

	if step.Name != "Navigate to homepage" {
		t.Errorf("expected name 'Navigate to homepage', got %s", step.Name)
	}
	if step.DurationMs != 1500 {
		t.Errorf("expected duration 1500, got %d", step.DurationMs)
	}
	if step.Status != "passed" {
		t.Errorf("expected status 'passed', got %s", step.Status)
	}
}

func TestStepResult_WithError(t *testing.T) {
	step := StepResult{
		Name:       "Click login button",
		DurationMs: 500,
		Status:     "failed",
		Error:      "Element not found: #login-btn",
	}

	if step.Error != "Element not found: #login-btn" {
		t.Errorf("expected error message, got %s", step.Error)
	}
}

func TestSyntheticResponse_StepSummary_Empty(t *testing.T) {
	response := &SyntheticResponse{}
	summary := response.StepSummary()

	if summary != "no steps recorded" {
		t.Errorf("expected 'no steps recorded', got %s", summary)
	}
}

func TestSyntheticResponse_StepSummary_WithSteps(t *testing.T) {
	response := &SyntheticResponse{
		Steps: []StepResult{
			{Name: "Step 1", DurationMs: 100, Status: "passed"},
			{Name: "Step 2", DurationMs: 200, Status: "failed"},
		},
	}

	summary := response.StepSummary()

	if summary == "no steps recorded" {
		t.Error("expected steps in summary")
	}
	// Should contain step names
	if !containsString(summary, "Step 1") {
		t.Error("summary should contain Step 1")
	}
	if !containsString(summary, "Step 2") {
		t.Error("summary should contain Step 2")
	}
}

func TestSyntheticChecker_findScreenshot_NoDir(t *testing.T) {
	checker := NewSyntheticChecker("")
	result := checker.findScreenshot("test")

	if result != "" {
		t.Errorf("expected empty result when no screenshot dir, got %s", result)
	}
}

func TestSyntheticChecker_findScreenshot_EmptyDir(t *testing.T) {
	// Create temp directory
	dir, err := os.MkdirTemp("", "sentinel-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	checker := NewSyntheticChecker(dir)
	result := checker.findScreenshot("test")

	if result != "" {
		t.Errorf("expected empty result for empty dir, got %s", result)
	}
}

func TestSyntheticChecker_findScreenshot_WithFiles(t *testing.T) {
	// Create temp directory with a screenshot
	dir, err := os.MkdirTemp("", "sentinel-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	// Create a fake screenshot file
	screenshotPath := filepath.Join(dir, "failure-screenshot.png")
	if err := os.WriteFile(screenshotPath, []byte("fake png"), 0644); err != nil {
		t.Fatalf("failed to create screenshot: %v", err)
	}

	checker := NewSyntheticChecker(dir)
	result := checker.findScreenshot("test")

	if result == "" {
		t.Error("expected to find screenshot")
	}
	if result != screenshotPath {
		t.Errorf("expected %s, got %s", screenshotPath, result)
	}
}

func TestSyntheticRequest_Fields(t *testing.T) {
	req := &SyntheticRequest{
		ScriptPath: "/path/to/script.spec.ts",
		Timeout:    30 * time.Second,
		Name:       "Login Flow",
	}

	if req.ScriptPath != "/path/to/script.spec.ts" {
		t.Errorf("expected script path, got %s", req.ScriptPath)
	}
	if req.Timeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", req.Timeout)
	}
	if req.Name != "Login Flow" {
		t.Errorf("expected 'Login Flow', got %s", req.Name)
	}
}

func TestSyntheticResponse_Success(t *testing.T) {
	response := &SyntheticResponse{
		Success:         true,
		TotalDurationMs: 5000,
		Steps: []StepResult{
			{Name: "Navigate", DurationMs: 2000, Status: "passed"},
			{Name: "Login", DurationMs: 3000, Status: "passed"},
		},
	}

	if !response.Success {
		t.Error("expected success")
	}
	if response.TotalDurationMs != 5000 {
		t.Errorf("expected 5000ms, got %d", response.TotalDurationMs)
	}
	if len(response.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(response.Steps))
	}
}

func TestSyntheticResponse_Failure(t *testing.T) {
	response := &SyntheticResponse{
		Success:         false,
		TotalDurationMs: 3000,
		ScreenshotPath:  "/tmp/failure.png",
		Steps: []StepResult{
			{Name: "Navigate", DurationMs: 1000, Status: "passed"},
			{Name: "Login", DurationMs: 2000, Status: "failed", Error: "timeout"},
		},
	}

	if response.Success {
		t.Error("expected failure")
	}
	if response.ScreenshotPath != "/tmp/failure.png" {
		t.Errorf("expected screenshot path, got %s", response.ScreenshotPath)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
