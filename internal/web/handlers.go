package web

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/katieblackabee/sentinel/internal/storage"
)

type DashboardData struct {
	Title           string
	BasePath        string
	AllOperational  bool
	OverallUptime   float64
	CheckGroups     map[string][]*CheckWithStatus
	RecentIncidents []*storage.Incident
	LastUpdated     time.Time
}

type CheckWithStatus struct {
	*storage.Check
	UptimePercent  float64
	Sparkline      []bool // Last 24 checks: true = up, false = down
	SSLDaysLeft    int    // Days until SSL cert expires (0 if no SSL)
	SSLExpiresDate string // Formatted expiry date
}

type CheckDetailData struct {
	Title     string
	BasePath  string
	Check     *storage.Check
	Stats     *storage.CheckStats
	Results   []*storage.CheckResult
	Incidents []*storage.Incident
	Period    string // "24h", "7d", "30d"
}

type SettingsData struct {
	Title         string
	BasePath      string
	Checks        []*storage.Check
	Message       string
	Error         string
	AlertConfig   *AlertConfigView
	RetentionConfig *RetentionConfigView
}

type AlertConfigView struct {
	ConsecutiveFailures  int
	RecoveryNotification bool
	CooldownMinutes      int
	EmailEnabled         bool
	SMTPHost             string
	SMTPPort             int
	FromAddress          string
	ToAddresses          []string
}

type RetentionConfigView struct {
	ResultsDays    int
	AggregatesDays int
}

type EditCheckData struct {
	Title    string
	BasePath string
	Check    *storage.Check
	Error    string
}

func (s *Server) HandleDashboard(c echo.Context) error {
	checks, err := s.storage.ListChecks()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load checks")
	}

	// Enrich checks with status
	checkGroups := make(map[string][]*CheckWithStatus)
	var totalUptime float64
	allUp := true

	for _, check := range checks {
		// Get latest result
		result, _ := s.storage.GetLatestResult(check.ID)
		if result != nil {
			check.Status = result.Status
			check.LastResponseMs = result.ResponseTimeMs
			check.LastCheckedAt = &result.CheckedAt
		} else {
			check.Status = "pending"
		}

		// Get stats
		stats, _ := s.storage.GetStats(check.ID)
		uptimePercent := 100.0
		if stats != nil {
			uptimePercent = stats.UptimePercent24h
		}
		totalUptime += uptimePercent

		if check.Status != "up" && check.Status != "pending" {
			allUp = false
		}

		// Get sparkline data (last 24 results)
		results, _ := s.storage.GetResults(check.ID, 24, 0)
		sparkline := make([]bool, len(results))
		for i, r := range results {
			sparkline[len(results)-1-i] = r.Status == "up"
		}

		// Get SSL info from most recent result
		var sslDaysLeft int
		var sslExpiresDate string
		if len(results) > 0 && results[0].SSLExpiresAt != nil {
			sslDaysLeft = results[0].SSLDaysLeft
			sslExpiresDate = results[0].SSLExpiresAt.Format("Jan 2, 2006")
		}

		cws := &CheckWithStatus{
			Check:          check,
			UptimePercent:  uptimePercent,
			Sparkline:      sparkline,
			SSLDaysLeft:    sslDaysLeft,
			SSLExpiresDate: sslExpiresDate,
		}

		// Group by first tag or "default"
		groupName := "default"
		if len(check.Tags) > 0 {
			groupName = check.Tags[0]
		}
		checkGroups[groupName] = append(checkGroups[groupName], cws)
	}

	// Calculate overall uptime
	overallUptime := 100.0
	if len(checks) > 0 {
		overallUptime = totalUptime / float64(len(checks))
	}

	// Get recent incidents
	incidents, _ := s.storage.ListIncidents(5, 0)

	data := DashboardData{
		Title:           "Dashboard",
		BasePath:        s.BasePath(),
		AllOperational:  allUp,
		OverallUptime:   overallUptime,
		CheckGroups:     checkGroups,
		RecentIncidents: incidents,
		LastUpdated:     time.Now(),
	}

	return c.Render(http.StatusOK, "dashboard.html", data)
}

