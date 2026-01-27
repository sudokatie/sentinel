package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/katieblackabee/sentinel/internal/checker"
	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Server struct {
	echo       *echo.Echo
	config     *config.ServerConfig
	fullConfig *config.Config
	storage    storage.Storage
	scheduler  *checker.Scheduler
	auth       *AuthManager
}

type Template struct {
	templates *template.Template
	basePath  string
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	// Inject base path into data if it's a map
	if m, ok := data.(map[string]interface{}); ok {
		m["BasePath"] = t.basePath
	}
	return t.templates.ExecuteTemplate(w, name, data)
}

func NewServer(cfg *config.ServerConfig, fullCfg *config.Config, store storage.Storage, sched *checker.Scheduler, users map[string]string) *Server {
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	// Parse templates
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	basePath := ""
	if cfg.BaseURL != "" {
		basePath = cfg.BaseURL
	}
	e.Renderer = &Template{templates: tmpl, basePath: basePath}

	// Static files
	staticContent, _ := fs.Sub(staticFS, "static")
	e.StaticFS("/static", staticContent)

	// Auth manager
	var auth *AuthManager
	if len(users) > 0 {
		auth = NewAuthManager(users, basePath)
	}

	server := &Server{
		echo:       e,
		config:     cfg,
		fullConfig: fullCfg,
		storage:    store,
		scheduler:  sched,
		auth:       auth,
	}

	// Register routes
	server.registerRoutes()

	return server
}

func (s *Server) registerRoutes() {
	// Auth routes (always public)
	s.echo.GET("/login", s.HandleLogin)
	s.echo.POST("/login", s.HandleLogin)
	s.echo.GET("/logout", s.HandleLogout)

	// Health check (public)
	s.echo.GET("/api/health", s.HandleHealth)

	// Protected routes
	if s.auth != nil {
		// Pages with auth
		s.echo.GET("/", s.HandleDashboard, s.auth.RequireAuth)
		s.echo.GET("/checks/:id", s.HandleCheckDetail, s.auth.RequireAuth)
		s.echo.GET("/settings", s.HandleSettings, s.auth.RequireAuth)
		s.echo.POST("/settings/checks", s.HandleCreateCheckForm, s.auth.RequireAuth)
		s.echo.GET("/settings/checks/:id/edit", s.HandleEditCheckForm, s.auth.RequireAuth)
		s.echo.POST("/settings/checks/:id/edit", s.HandleEditCheckForm, s.auth.RequireAuth)
		s.echo.POST("/settings/checks/:id/delete", s.HandleDeleteCheckForm, s.auth.RequireAuth)

		// API with auth
		api := s.echo.Group("/api", s.auth.RequireAuth)
		api.GET("/checks", s.HandleListChecks)
		api.POST("/checks", s.HandleCreateCheck)
		api.GET("/checks/:id", s.HandleGetCheck)
		api.PUT("/checks/:id", s.HandleUpdateCheck)
		api.DELETE("/checks/:id", s.HandleDeleteCheck)
		api.GET("/checks/:id/results", s.HandleGetCheckResults)
		api.GET("/checks/:id/stats", s.HandleGetCheckStats)
		api.POST("/checks/:id/trigger", s.HandleTriggerCheck)
		api.GET("/incidents", s.HandleListIncidents)
		api.GET("/incidents/:id", s.HandleGetIncident)
	} else {
		// No auth - public access
		s.echo.GET("/", s.HandleDashboard)
		s.echo.GET("/checks/:id", s.HandleCheckDetail)
		s.echo.GET("/settings", s.HandleSettings)
		s.echo.POST("/settings/checks", s.HandleCreateCheckForm)
		s.echo.GET("/settings/checks/:id/edit", s.HandleEditCheckForm)
		s.echo.POST("/settings/checks/:id/edit", s.HandleEditCheckForm)
		s.echo.POST("/settings/checks/:id/delete", s.HandleDeleteCheckForm)

		api := s.echo.Group("/api")
		api.GET("/checks", s.HandleListChecks)
		api.POST("/checks", s.HandleCreateCheck)
		api.GET("/checks/:id", s.HandleGetCheck)
		api.PUT("/checks/:id", s.HandleUpdateCheck)
		api.DELETE("/checks/:id", s.HandleDeleteCheck)
		api.GET("/checks/:id/results", s.HandleGetCheckResults)
		api.GET("/checks/:id/stats", s.HandleGetCheckStats)
		api.POST("/checks/:id/trigger", s.HandleTriggerCheck)
		api.GET("/incidents", s.HandleListIncidents)
		api.GET("/incidents/:id", s.HandleGetIncident)
	}
}

func (s *Server) BasePath() string {
	return s.config.BaseURL
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	fmt.Printf("Starting server on %s\n", addr)

	server := &http.Server{
		Addr:         addr,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return s.echo.StartServer(server)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}
