package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type AuthConfig struct {
	Users map[string]string // username -> password hash
}

type Session struct {
	Username  string
	ExpiresAt time.Time
}

type AuthManager struct {
	users    map[string]string
	sessions map[string]*Session
	basePath string
	mu       sync.RWMutex
}

func NewAuthManager(users map[string]string, basePath string) *AuthManager {
	return &AuthManager{
		users:    users,
		sessions: make(map[string]*Session),
		basePath: basePath,
	}
}

func (a *AuthManager) ValidateUser(username, password string) bool {
	storedPass, exists := a.users[username]
	if !exists {
		return false
	}
	// Constant-time comparison
	return subtle.ConstantTimeCompare([]byte(password), []byte(storedPass)) == 1
}

func (a *AuthManager) CreateSession(username string) string {
	token := generateToken()
	a.mu.Lock()
	a.sessions[token] = &Session{
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	a.mu.Unlock()
	return token
}

func (a *AuthManager) ValidateSession(token string) *Session {
	a.mu.RLock()
	session, exists := a.sessions[token]
	a.mu.RUnlock()

	if !exists {
		return nil
	}
	if time.Now().After(session.ExpiresAt) {
		a.mu.Lock()
		delete(a.sessions, token)
		a.mu.Unlock()
		return nil
	}
	return session
}

func (a *AuthManager) DeleteSession(token string) {
	a.mu.Lock()
	delete(a.sessions, token)
	a.mu.Unlock()
}

func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Middleware
func (a *AuthManager) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Check for session cookie
		cookie, err := c.Cookie("sentinel_session")
		if err != nil || cookie.Value == "" {
			return c.Redirect(http.StatusSeeOther, a.basePath+"/login")
		}

		session := a.ValidateSession(cookie.Value)
		if session == nil {
			return c.Redirect(http.StatusSeeOther, a.basePath+"/login")
		}

		// Store username in context
		c.Set("username", session.Username)
		return next(c)
	}
}

// Handlers
func (s *Server) HandleLogin(c echo.Context) error {
	if c.Request().Method == http.MethodGet {
		error := c.QueryParam("error")
		return c.Render(http.StatusOK, "login.html", map[string]interface{}{
			"Title": "Login",
			"Error": error,
		})
	}

	// POST - process login
	username := c.FormValue("username")
	password := c.FormValue("password")

	if !s.auth.ValidateUser(username, password) {
		return c.Redirect(http.StatusSeeOther, s.BasePath()+"/login?error=Invalid+credentials")
	}

	// Create session
	token := s.auth.CreateSession(username)

	cookie := &http.Cookie{
		Name:     "sentinel_session",
		Value:    token,
		Path:     s.BasePath() + "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400, // 24 hours
	}
	c.SetCookie(cookie)

	return c.Redirect(http.StatusSeeOther, s.BasePath()+"/")
}

func (s *Server) HandleLogout(c echo.Context) error {
	cookie, err := c.Cookie("sentinel_session")
	if err == nil && cookie.Value != "" {
		s.auth.DeleteSession(cookie.Value)
	}

	// Clear cookie
	c.SetCookie(&http.Cookie{
		Name:     "sentinel_session",
		Value:    "",
		Path:     s.BasePath() + "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	return c.Redirect(http.StatusSeeOther, s.BasePath()+"/login")
}
