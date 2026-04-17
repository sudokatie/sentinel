package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/katieblackabee/sentinel/internal/probe"
	"github.com/katieblackabee/sentinel/internal/storage"
)

func setupProbeHandler(t *testing.T) (*ProbeHandler, *storage.SQLiteStorage) {
	store, err := storage.NewSQLiteStorage(":memory:")
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	registry := probe.NewProbeRegistry()
	coordinator := probe.NewCoordinator(registry, store)

	handler := NewProbeHandler(store, registry, coordinator)
	return handler, store
}

func TestRegisterProbe(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	input := RegisterProbeInput{
		Name:      "test-probe",
		Region:    "us-east",
		City:      "New York",
		Country:   "USA",
		Latitude:  40.7128,
		Longitude: -74.0060,
	}
	body, _ := json.Marshal(input)

	req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.RegisterProbe(c)
	if err != nil {
		t.Fatalf("RegisterProbe returned error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	// Check that the response contains probe data with api_key
	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be a map, got %T", resp.Data)
	}

	if dataMap["name"] != "test-probe" {
		t.Errorf("Expected name 'test-probe', got %v", dataMap["name"])
	}
	if dataMap["region"] != "us-east" {
		t.Errorf("Expected region 'us-east', got %v", dataMap["region"])
	}
	if dataMap["api_key"] == nil || dataMap["api_key"] == "" {
		t.Error("Expected api_key to be present")
	}
}

func TestRegisterProbeValidation(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	tests := []struct {
		name     string
		input    RegisterProbeInput
		expected string
	}{
		{
			name:     "missing name",
			input:    RegisterProbeInput{Region: "us-east"},
			expected: "name is required",
		},
		{
			name:     "missing region",
			input:    RegisterProbeInput{Name: "test-probe"},
			expected: "region is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.input)

			req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := handler.RegisterProbe(c)
			if err != nil {
				t.Fatalf("RegisterProbe returned error: %v", err)
			}

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected status %d, got %d", http.StatusBadRequest, rec.Code)
			}

			var resp APIResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			if resp.Error != tt.expected {
				t.Errorf("Expected error '%s', got '%s'", tt.expected, resp.Error)
			}
		})
	}
}

