package storage

import (
	"time"
)

type Check struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	IntervalSecs   int       `json:"interval_seconds"`
	TimeoutSecs    int       `json:"timeout_seconds"`
	ExpectedStatus int       `json:"expected_status"`
	Enabled        bool      `json:"enabled"`
	Tags           []string  `json:"tags"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// Computed fields (not stored in DB)
	Status         string     `json:"status"`
	LastResponseMs int        `json:"last_response_ms"`
	LastCheckedAt  *time.Time `json:"last_checked_at,omitempty"`
}

func (c *Check) IsUp() bool {
	return c.Status == "up"
}

func (c *Check) IsDown() bool {
	return c.Status == "down"
}

func (c *Check) IsPending() bool {
	return c.Status == "pending" || c.Status == ""
}

type CheckResult struct {
	ID             int64     `json:"id"`
	CheckID        int64     `json:"check_id"`
	Status         string    `json:"status"` // "up" or "down"
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int       `json:"response_time_ms"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}

func (r *CheckResult) IsUp() bool {
	return r.Status == "up"
}

type Incident struct {
	ID              int64      `json:"id"`
	CheckID         int64      `json:"check_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationSeconds int        `json:"duration_seconds"`
	Cause           string     `json:"cause,omitempty"`

	// Joined field
	CheckName string `json:"check_name,omitempty"`
}

func (i *Incident) IsActive() bool {
	return i.EndedAt == nil
}

func (i *Incident) Duration() time.Duration {
	if i.DurationSeconds > 0 {
		return time.Duration(i.DurationSeconds) * time.Second
	}
	if i.EndedAt != nil {
		return i.EndedAt.Sub(i.StartedAt)
	}
	return time.Since(i.StartedAt)
}

func (i *Incident) DurationString() string {
	d := i.Duration()
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		return d.Round(time.Minute).String()
	}
	return d.Round(time.Hour).String()
}

type AlertLog struct {
	ID           int64     `json:"id"`
	IncidentID   int64     `json:"incident_id"`
	Channel      string    `json:"channel"`
	SentAt       time.Time `json:"sent_at"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

type CheckStats struct {
	UptimePercent24h float64 `json:"uptime_percent_24h"`
	UptimePercent7d  float64 `json:"uptime_percent_7d"`
	UptimePercent30d float64 `json:"uptime_percent_30d"`
	AvgResponseMs24h int     `json:"avg_response_ms_24h"`
	AvgResponseMs7d  int     `json:"avg_response_ms_7d"`
	AvgResponseMs30d int     `json:"avg_response_ms_30d"`
}

// CreateCheckInput is used for creating new checks via API
type CreateCheckInput struct {
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	IntervalSecs   int      `json:"interval_seconds,omitempty"`
	TimeoutSecs    int      `json:"timeout_seconds,omitempty"`
	ExpectedStatus int      `json:"expected_status,omitempty"`
	Enabled        *bool    `json:"enabled,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

func (i *CreateCheckInput) ToCheck() *Check {
	enabled := true
	if i.Enabled != nil {
		enabled = *i.Enabled
	}

	intervalSecs := 3600
	if i.IntervalSecs > 0 {
		intervalSecs = i.IntervalSecs
	}

	timeoutSecs := 10
	if i.TimeoutSecs > 0 {
		timeoutSecs = i.TimeoutSecs
	}

	expectedStatus := 200
	if i.ExpectedStatus > 0 {
		expectedStatus = i.ExpectedStatus
	}

	return &Check{
		Name:           i.Name,
		URL:            i.URL,
		IntervalSecs:   intervalSecs,
		TimeoutSecs:    timeoutSecs,
		ExpectedStatus: expectedStatus,
		Enabled:        enabled,
		Tags:           i.Tags,
	}
}
