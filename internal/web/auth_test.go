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

	"github.com/labstack/echo/v4"

	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
)

func TestNewAuthManager(t *testing.T) {
	users := map[string]string{
		"admin": "password123",
	}

	auth := NewAuthManager(users, "/app")

	if auth == nil {
		t.Fatal("expected auth manager to be created")
	}
	if len(auth.users) != 1 {
		t.Errorf("expected 1 user, got %d", len(auth.users))
	}
	if auth.basePath != "/app" {
		t.Errorf("expected basePath /app, got %s", auth.basePath)
	}
}

func TestValidateUser(t *testing.T) {
	users := map[string]string{
		"admin": "password123",
		"user":  "userpass",
	}

	auth := NewAuthManager(users, "")

	// Valid credentials
	if !auth.ValidateUser("admin", "password123") {
		t.Error("expected valid credentials to pass")
	}

	// Invalid password
	if auth.ValidateUser("admin", "wrongpassword") {
		t.Error("expected invalid password to fail")
	}

	// Invalid username
	if auth.ValidateUser("nonexistent", "password") {
		t.Error("expected invalid username to fail")
	}

	// Empty credentials
	if auth.ValidateUser("", "") {
		t.Error("expected empty credentials to fail")
	}
}

func TestCreateSession(t *testing.T) {
	auth := NewAuthManager(map[string]string{"admin": "pass"}, "")

	token := auth.CreateSession("admin")

	if token == "" {
		t.Error("expected non-empty token")
	}
	if len(token) != 64 { // 32 bytes hex encoded = 64 chars
		t.Errorf("expected token length 64, got %d", len(token))
	}
}

func TestValidateSession(t *testing.T) {
	auth := NewAuthManager(map[string]string{"admin": "pass"}, "")

	// Create session
	token := auth.CreateSession("admin")

	// Valid session
	session := auth.ValidateSession(token)
	if session == nil {
		t.Fatal("expected valid session")
	}
	if session.Username != "admin" {
		t.Errorf("expected username admin, got %s", session.Username)
	}

	// Invalid token
	session = auth.ValidateSession("invalid-token")
	if session != nil {
		t.Error("expected nil for invalid token")
	}

	// Empty token
	session = auth.ValidateSession("")
	if session != nil {
		t.Error("expected nil for empty token")
	}
}

func TestValidateSessionExpired(t *testing.T) {
	auth := NewAuthManager(map[string]string{"admin": "pass"}, "")

	// Create session and manually expire it
	token := auth.CreateSession("admin")
	auth.sessions[token].ExpiresAt = time.Now().Add(-1 * time.Hour)

	// Should fail due to expiration
	session := auth.ValidateSession(token)
	if session != nil {
		t.Error("expected nil for expired session")
	}

	// Session should be deleted
	auth.mu.RLock()
	_, exists := auth.sessions[token]
	auth.mu.RUnlock()
	if exists {
		t.Error("expected expired session to be deleted")
	}
}

func TestDeleteSession(t *testing.T) {
	auth := NewAuthManager(map[string]string{"admin": "pass"}, "")

	token := auth.CreateSession("admin")

	// Verify session exists
	if auth.ValidateSession(token) == nil {
		t.Fatal("expected session to exist")
	}

	// Delete session
	auth.DeleteSession(token)

	// Verify session is gone
	if auth.ValidateSession(token) != nil {
		t.Error("expected session to be deleted")
	}
}

func TestDeleteSessionNonExistent(t *testing.T) {
	auth := NewAuthManager(map[string]string{"admin": "pass"}, "")

	// Should not panic
	auth.DeleteSession("non-existent-token")
}

func TestGenerateToken(t *testing.T) {
	token1 := generateToken()
	token2 := generateToken()

	if token1 == token2 {
		t.Error("expected different tokens")
	}

	if len(token1) != 64 {
		t.Errorf("expected token length 64, got %d", len(token1))
	}
}

func TestRequireAuthMiddleware(t *testing.T) {
	auth := NewAuthManager(map[string]string{"admin": "pass"}, "")

	e := echo.New()

	// Create handler that uses middleware
	handler := auth.RequireAuth(func(c echo.Context) error {
		return c.String(http.StatusOK, "protected")
	})

	// Request without cookie
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rec.Code)
	}

	// Request with valid session
	token := auth.CreateSession("admin")
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sentinel_session", Value: token})
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	err = handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Request with invalid session
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sentinel_session", Value: "invalid-token"})
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)

	err = handler(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rec.Code)
	}
}

func setupTestServerWithAuth(t *testing.T) (*Server, storage.Storage) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	cfg := &config.ServerConfig{
		Host: "localhost",
		Port: 3000,
		Users: map[string]string{
			"admin": "admin123",
		},
	}

	server := NewServer(cfg, nil, store, nil, cfg.Users)

	t.Cleanup(func() {
		store.Close()
		os.Remove(dbPath)
	})

	return server, store
}

func TestHandleLoginGet(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "/login") {
		t.Error("expected login page")
	}
}

func TestHandleLoginGetWithError(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	req := httptest.NewRequest(http.MethodGet, "/login?error=Invalid+credentials", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestHandleLoginPostSuccess(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "admin123")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rec.Code)
	}

	// Check for session cookie
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "sentinel_session" && c.Value != "" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}

func TestHandleLoginPostInvalid(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	form := url.Values{}
	form.Add("username", "admin")
	form.Add("password", "wrongpassword")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "error") {
		t.Error("expected redirect to login with error")
	}
}

func TestHandleLogoutWithSession(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	// Create a session first
	token := server.auth.CreateSession("admin")

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "sentinel_session", Value: token})
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rec.Code)
	}

	// Session should be deleted
	if server.auth.ValidateSession(token) != nil {
		t.Error("expected session to be deleted")
	}

	// Cookie should be cleared
	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "sentinel_session" {
			if c.MaxAge > 0 {
				t.Error("expected cookie to be cleared")
			}
		}
	}
}

func TestHandleLogoutWithoutSession(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Should not panic, just redirect
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rec.Code)
	}
}

func TestProtectedRouteRequiresAuth(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	// Access dashboard without auth
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Should redirect to login
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if !strings.Contains(location, "login") {
		t.Error("expected redirect to login")
	}
}

func TestProtectedRouteWithValidAuth(t *testing.T) {
	server, _ := setupTestServerWithAuth(t)

	// Create session
	token := server.auth.CreateSession("admin")

	// Access dashboard with auth
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "sentinel_session", Value: token})
	rec := httptest.NewRecorder()

	server.echo.ServeHTTP(rec, req)

	// Should succeed
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestSessionStructure(t *testing.T) {
	session := &Session{
		Username:  "testuser",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if session.Username != "testuser" {
		t.Error("expected username to be set")
	}
	if session.ExpiresAt.Before(time.Now()) {
		t.Error("expected expiration to be in the future")
	}
}