func TestDeregisterProbe(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	// First, register a probe
	input := RegisterProbeInput{
		Name:   "test-probe",
		Region: "us-east",
	}
	body, _ := json.Marshal(input)

	req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler.RegisterProbe(c)

	var regResp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &regResp)
	dataMap := regResp.Data.(map[string]interface{})
	probeID := int64(dataMap["id"].(float64))

	// Now deregister the probe
	req = httptest.NewRequest(http.MethodDelete, "/api/probes/1", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := handler.DeregisterProbe(c)
	if err != nil {
		t.Fatalf("DeregisterProbe returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	// Verify probe is deleted
	deletedProbe, _ := store.GetProbe(probeID)
	if deletedProbe != nil {
		t.Error("Expected probe to be deleted from storage")
	}
}

func TestDeregisterProbeNotFound(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	req := httptest.NewRequest(http.MethodDelete, "/api/probes/999", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("999")

	err := handler.DeregisterProbe(c)
	if err != nil {
		t.Fatalf("DeregisterProbe returned error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "Probe not found" {
		t.Errorf("Expected error 'Probe not found', got '%s'", resp.Error)
	}
}

func TestProbeHeartbeat(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	// First, register a probe
	input := RegisterProbeInput{
		Name:   "test-probe",
		Region: "us-east",
	}
	body, _ := json.Marshal(input)

	req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler.RegisterProbe(c)

	// Now send heartbeat
	req = httptest.NewRequest(http.MethodPost, "/api/probes/1/heartbeat", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := handler.ProbeHeartbeat(c)
	if err != nil {
		t.Fatalf("ProbeHeartbeat returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}
}

func TestProbeHeartbeatNotFound(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/probes/999/heartbeat", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("999")

	err := handler.ProbeHeartbeat(c)
	if err != nil {
		t.Fatalf("ProbeHeartbeat returned error: %v", err)
	}

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "Probe not found" {
		t.Errorf("Expected error 'Probe not found', got '%s'", resp.Error)
	}
}

func TestListProbes(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	// Register a couple of probes
	for _, name := range []string{"probe-1", "probe-2"} {
		input := RegisterProbeInput{
			Name:   name,
			Region: "us-east",
		}
		body, _ := json.Marshal(input)

		req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler.RegisterProbe(c)
	}

	// List probes
	req := httptest.NewRequest(http.MethodGet, "/api/probes", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListProbes(c)
	if err != nil {
		t.Fatalf("ListProbes returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	dataList, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("Expected data to be a list, got %T", resp.Data)
	}

	if len(dataList) != 2 {
		t.Errorf("Expected 2 probes, got %d", len(dataList))
	}
}

func TestSubmitProbeResult(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	// Register a probe first
	regInput := RegisterProbeInput{
		Name:   "test-probe",
		Region: "us-east",
	}
	body, _ := json.Marshal(regInput)

	req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler.RegisterProbe(c)

	// Create a check
	check := &storage.Check{
		Name:           "test-check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	// Submit a result
	resultInput := SubmitResultInput{
		CheckID:        check.ID,
		Status:         "up",
		ResponseTimeMs: 150,
		StatusCode:     200,
	}
	body, _ = json.Marshal(resultInput)

	req = httptest.NewRequest(http.MethodPost, "/api/probes/1/results", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := handler.SubmitProbeResult(c)
	if err != nil {
		t.Fatalf("SubmitProbeResult returned error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	dataMap, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected data to be a map, got %T", resp.Data)
	}

	if dataMap["status"] != "up" {
		t.Errorf("Expected status 'up', got %v", dataMap["status"])
	}
}

func TestGetProbeResults(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	// Register a probe
	regInput := RegisterProbeInput{
		Name:   "test-probe",
		Region: "us-east",
	}
	body, _ := json.Marshal(regInput)

	req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler.RegisterProbe(c)

	// Create a check
	check := &storage.Check{
		Name:           "test-check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	// Submit a few results
	for i := 0; i < 3; i++ {
		resultInput := SubmitResultInput{
			CheckID:        check.ID,
			Status:         "up",
			ResponseTimeMs: 150,
			StatusCode:     200,
		}
		body, _ = json.Marshal(resultInput)

		req = httptest.NewRequest(http.MethodPost, "/api/probes/1/results", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("1")

		handler.SubmitProbeResult(c)
	}

	// Get results
	req = httptest.NewRequest(http.MethodGet, "/api/checks/1/probe-results", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := handler.GetProbeResults(c)
	if err != nil {
		t.Fatalf("GetProbeResults returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	dataList, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("Expected data to be a list, got %T", resp.Data)
	}

	if len(dataList) != 3 {
		t.Errorf("Expected 3 results, got %d", len(dataList))
	}
}

func TestGetProbeResultsWithRegionFilter(t *testing.T) {
	handler, store := setupProbeHandler(t)
	defer store.Close()

	e := echo.New()

	// Register probes in different regions
	for _, region := range []string{"us-east", "eu-west"} {
		regInput := RegisterProbeInput{
			Name:   "probe-" + region,
			Region: region,
		}
		body, _ := json.Marshal(regInput)

		req := httptest.NewRequest(http.MethodPost, "/api/probes/register", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler.RegisterProbe(c)
	}

	// Create a check
	check := &storage.Check{
		Name:           "test-check",
		URL:            "https://example.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	// Submit results from both probes
	for probeID := 1; probeID <= 2; probeID++ {
		resultInput := SubmitResultInput{
			CheckID:        check.ID,
			Status:         "up",
			ResponseTimeMs: 150,
			StatusCode:     200,
		}
		body, _ := json.Marshal(resultInput)

		req := httptest.NewRequest(http.MethodPost, "/api/probes/1/results", bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(string(rune('0' + probeID)))

		handler.SubmitProbeResult(c)
	}

	// Get results filtered by region
	req := httptest.NewRequest(http.MethodGet, "/api/checks/1/probe-results?region=us-east", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("1")

	err := handler.GetProbeResults(c)
	if err != nil {
		t.Fatalf("GetProbeResults returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	dataList, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("Expected data to be a list, got %T", resp.Data)
	}

	// Should only have results from us-east probe
	if len(dataList) != 1 {
		t.Errorf("Expected 1 result from us-east, got %d", len(dataList))
	}
}
