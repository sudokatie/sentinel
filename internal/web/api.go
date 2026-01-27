package web

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/katieblackabee/sentinel/internal/storage"
)

type APIResponse struct {
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (s *Server) HandleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) HandleListChecks(c echo.Context) error {
	checks, err := s.storage.ListChecks()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	// Enrich with latest status
	for _, check := range checks {
		result, _ := s.storage.GetLatestResult(check.ID)
		if result != nil {
			check.Status = result.Status
			check.LastResponseMs = result.ResponseTimeMs
			check.LastCheckedAt = &result.CheckedAt
		} else {
			check.Status = "pending"
		}
	}

	return c.JSON(http.StatusOK, APIResponse{Data: checks})
}

func (s *Server) HandleCreateCheck(c echo.Context) error {
	var input storage.CreateCheckInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid request body"})
	}

	if input.Name == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "name is required"})
	}
	if input.URL == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "url is required"})
	}

	check := input.ToCheck()

	if err := s.storage.CreateCheck(check); err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	// Add to scheduler
	if s.scheduler != nil {
		s.scheduler.AddCheck(check)
	}

	check.Status = "pending"
	return c.JSON(http.StatusCreated, APIResponse{Data: check})
}

func (s *Server) HandleGetCheck(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid check ID"})
	}

	check, err := s.storage.GetCheck(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}
	if check == nil {
		return c.JSON(http.StatusNotFound, APIResponse{Error: "Check not found"})
	}

	// Enrich with status
	result, _ := s.storage.GetLatestResult(check.ID)
	if result != nil {
		check.Status = result.Status
		check.LastResponseMs = result.ResponseTimeMs
		check.LastCheckedAt = &result.CheckedAt
	} else {
		check.Status = "pending"
	}

	return c.JSON(http.StatusOK, APIResponse{Data: check})
}

func (s *Server) HandleUpdateCheck(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid check ID"})
	}

	existing, err := s.storage.GetCheck(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}
	if existing == nil {
		return c.JSON(http.StatusNotFound, APIResponse{Error: "Check not found"})
	}

	var input storage.CreateCheckInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid request body"})
	}

	// Update fields
	if input.Name != "" {
		existing.Name = input.Name
	}
	if input.URL != "" {
		existing.URL = input.URL
	}
	if input.IntervalSecs > 0 {
		existing.IntervalSecs = input.IntervalSecs
	}
	if input.TimeoutSecs > 0 {
		existing.TimeoutSecs = input.TimeoutSecs
	}
	if input.ExpectedStatus > 0 {
		existing.ExpectedStatus = input.ExpectedStatus
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.Tags != nil {
		existing.Tags = input.Tags
	}

	if err := s.storage.UpdateCheck(existing); err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	// Update scheduler
	if s.scheduler != nil {
		s.scheduler.UpdateCheck(existing)
	}

	return c.JSON(http.StatusOK, APIResponse{Data: existing})
}

func (s *Server) HandleDeleteCheck(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid check ID"})
	}

	// Remove from scheduler first
	if s.scheduler != nil {
		s.scheduler.RemoveCheck(id)
	}

	if err := s.storage.DeleteCheck(id); err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, APIResponse{Data: map[string]bool{"deleted": true}})
}

func (s *Server) HandleGetCheckResults(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid check ID"})
	}

	limit := 50
	offset := 0

	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 1000 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	results, err := s.storage.GetResults(id, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, APIResponse{Data: results})
}

func (s *Server) HandleGetCheckStats(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid check ID"})
	}

	stats, err := s.storage.GetStats(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, APIResponse{Data: stats})
}

func (s *Server) HandleTriggerCheck(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid check ID"})
	}

	if s.scheduler == nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: "Scheduler not available"})
	}

	resp, err := s.scheduler.TriggerCheck(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	status := "up"
	if resp.Error != nil || resp.StatusCode != 200 {
		status = "down"
	}

	result := map[string]interface{}{
		"status":           status,
		"status_code":      resp.StatusCode,
		"response_time_ms": resp.ResponseTimeMs,
	}
	if resp.Error != nil {
		result["error"] = resp.Error.Error()
	}

	return c.JSON(http.StatusOK, APIResponse{Data: result})
}

func (s *Server) HandleListIncidents(c echo.Context) error {
	limit := 20
	offset := 0

	if l := c.QueryParam("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}
	if o := c.QueryParam("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	incidents, err := s.storage.ListIncidents(limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, APIResponse{Data: incidents})
}

func (s *Server) HandleGetIncident(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid incident ID"})
	}

	incident, err := s.storage.GetIncident(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}
	if incident == nil {
		return c.JSON(http.StatusNotFound, APIResponse{Error: "Incident not found"})
	}

	return c.JSON(http.StatusOK, APIResponse{Data: incident})
}
