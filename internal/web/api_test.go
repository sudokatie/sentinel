package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestAPIListIncidents(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/incidents", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}
