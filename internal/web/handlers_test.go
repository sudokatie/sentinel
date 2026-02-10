package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
)

func setupTestServerWithTemplates(t *testing.T) (*Server, storage.Storage) {
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

	fullCfg := &config.Config{
		Server: *cfg,
		Alerts: config.AlertsConfig{
			ConsecutiveFailures:  2,
			RecoveryNotification: true,
			CooldownMinutes:      5,
			Email: config.EmailConfig{
				Enabled:     true,
				SMTPHost:    "smtp.test.com",
				SMTPPort:    587,
				FromAddress: "test@example.com",
				ToAddresses: []string{"alert@example.com"},
			},
		},
		Retention: config.RetentionConfig{
			ResultsDays:    7,
			AggregatesDays: 90,
		},
	}

	// Create server with full config for handler tests
	server := NewServer(cfg, fullCfg, store, nil, nil)

	t.Cleanup(func() {
		store.Close()
		os.Remove(dbPath)
	})

	return server, store
}

func TestHandleDashboardEmpty(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Sentinel") {
		t.Error("expected dashboard to contain Sentinel")
	}
}

func TestHandleDashboardWithChecks(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	// Create checks
	check := &storage.Check{
		Name:           "Test Check",
		URL:            "https://test.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Tags:           []string{"api"},
	}
	store.CreateCheck(check)

	// Add a result
	store.SaveResult(&storage.CheckResult{
		CheckID:        check.ID,
		Status:         "up",
		StatusCode:     200,
		ResponseTimeMs: 100,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Test Check") {
		t.Error("expected dashboard to contain check name")
	}
}

func TestHandleDashboardWithIncidents(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	check := &storage.Check{Name: "Incident Check", URL: "https://incident.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	// Add an incident
	store.CreateIncident(&storage.Incident{
		CheckID:   check.ID,
		StartedAt: time.Now(),
		Cause:     "Test incident",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleCheckDetail(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	check := &storage.Check{
		Name:           "Detail Check",
		URL:            "https://detail.com",
		IntervalSecs:   60,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
	}
	store.CreateCheck(check)

	// Add results
	for i := 0; i < 5; i++ {
		store.SaveResult(&storage.CheckResult{
			CheckID:        check.ID,
			Status:         "up",
			StatusCode:     200,
			ResponseTimeMs: 100 + i*10,
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/checks/1", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Detail Check") {
		t.Error("expected check detail to contain check name")
	}
}

func TestHandleCheckDetailWithPeriod(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	check := &storage.Check{Name: "Period Check", URL: "https://period.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	// Test different periods
	for _, period := range []string{"24h", "7d", "30d"} {
		req := httptest.NewRequest(http.MethodGet, "/checks/1?period="+period, nil)
		rec := httptest.NewRecorder()

		server.echo.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for period %s, got %d", period, rec.Code)
		}
	}
}

func TestHandleCheckDetailNotFound(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/checks/999", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}

func TestHandleCheckDetailInvalidID(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/checks/invalid", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleSettings(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	// Create some checks
	store.CreateCheck(&storage.Check{Name: "Check A", URL: "https://a.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true})
	store.CreateCheck(&storage.Check{Name: "Check B", URL: "https://b.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true})

	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("expected settings page")
	}
}

func TestHandleSettingsWithMessage(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/settings?message=Check+created", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleSettingsWithError(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/settings?error=Something+failed", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleCreateCheckForm(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	form := url.Values{}
	form.Add("name", "New Check")
	form.Add("url", "https://newcheck.com")
	form.Add("interval", "30")

	req := httptest.NewRequest(http.MethodPost, "/settings/checks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Should redirect after creation
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	// Verify check was created
	checks, _ := store.ListChecks()
	if len(checks) != 1 {
		t.Errorf("expected 1 check, got %d", len(checks))
	}
	if checks[0].Name != "New Check" {
		t.Errorf("expected name 'New Check', got %s", checks[0].Name)
	}
}

func TestHandleCreateCheckFormMissingName(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	form := url.Values{}
	form.Add("url", "https://test.com")

	req := httptest.NewRequest(http.MethodPost, "/settings/checks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Should redirect with error
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Error("expected redirect to contain error")
	}
}

func TestHandleCreateCheckFormMissingURL(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	form := url.Values{}
	form.Add("name", "Test")

	req := httptest.NewRequest(http.MethodPost, "/settings/checks", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Error("expected redirect to contain error")
	}
}

func TestHandleDeleteCheckForm(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	check := &storage.Check{Name: "To Delete", URL: "https://delete.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	req := httptest.NewRequest(http.MethodPost, "/settings/checks/1/delete", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	// Verify deleted
	deleted, _ := store.GetCheck(check.ID)
	if deleted != nil {
		t.Error("expected check to be deleted")
	}
}

func TestHandleDeleteCheckFormInvalidID(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodPost, "/settings/checks/invalid/delete", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Error("expected redirect to contain error")
	}
}

func TestHandleEditCheckFormGet(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	check := &storage.Check{Name: "Edit Me", URL: "https://edit.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	req := httptest.NewRequest(http.MethodGet, "/settings/checks/1/edit", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Edit Me") {
		t.Error("expected edit form to contain check name")
	}
}

func TestHandleEditCheckFormPost(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	check := &storage.Check{Name: "Original", URL: "https://original.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	form := url.Values{}
	form.Add("name", "Updated Name")
	form.Add("url", "https://updated.com")
	form.Add("interval", "30")
	form.Add("timeout", "5")
	form.Add("expected_status", "201")
	form.Add("enabled", "1")

	req := httptest.NewRequest(http.MethodPost, "/settings/checks/1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}

	// Verify updated
	updated, _ := store.GetCheck(check.ID)
	if updated.Name != "Updated Name" {
		t.Errorf("expected name 'Updated Name', got %s", updated.Name)
	}
}

func TestHandleEditCheckFormNotFound(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/settings/checks/999/edit", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}
}

func TestHandleEditCheckFormInvalidID(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/settings/checks/invalid/edit", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}
}

func TestHandleEditCheckFormMissingFields(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	check := &storage.Check{Name: "Test", URL: "https://test.com", IntervalSecs: 60, TimeoutSecs: 10, ExpectedStatus: 200, Enabled: true}
	store.CreateCheck(check)

	// Submit with empty name
	form := url.Values{}
	form.Add("name", "")
	form.Add("url", "https://test.com")

	req := httptest.NewRequest(http.MethodPost, "/settings/checks/1/edit", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Should re-render with error
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleLoginNoAuth(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Without auth configured, renders login page but auth is nil
	// The handler will render login.html since auth is nil
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected status 303, got %d", rec.Code)
	}
}

func TestBasePath(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	store, _ := storage.NewSQLiteStorage(dbPath)
	defer store.Close()

	cfg := &config.ServerConfig{
		Host:    "localhost",
		Port:    3000,
		BaseURL: "/sentinel",
	}

	server := NewServer(cfg, nil, store, nil, nil)

	if server.BasePath() != "/sentinel" {
		t.Errorf("expected base path /sentinel, got %s", server.BasePath())
	}
}

func TestDashboardDataStructure(t *testing.T) {
	data := DashboardData{
		Title:          "Test",
		BasePath:       "/app",
		AllOperational: true,
		OverallUptime:  99.9,
		LastUpdated:    time.Now(),
	}

	if data.Title != "Test" {
		t.Error("expected Title to be set")
	}
	if !data.AllOperational {
		t.Error("expected AllOperational to be true")
	}
}

func TestCheckWithStatusStructure(t *testing.T) {
	check := &storage.Check{Name: "Test"}
	cws := &CheckWithStatus{
		Check:         check,
		UptimePercent: 99.5,
		Sparkline:     []bool{true, true, false, true},
	}

	if cws.UptimePercent != 99.5 {
		t.Errorf("expected uptime 99.5, got %f", cws.UptimePercent)
	}
	if len(cws.Sparkline) != 4 {
		t.Errorf("expected 4 sparkline points, got %d", len(cws.Sparkline))
	}
}

func TestTemplateRender(t *testing.T) {
	tmpl := &Template{
		basePath: "/app",
	}

	// Just verify structure exists
	if tmpl.basePath != "/app" {
		t.Error("expected basePath to be set")
	}
}

func TestHandleStatusPage(t *testing.T) {
	server, store := setupTestServerWithTemplates(t)

	// Create a check with a tag
	check := &storage.Check{
		Name:           "API Server",
		URL:            "https://api.example.com",
		IntervalSecs:   30,
		TimeoutSecs:    10,
		ExpectedStatus: 200,
		Enabled:        true,
		Tags:           []string{"public", "api"},
	}
	if err := store.CreateCheck(check); err != nil {
		t.Fatalf("failed to create check: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/status/public", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "API Server") {
		t.Error("expected status page to contain check name")
	}
	if !strings.Contains(body, "public Status") {
		t.Error("expected status page to contain slug in title")
	}
}

func TestHandleStatusPageNotFound(t *testing.T) {
	server, _ := setupTestServerWithTemplates(t)

	req := httptest.NewRequest(http.MethodGet, "/status/nonexistent", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}
}