func (s *Server) HandleCheckDetail(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.String(http.StatusBadRequest, "Invalid check ID")
	}

	check, err := s.storage.GetCheck(id)
	if err != nil || check == nil {
		return c.String(http.StatusNotFound, "Check not found")
	}

	// Get latest result for status
	result, _ := s.storage.GetLatestResult(check.ID)
	if result != nil {
		check.Status = result.Status
		check.LastResponseMs = result.ResponseTimeMs
		check.LastCheckedAt = &result.CheckedAt
	}

	// Get stats
	stats, _ := s.storage.GetStats(check.ID)

	// Get period from query param (default 24h)
	period := c.QueryParam("period")
	if period == "" {
		period = "24h"
	}

	// Calculate time range based on period
	var startTime time.Time
	now := time.Now()
	switch period {
	case "7d":
		startTime = now.Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = now.Add(-30 * 24 * time.Hour)
	default:
		period = "24h"
		startTime = now.Add(-24 * time.Hour)
	}

	// Get results for chart within time range
	results, _ := s.storage.GetResultsInRange(check.ID, startTime, now)

	// Results are already in chronological order from GetResultsInRange

	// Get incidents
	incidents, _ := s.storage.ListIncidentsForCheck(check.ID, 10)

	data := CheckDetailData{
		Title:     check.Name,
		BasePath:  s.BasePath(),
		Check:     check,
		Stats:     stats,
		Results:   results,
		Incidents: incidents,
		Period:    period,
	}

	return c.Render(http.StatusOK, "check.html", data)
}

func (s *Server) HandleSettings(c echo.Context) error {
	checks, err := s.storage.ListChecks()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load checks")
	}

	// Sort by name
	sort.Slice(checks, func(i, j int) bool {
		return checks[i].Name < checks[j].Name
	})

	// Build alert config view
	var alertConfig *AlertConfigView
	var retentionConfig *RetentionConfigView

	if s.fullConfig != nil {
		alertConfig = &AlertConfigView{
			ConsecutiveFailures:  s.fullConfig.Alerts.ConsecutiveFailures,
			RecoveryNotification: s.fullConfig.Alerts.RecoveryNotification,
			CooldownMinutes:      s.fullConfig.Alerts.CooldownMinutes,
			EmailEnabled:         s.fullConfig.Alerts.Email.Enabled,
			SMTPHost:             s.fullConfig.Alerts.Email.SMTPHost,
			SMTPPort:             s.fullConfig.Alerts.Email.SMTPPort,
			FromAddress:          s.fullConfig.Alerts.Email.FromAddress,
			ToAddresses:          s.fullConfig.Alerts.Email.ToAddresses,
		}
		retentionConfig = &RetentionConfigView{
			ResultsDays:    s.fullConfig.Retention.ResultsDays,
			AggregatesDays: s.fullConfig.Retention.AggregatesDays,
		}
	}

	data := SettingsData{
		Title:           "Settings",
		BasePath:        s.BasePath(),
		Checks:          checks,
		Message:         c.QueryParam("message"),
		Error:           c.QueryParam("error"),
		AlertConfig:     alertConfig,
		RetentionConfig: retentionConfig,
	}

	return c.Render(http.StatusOK, "settings.html", data)
}

func (s *Server) HandleCreateCheckForm(c echo.Context) error {
	name := c.FormValue("name")
	url := c.FormValue("url")
	intervalStr := c.FormValue("interval")

	if name == "" || url == "" {
		return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?error=Name+and+URL+are+required")
	}

	interval := 3600
	if intervalStr != "" {
		if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
			interval = i
		}
	}

	check := &storage.Check{
		Name:           name,
		URL:            url,
		IntervalSecs:   interval,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}

	if err := s.storage.CreateCheck(check); err != nil {
		return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?error=Failed+to+create+check")
	}

	// Add to scheduler
	if s.scheduler != nil {
		s.scheduler.AddCheck(check)
	}

	return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?message=Check+created")
}

func (s *Server) HandleDeleteCheckForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?error=Invalid+check+ID")
	}

	// Remove from scheduler first
	if s.scheduler != nil {
		s.scheduler.RemoveCheck(id)
	}

	if err := s.storage.DeleteCheck(id); err != nil {
		return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?error=Failed+to+delete+check")
	}

	return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?message=Check+deleted")
}

