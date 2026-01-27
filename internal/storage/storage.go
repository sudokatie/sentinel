package storage

import (
	"time"
)

// Storage defines the interface for data persistence
type Storage interface {
	// Checks
	CreateCheck(check *Check) error
	GetCheck(id int64) (*Check, error)
	GetCheckByURL(url string) (*Check, error)
	ListChecks() ([]*Check, error)
	ListEnabledChecks() ([]*Check, error)
	UpdateCheck(check *Check) error
	DeleteCheck(id int64) error

	// Check Results
	SaveResult(result *CheckResult) error
	GetResults(checkID int64, limit int, offset int) ([]*CheckResult, error)
	GetLatestResult(checkID int64) (*CheckResult, error)
	GetResultsInRange(checkID int64, start, end time.Time) ([]*CheckResult, error)
	GetRecentResults(checkID int64, count int) ([]*CheckResult, error)
	GetStats(checkID int64) (*CheckStats, error)

	// Incidents
	CreateIncident(incident *Incident) error
	GetIncident(id int64) (*Incident, error)
	GetActiveIncident(checkID int64) (*Incident, error)
	CloseIncident(id int64, endedAt time.Time) error
	ListIncidents(limit int, offset int) ([]*Incident, error)
	ListIncidentsForCheck(checkID int64, limit int) ([]*Incident, error)

	// Alert Log
	LogAlert(log *AlertLog) error
	GetLastAlertForIncident(incidentID int64, channel string) (*AlertLog, error)

	// Maintenance
	CleanupOldResults(olderThan time.Time) error
	Close() error
}
