package checker

import (
	"fmt"
	"time"

	"github.com/katieblackabee/sentinel/internal/storage"
)

// Alerter is an interface for sending alerts (to avoid circular imports)
type Alerter interface {
	SendDownAlert(check *storage.Check, incident *storage.Incident, errorMsg string) error
	SendRecoveryAlert(check *storage.Check, incident *storage.Incident) error
}

// ProcessResult handles a check response: saves result, detects state changes, manages incidents
func ProcessResult(store storage.Storage, alerter Alerter, check *storage.Check, response *CheckResponse, consecutiveFailures int) error {
	// Determine status
	status := DetermineStatus(response, check.ExpectedStatus)

	// Build result
	result := &storage.CheckResult{
		CheckID:        check.ID,
		Status:         status,
		StatusCode:     response.StatusCode,
		ResponseTimeMs: response.ResponseTimeMs,
	}
	if response.Error != nil {
		result.ErrorMessage = response.Error.Error()
	}

	// Save result
	if err := store.SaveResult(result); err != nil {
		return fmt.Errorf("saving result: %w", err)
	}

	// Get previous status to detect state change
	previousStatus := check.Status
	if previousStatus == "" || previousStatus == "pending" {
		// First check, no state change detection needed
		return nil
	}

	// Detect state changes
	if status == "down" && previousStatus == "up" {
		// UP -> DOWN transition
		shouldAlert, err := ShouldAlert(store, check.ID, consecutiveFailures)
		if err != nil {
			return fmt.Errorf("checking alert threshold: %w", err)
		}

		if shouldAlert {
			// Create incident
			incident := &storage.Incident{
				CheckID:   check.ID,
				StartedAt: time.Now(),
				Cause:     result.ErrorMessage,
			}
			if err := store.CreateIncident(incident); err != nil {
				return fmt.Errorf("creating incident: %w", err)
			}

			// Send alert
			if alerter != nil {
				if err := alerter.SendDownAlert(check, incident, result.ErrorMessage); err != nil {
					// Log but don't fail - alert failure shouldn't stop monitoring
					fmt.Printf("failed to send down alert: %v\n", err)
				}
			}
		}
	} else if status == "up" && previousStatus == "down" {
		// DOWN -> UP transition (recovery)
		incident, err := store.GetActiveIncident(check.ID)
		if err != nil {
			return fmt.Errorf("getting active incident: %w", err)
		}

		if incident != nil {
			// Close the incident
			if err := store.CloseIncident(incident.ID, time.Now()); err != nil {
				return fmt.Errorf("closing incident: %w", err)
			}

			// Reload to get duration
			incident, _ = store.GetIncident(incident.ID)

			// Send recovery alert
			if alerter != nil {
				if err := alerter.SendRecoveryAlert(check, incident); err != nil {
					fmt.Printf("failed to send recovery alert: %v\n", err)
				}
			}
		}
	}

	return nil
}

// DetermineStatus returns "up" or "down" based on the check response
func DetermineStatus(response *CheckResponse, expectedStatus int) string {
	if response.Error != nil {
		return "down"
	}
	if expectedStatus == 0 {
		expectedStatus = 200
	}
	if response.StatusCode != expectedStatus {
		return "down"
	}
	return "up"
}

// ShouldAlert checks if we have enough consecutive failures to trigger an alert
func ShouldAlert(store storage.Storage, checkID int64, threshold int) (bool, error) {
	if threshold < 1 {
		threshold = 2 // Default
	}

	// Get recent results
	results, err := store.GetRecentResults(checkID, threshold)
	if err != nil {
		return false, err
	}

	// Need at least 'threshold' results to alert
	if len(results) < threshold {
		return false, nil
	}

	// Check if all recent results are down
	for _, r := range results {
		if r.Status == "up" {
			return false, nil
		}
	}

	// Check if we already have an active incident (avoid duplicate alerts)
	incident, err := store.GetActiveIncident(checkID)
	if err != nil {
		return false, err
	}
	if incident != nil {
		return false, nil // Already have an incident, don't create another
	}

	return true, nil
}