func (s *Server) HandleEditCheckForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?error=Invalid+check+ID")
	}

	check, err := s.storage.GetCheck(id)
	if err != nil || check == nil {
		return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?error=Check+not+found")
	}

	// Handle GET - show edit form
	if c.Request().Method == "GET" {
		data := EditCheckData{
			Title:    "Edit Check",
			BasePath: s.BasePath(),
			Check:    check,
		}
		return c.Render(http.StatusOK, "edit.html", data)
	}

	// Handle POST - process form
	check.Name = c.FormValue("name")
	check.URL = c.FormValue("url")

	if intervalStr := c.FormValue("interval"); intervalStr != "" {
		if i, err := strconv.Atoi(intervalStr); err == nil && i > 0 {
			check.IntervalSecs = i
		}
	}

	if timeoutStr := c.FormValue("timeout"); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil && t > 0 {
			check.TimeoutSecs = t
		}
	}

	if statusStr := c.FormValue("expected_status"); statusStr != "" {
		if s, err := strconv.Atoi(statusStr); err == nil && s > 0 {
			check.ExpectedStatus = s
		}
	}

	check.Enabled = c.FormValue("enabled") == "1"

	if check.Name == "" || check.URL == "" {
		data := EditCheckData{
			Title:    "Edit Check",
			BasePath: s.BasePath(),
			Check:    check,
			Error:    "Name and URL are required",
		}
		return c.Render(http.StatusOK, "edit.html", data)
	}

	if err := s.storage.UpdateCheck(check); err != nil {
		data := EditCheckData{
			Title:    "Edit Check",
			BasePath: s.BasePath(),
			Check:    check,
			Error:    "Failed to update check",
		}
		return c.Render(http.StatusOK, "edit.html", data)
	}

	// Update scheduler
	if s.scheduler != nil {
		s.scheduler.UpdateCheck(check)
	}

	return c.Redirect(http.StatusSeeOther, s.BasePath()+"/settings?message=Check+updated")
}

// StatusPageData holds data for public status pages
type StatusPageData struct {
	Title          string
	Slug           string
	AllOperational bool
	OverallUptime  float64
	Checks         []*CheckWithStatus
	LastUpdated    time.Time
}

// handleStatusPage renders a public status page for a given tag/slug
func (s *Server) handleStatusPage(c echo.Context) error {
	slug := c.Param("slug")
	if slug == "" {
		return c.String(http.StatusNotFound, "Status page not found")
	}

	checks, err := s.storage.ListChecksByTag(slug)
	if err != nil || len(checks) == 0 {
		return c.String(http.StatusNotFound, "Status page not found")
	}

	// Build status data
	var statusChecks []*CheckWithStatus
	allUp := true
	var totalUptime float64

	for _, check := range checks {
		stats, _ := s.storage.GetStats(check.ID)
		uptime := 100.0
		if stats != nil {
			uptime = stats.UptimePercent24h
		}
		totalUptime += uptime

		// Get sparkline
		results, _ := s.storage.GetRecentResults(check.ID, 24)
		sparkline := make([]bool, len(results))
		for i, r := range results {
			sparkline[i] = r.IsUp()
		}

		if check.Status != "up" {
			allUp = false
		}

		// Get SSL info from most recent result
		var sslDaysLeft int
		var sslExpiresDate string
		if len(results) > 0 && results[0].SSLExpiresAt != nil {
			sslDaysLeft = results[0].SSLDaysLeft
			sslExpiresDate = results[0].SSLExpiresAt.Format("Jan 2, 2006")
		}

		statusChecks = append(statusChecks, &CheckWithStatus{
			Check:          check,
			UptimePercent:  uptime,
			Sparkline:      sparkline,
			SSLDaysLeft:    sslDaysLeft,
			SSLExpiresDate: sslExpiresDate,
		})
	}

	overallUptime := 100.0
	if len(checks) > 0 {
		overallUptime = totalUptime / float64(len(checks))
	}

	data := StatusPageData{
		Title:          slug + " Status",
		Slug:           slug,
		AllOperational: allUp,
		OverallUptime:  overallUptime,
		Checks:         statusChecks,
		LastUpdated:    time.Now(),
	}

	return c.Render(http.StatusOK, "status.html", data)
}
