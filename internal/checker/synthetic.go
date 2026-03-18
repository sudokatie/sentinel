package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SyntheticChecker runs Playwright scripts as health checks.
type SyntheticChecker struct {
	// ScreenshotDir is where failure screenshots are saved.
	ScreenshotDir string
	// NodePath is the path to node binary (default: "node").
	NodePath string
	// PlaywrightPath is the path to playwright (default: "npx playwright").
	PlaywrightPath string
}

// SyntheticRequest defines a synthetic check to run.
type SyntheticRequest struct {
	// ScriptPath is the path to the Playwright test script.
	ScriptPath string
	// Timeout for the entire script execution.
	Timeout time.Duration
	// Name is a friendly name for the check.
	Name string
}

// StepResult represents timing for a single step in the script.
type StepResult struct {
	Name       string `json:"name"`
	DurationMs int    `json:"duration_ms"`
	Status     string `json:"status"` // "passed", "failed", "skipped"
	Error      string `json:"error,omitempty"`
}

// SyntheticResponse contains results from a synthetic check.
type SyntheticResponse struct {
	// Success is true if all steps passed.
	Success bool
	// TotalDurationMs is the total script execution time.
	TotalDurationMs int
	// Steps contains timing for each step.
	Steps []StepResult
	// ScreenshotPath is set if a failure screenshot was taken.
	ScreenshotPath string
	// Error contains any execution error.
	Error error
	// Output is the raw stdout from Playwright.
	Output string
}

// NewSyntheticChecker creates a new synthetic checker.
func NewSyntheticChecker(screenshotDir string) *SyntheticChecker {
	return &SyntheticChecker{
		ScreenshotDir:  screenshotDir,
		NodePath:       "node",
		PlaywrightPath: "npx",
	}
}

// Execute runs a Playwright script and returns the results.
func (s *SyntheticChecker) Execute(req *SyntheticRequest) *SyntheticResponse {
	response := &SyntheticResponse{}

	// Validate script exists
	if _, err := os.Stat(req.ScriptPath); os.IsNotExist(err) {
		response.Error = fmt.Errorf("script not found: %s", req.ScriptPath)
		return response
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()

	// Build the command
	// Use Playwright Test CLI with JSON reporter for structured output
	args := []string{
		"playwright", "test",
		req.ScriptPath,
		"--reporter=json",
	}

	// Add screenshot directory if configured
	if s.ScreenshotDir != "" {
		args = append(args, "--output", s.ScreenshotDir)
	}

	cmd := exec.CommandContext(ctx, s.PlaywrightPath, args...)

	start := time.Now()
	output, cmdErr := cmd.CombinedOutput()
	elapsed := time.Since(start)

	response.TotalDurationMs = int(elapsed.Milliseconds())
	response.Output = string(output)

	// Parse the JSON output
	if parseErr := s.parseOutput(response, output); parseErr != nil {
		// If parsing fails, check if the command itself failed
		if ctx.Err() == context.DeadlineExceeded {
			response.Error = fmt.Errorf("timeout after %v", req.Timeout)
		} else if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			response.Error = fmt.Errorf("script failed with exit code %d", exitErr.ExitCode())
		} else if cmdErr != nil {
			response.Error = cmdErr
		}
	}

	// Take screenshot on failure
	if !response.Success && s.ScreenshotDir != "" {
		screenshotPath := s.findScreenshot(req.Name)
		if screenshotPath != "" {
			response.ScreenshotPath = screenshotPath
		}
	}

	return response
}

// parseOutput extracts step results from Playwright JSON output.
func (s *SyntheticChecker) parseOutput(response *SyntheticResponse, output []byte) error {
	// Try to find JSON in the output (Playwright mixes JSON with other text)
	jsonStart := strings.Index(string(output), "{")
	if jsonStart == -1 {
		return fmt.Errorf("no JSON found in output")
	}

	jsonData := output[jsonStart:]

	// Parse Playwright JSON reporter format
	var result struct {
		Suites []struct {
			Specs []struct {
				Title string `json:"title"`
				Tests []struct {
					Results []struct {
						Status   string `json:"status"`
						Duration int    `json:"duration"`
						Error    *struct {
							Message string `json:"message"`
						} `json:"error"`
						Steps []struct {
							Title    string `json:"title"`
							Duration int    `json:"duration"`
						} `json:"steps"`
					} `json:"results"`
				} `json:"tests"`
			} `json:"specs"`
		} `json:"suites"`
		Stats struct {
			Expected   int `json:"expected"`
			Unexpected int `json:"unexpected"`
		} `json:"stats"`
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		// Fallback: treat as simple pass/fail based on exit code
		return err
	}

	response.Success = result.Stats.Unexpected == 0

	// Extract step timings
	for _, suite := range result.Suites {
		for _, spec := range suite.Specs {
			for _, test := range spec.Tests {
				for _, testResult := range test.Results {
					// Add test-level result
					step := StepResult{
						Name:       spec.Title,
						DurationMs: testResult.Duration,
						Status:     testResult.Status,
					}
					if testResult.Error != nil {
						step.Error = testResult.Error.Message
					}
					response.Steps = append(response.Steps, step)

					// Add individual steps if available
					for _, s := range testResult.Steps {
						response.Steps = append(response.Steps, StepResult{
							Name:       s.Title,
							DurationMs: s.Duration,
							Status:     "passed", // Steps don't have status in output
						})
					}
				}
			}
		}
	}

	return nil
}

// findScreenshot looks for a screenshot in the output directory.
func (s *SyntheticChecker) findScreenshot(checkName string) string {
	if s.ScreenshotDir == "" {
		return ""
	}

	// Look for PNG files in the screenshot directory
	pattern := filepath.Join(s.ScreenshotDir, "*.png")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}

	// Return the most recent screenshot
	var mostRecent string
	var mostRecentTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.ModTime().After(mostRecentTime) {
			mostRecentTime = info.ModTime()
			mostRecent = match
		}
	}

	return mostRecent
}

// IsPlaywrightAvailable checks if Playwright is installed and accessible.
func (s *SyntheticChecker) IsPlaywrightAvailable() bool {
	cmd := exec.Command(s.PlaywrightPath, "playwright", "--version")
	err := cmd.Run()
	return err == nil
}

// StepSummary returns a formatted summary of step timings.
func (r *SyntheticResponse) StepSummary() string {
	if len(r.Steps) == 0 {
		return "no steps recorded"
	}

	var lines []string
	for _, step := range r.Steps {
		status := "✓"
		if step.Status == "failed" {
			status = "✗"
		}
		lines = append(lines, fmt.Sprintf("%s %s (%dms)", status, step.Name, step.DurationMs))
	}
	return strings.Join(lines, "\n")
}
