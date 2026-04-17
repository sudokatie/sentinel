package web

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/katieblackabee/sentinel/internal/probe"
	"github.com/katieblackabee/sentinel/internal/storage"
)

// ProbeHandler handles probe-related HTTP requests.
type ProbeHandler struct {
	store       storage.Storage
	registry    *probe.ProbeRegistry
	coordinator *probe.Coordinator
}

// NewProbeHandler creates a new ProbeHandler.
func NewProbeHandler(store storage.Storage, registry *probe.ProbeRegistry, coord *probe.Coordinator) *ProbeHandler {
	return &ProbeHandler{
		store:       store,
		registry:    registry,
		coordinator: coord,
	}
}

// RegisterProbeInput is the JSON input for probe registration.
type RegisterProbeInput struct {
	Name      string  `json:"name"`
	Region    string  `json:"region"`
	City      string  `json:"city"`
	Country   string  `json:"country"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ProbeResponse is the JSON response for a probe.
type ProbeResponse struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Region        string     `json:"region"`
	City          string     `json:"city,omitempty"`
	Country       string     `json:"country,omitempty"`
	Latitude      float64    `json:"latitude,omitempty"`
	Longitude     float64    `json:"longitude,omitempty"`
	APIKey        string     `json:"api_key,omitempty"`
	Status        string     `json:"status"`
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
}

// SubmitResultInput is the JSON input for submitting probe results.
type SubmitResultInput struct {
	CheckID        int64  `json:"check_id"`
	Status         string `json:"status"`
	ResponseTimeMs int64  `json:"response_time_ms"`
	StatusCode     int64  `json:"status_code"`
	Error          string `json:"error"`
}

// ProbeResultResponse is the JSON response for a probe result.
type ProbeResultResponse struct {
	ID             int64     `json:"id"`
	CheckID        int64     `json:"check_id"`
	ProbeID        int64     `json:"probe_id"`
	Status         string    `json:"status"`
	ResponseTimeMs *int64    `json:"response_time_ms,omitempty"`
	StatusCode     *int64    `json:"status_code,omitempty"`
	Error          string    `json:"error,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
}

// RegisterProbe handles POST /api/probes/register
func (h *ProbeHandler) RegisterProbe(c echo.Context) error {
	var input RegisterProbeInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid request body"})
	}

	if input.Name == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "name is required"})
	}
	if input.Region == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "region is required"})
	}

	// Register in the registry (generates API key)
	probeInfo, err := h.registry.Register(input.Name, input.Region, input.City, input.Country, input.Latitude, input.Longitude)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	// Save to storage
	storageProbe := &storage.Probe{
		ID:      probeInfo.ID,
		Name:    probeInfo.Name,
		Region:  probeInfo.Region,
		City:    sql.NullString{String: probeInfo.City, Valid: probeInfo.City != ""},
		Country: sql.NullString{String: probeInfo.Country, Valid: probeInfo.Country != ""},
		Latitude: sql.NullFloat64{Float64: probeInfo.Latitude, Valid: probeInfo.Latitude != 0},
		Longitude: sql.NullFloat64{Float64: probeInfo.Longitude, Valid: probeInfo.Longitude != 0},
		APIKey:  probeInfo.APIKey,
		Status:  probeInfo.Status,
	}

	if err := h.store.CreateProbe(storageProbe); err != nil {
		// Rollback registry registration
		h.registry.Deregister(probeInfo.ID)
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	response := ProbeResponse{
		ID:        storageProbe.ID,
		Name:      storageProbe.Name,
		Region:    storageProbe.Region,
		City:      probeInfo.City,
		Country:   probeInfo.Country,
		Latitude:  probeInfo.Latitude,
		Longitude: probeInfo.Longitude,
		APIKey:    probeInfo.APIKey,
		Status:    storageProbe.Status,
	}

	return c.JSON(http.StatusCreated, APIResponse{Data: response})
}

// DeregisterProbe handles DELETE /api/probes/:id
func (h *ProbeHandler) DeregisterProbe(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid probe ID"})
	}

	// Check if probe exists in storage
	existingProbe, err := h.store.GetProbe(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}
	if existingProbe == nil {
		return c.JSON(http.StatusNotFound, APIResponse{Error: "Probe not found"})
	}

	// Remove from registry
	h.registry.Deregister(id)

	// Remove from storage
	if err := h.store.DeleteProbe(id); err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, APIResponse{Data: map[string]bool{"deleted": true}})
}

