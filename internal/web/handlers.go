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
	Title          string
	AllOperational bool
	OverallUptime  float64
	CheckGroups    map[string][]*CheckWithStatus
	RecentIncidents []*storage.Incident
	LastUpdated    time.Time
}

type CheckWithStatus struct {
	*storage.Check
	UptimePercent float64
	Sparkline     []bool // Last 24 checks: true = up, false = down
}

type CheckDetailData struct {
	Title     string
	Check     *storage.Check
	Stats     *storage.CheckStats
	Results   []*storage.CheckResult
	Incidents []*storage.Incident
}

type SettingsData struct {
	Title   string
	Checks  []*storage.Check
	Message string
	Error   string
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

		cws := &CheckWithStatus{
			Check:         check,
			UptimePercent: uptimePercent,
			Sparkline:     sparkline,
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

	// Get results for chart (last 100)
	results, _ := s.storage.GetResults(check.ID, 100, 0)

	// Reverse results for chronological order
	for i, j := 0, len(results)-1; i < j; i, j = i+1, j-1 {
		results[i], results[j] = results[j], results[i]
	}

	// Get incidents
	incidents, _ := s.storage.ListIncidentsForCheck(check.ID, 10)

	data := CheckDetailData{
		Title:     check.Name,
		Check:     check,
		Stats:     stats,
		Results:   results,
		Incidents: incidents,
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

	data := SettingsData{
		Title:   "Settings",
		Checks:  checks,
		Message: c.QueryParam("message"),
		Error:   c.QueryParam("error"),
	}

	return c.Render(http.StatusOK, "settings.html", data)
}

func (s *Server) HandleCreateCheckForm(c echo.Context) error {
	name := c.FormValue("name")
	url := c.FormValue("url")
	intervalStr := c.FormValue("interval")

	if name == "" || url == "" {
		return c.Redirect(http.StatusSeeOther, "/settings?error=Name+and+URL+are+required")
	}

	interval := 60
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
		return c.Redirect(http.StatusSeeOther, "/settings?error=Failed+to+create+check")
	}

	// Add to scheduler
	if s.scheduler != nil {
		s.scheduler.AddCheck(check)
	}

	return c.Redirect(http.StatusSeeOther, "/settings?message=Check+created")
}

func (s *Server) HandleDeleteCheckForm(c echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/settings?error=Invalid+check+ID")
	}

	// Remove from scheduler first
	if s.scheduler != nil {
		s.scheduler.RemoveCheck(id)
	}

	if err := s.storage.DeleteCheck(id); err != nil {
		return c.Redirect(http.StatusSeeOther, "/settings?error=Failed+to+delete+check")
	}

	return c.Redirect(http.StatusSeeOther, "/settings?message=Check+deleted")
}
