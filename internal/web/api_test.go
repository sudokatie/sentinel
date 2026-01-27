package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, storage.Storage) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	cfg := &config.ServerConfig{
		Host: "localhost",
		Port: 3000,
	}

	server := &Server{
		echo:    echo.New(),
		config:  cfg,
		storage: store,
	}
	server.registerRoutes()

	t.Cleanup(func() {
		store.Close()
		os.Remove(dbPath)
	})

	return server, store
}

func TestAPIHealth(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %s", resp["status"])
	}
}

func TestAPIListChecksEmpty(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/checks", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestAPIListChecksWithData(t *testing.T) {
	server, store := setupTestServer(t)

	// Create some checks
	checks := []*storage.Check{
		{Name: "Check 1", URL: "https://check1.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true},
		{Name: "Check 2", URL: "https://check2.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true},
	}
	for _, c := range checks {
		store.CreateCheck(c)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/checks", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be array")
	}
	if len(data) != 2 {
		t.Errorf("expected 2 checks, got %d", len(data))
	}
}

func TestAPICreateCheck(t *testing.T) {
	server, _ := setupTestServer(t)

	body := `{"name":"Test Check","url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/checks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp APIResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestAPICreateCheckWithOptions(t *testing.T) {
	server, _ := setupTestServer(t)

	body := `{"name":"Full Check","url":"https://example.com","interval_seconds":30,"timeout_seconds":5,"expected_status":201,"tags":["api","v2"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/checks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAPICreateCheckValidation(t *testing.T) {
	server, _ := setupTestServer(t)

	// Missing name
	body := `{"url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/checks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing name, got %d", rec.Code)
	}

	// Missing URL
	body = `{"name":"Test"}`
	req = httptest.NewRequest(http.MethodPost, "/api/checks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing URL, got %d", rec.Code)
	}

	// Invalid JSON
	body = `{invalid}`
	req = httptest.NewRequest(http.MethodPost, "/api/checks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestAPIGetCheck(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{
		Name:           "Get Test",
		URL:            "https://gettest.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	req := httptest.NewRequest(http.MethodGet, "/api/checks/1", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestAPIGetCheckNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/checks/999", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestAPIGetCheckInvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/checks/invalid", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPIUpdateCheck(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{
		Name:           "Update Test",
		URL:            "https://updatetest.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	body := `{"name":"Updated Name","interval_seconds":30}`
	req := httptest.NewRequest(http.MethodPut, "/api/checks/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify update
	updated, _ := store.GetCheck(check.ID)
	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %s", updated.Name)
	}
	if updated.IntervalSecs != 30 {
		t.Errorf("expected interval 30, got %d", updated.IntervalSecs)
	}
}

func TestAPIUpdateCheckNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	body := `{"name":"Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/api/checks/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestAPIUpdateCheckInvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	body := `{"name":"Updated"}`
	req := httptest.NewRequest(http.MethodPut, "/api/checks/invalid", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPIUpdateCheckInvalidJSON(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{Name: "Test", URL: "https://test.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	body := `{invalid}`
	req := httptest.NewRequest(http.MethodPut, "/api/checks/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPIDeleteCheck(t *testing.T) {
	server, store := setupTestServer(t)

	// Create a check first
	check := &storage.Check{
		Name:           "To Delete",
		URL:            "https://delete.me",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/checks/1", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify deleted
	deleted, _ := store.GetCheck(check.ID)
	if deleted != nil {
		t.Error("expected check to be deleted")
	}
}

func TestAPIDeleteCheckInvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/checks/invalid", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPIGetCheckResults(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{Name: "Results Test", URL: "https://results.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	// Add some results
	for i := 0; i < 5; i++ {
		store.SaveResult(&storage.CheckResult{
			CheckID:        check.ID,
			Status:         "up",
			StatusCode:     200,
			ResponseTimeMs: 100,
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/checks/1/results", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be array")
	}
	if len(data) != 5 {
		t.Errorf("expected 5 results, got %d", len(data))
	}
}

func TestAPIGetCheckResultsWithPagination(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{Name: "Pagination Test", URL: "https://pagination.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	// Add 10 results
	for i := 0; i < 10; i++ {
		store.SaveResult(&storage.CheckResult{
			CheckID:        check.ID,
			Status:         "up",
			StatusCode:     200,
			ResponseTimeMs: 100,
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/checks/1/results?limit=3&offset=2", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be array")
	}
	if len(data) != 3 {
		t.Errorf("expected 3 results with limit=3, got %d", len(data))
	}
}

func TestAPIGetCheckResultsInvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/checks/invalid/results", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPIGetCheckStats(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{Name: "Stats Test", URL: "https://stats.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	// Add some results
	for i := 0; i < 10; i++ {
		store.SaveResult(&storage.CheckResult{
			CheckID:        check.ID,
			Status:         "up",
			StatusCode:     200,
			ResponseTimeMs: 100 + i*10,
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/checks/1/stats", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.Error != "" {
		t.Errorf("unexpected error: %s", resp.Error)
	}
}

func TestAPIGetCheckStatsInvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/checks/invalid/stats", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPITriggerCheckNoScheduler(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{Name: "Trigger Test", URL: "https://trigger.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	req := httptest.NewRequest(http.MethodPost, "/api/checks/1/trigger", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Without scheduler, should return error
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 without scheduler, got %d", rec.Code)
	}
}

func TestAPITriggerCheckInvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/checks/invalid/trigger", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPIListIncidents(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/incidents", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAPIListIncidentsWithData(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{Name: "Incident Test", URL: "https://incident.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	// Create incidents
	for i := 0; i < 3; i++ {
		store.CreateIncident(&storage.Incident{
			CheckID:   check.ID,
			StartedAt: time.Now(),
			Cause:     "Test",
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/api/incidents?limit=2", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp APIResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	data, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatal("expected data to be array")
	}
	if len(data) != 2 {
		t.Errorf("expected 2 incidents with limit=2, got %d", len(data))
	}
}

func TestAPIGetIncident(t *testing.T) {
	server, store := setupTestServer(t)

	check := &storage.Check{Name: "Get Incident", URL: "https://getincident.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	incident := &storage.Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
		Cause:     "Test cause",
	}
	store.CreateIncident(incident)

	req := httptest.NewRequest(http.MethodGet, "/api/incidents/1", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAPIGetIncidentNotFound(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/incidents/999", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestAPIGetIncidentInvalidID(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/incidents/invalid", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestAPIResponseStructure(t *testing.T) {
	// Test that APIResponse JSON marshals correctly
	resp := APIResponse{
		Data:  map[string]string{"key": "value"},
		Error: "",
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled map[string]interface{}
	json.Unmarshal(bytes, &unmarshaled)

	if _, ok := unmarshaled["data"]; !ok {
		t.Error("expected data field in response")
	}

	// Error should be omitted when empty
	if _, ok := unmarshaled["error"]; ok {
		t.Error("expected error field to be omitted when empty")
	}

	// Test with error
	respWithError := APIResponse{
		Error: "something went wrong",
	}

	bytes, _ = json.Marshal(respWithError)
	json.Unmarshal(bytes, &unmarshaled)

	if unmarshaled["error"] != "something went wrong" {
		t.Error("expected error field to be present")
	}
}