// ProbeHeartbeat handles POST /api/probes/:id/heartbeat
func (h *ProbeHandler) ProbeHeartbeat(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid probe ID"})
	}

	// Check if probe exists in storage
	existingProbe, err := h.store.GetProbe(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}
	if existingProbe == nil {
		return c.JSON(http.StatusNotFound, APIResponse{Error: "Probe not found"})
	}

	// Update heartbeat in registry
	if err := h.registry.Heartbeat(id); err != nil && err != probe.ErrProbeNotFound {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	// Update heartbeat in storage
	if err := h.store.UpdateProbeHeartbeat(id); err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, APIResponse{Data: map[string]string{"status": "ok"}})
}

// ListProbes handles GET /api/probes
func (h *ProbeHandler) ListProbes(c echo.Context) error {
	probes, err := h.store.ListActiveProbes()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	responses := make([]ProbeResponse, 0, len(probes))
	for _, p := range probes {
		resp := ProbeResponse{
			ID:     p.ID,
			Name:   p.Name,
			Region: p.Region,
			Status: p.Status,
		}
		if p.City.Valid {
			resp.City = p.City.String
		}
		if p.Country.Valid {
			resp.Country = p.Country.String
		}
		if p.Latitude.Valid {
			resp.Latitude = p.Latitude.Float64
		}
		if p.Longitude.Valid {
			resp.Longitude = p.Longitude.Float64
		}
		if p.LastHeartbeat.Valid {
			resp.LastHeartbeat = &p.LastHeartbeat.Time
		}
		if p.CreatedAt.Valid {
			resp.CreatedAt = &p.CreatedAt.Time
		}
		responses = append(responses, resp)
	}

	return c.JSON(http.StatusOK, APIResponse{Data: responses})
}

// SubmitProbeResult handles POST /api/probes/:id/results
func (h *ProbeHandler) SubmitProbeResult(c echo.Context) error {
	probeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid probe ID"})
	}

	var input SubmitResultInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "Invalid request body"})
	}

	if input.CheckID == 0 {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "check_id is required"})
	}
	if input.Status == "" {
		return c.JSON(http.StatusBadRequest, APIResponse{Error: "status is required"})
	}

	result := &storage.ProbeResult{
		CheckID:        input.CheckID,
		ProbeID:        probeID,
		Status:         input.Status,
		ResponseTimeMs: sql.NullInt64{Int64: input.ResponseTimeMs, Valid: input.ResponseTimeMs > 0},
		StatusCode:     sql.NullInt64{Int64: input.StatusCode, Valid: input.StatusCode > 0},
		Error:          sql.NullString{String: input.Error, Valid: input.Error != ""},
		CheckedAt:      time.Now(),
	}

	if err := h.store.SaveProbeResult(result); err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	response := ProbeResultResponse{
		ID:        result.ID,
		CheckID:   result.CheckID,
		ProbeID:   result.ProbeID,
		Status:    result.Status,
		CheckedAt: result.CheckedAt,
	}
	if result.ResponseTimeMs.Valid {
		response.ResponseTimeMs = &result.ResponseTimeMs.Int64
	}
	if result.StatusCode.Valid {
		response.StatusCode = &result.StatusCode.Int64
	}
	if result.Error.Valid {
		response.Error = result.Error.String
	}

	return c.JSON(http.StatusCreated, APIResponse{Data: response})
}

// GetProbeResults handles GET /api/checks/:id/probe-results
func (h *ProbeHandler) GetProbeResults(c echo.Context) error {
	checkID, err := strconv.ParseInt(c.Param("id"), 10, 64)
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

	regionFilter := c.QueryParam("region")

	results, err := h.store.GetProbeResults(checkID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
	}

	// If region filter is specified, we need to filter by probe region
	if regionFilter != "" {
		// Get probe info for region mapping
		probes, err := h.store.ListProbes()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, APIResponse{Error: err.Error()})
		}

		probeRegions := make(map[int64]string)
		for _, p := range probes {
			probeRegions[p.ID] = p.Region
		}

		var filteredResults []*storage.ProbeResult
		for _, r := range results {
			if probeRegions[r.ProbeID] == regionFilter {
				filteredResults = append(filteredResults, r)
			}
		}
		results = filteredResults
	}

	responses := make([]ProbeResultResponse, 0, len(results))
	for _, r := range results {
		resp := ProbeResultResponse{
			ID:        r.ID,
			CheckID:   r.CheckID,
			ProbeID:   r.ProbeID,
			Status:    r.Status,
			CheckedAt: r.CheckedAt,
		}
		if r.ResponseTimeMs.Valid {
			resp.ResponseTimeMs = &r.ResponseTimeMs.Int64
		}
		if r.StatusCode.Valid {
			resp.StatusCode = &r.StatusCode.Int64
		}
		if r.Error.Valid {
			resp.Error = r.Error.String
		}
		responses = append(responses, resp)
	}

	return c.JSON(http.StatusOK, APIResponse{Data: responses})
}
