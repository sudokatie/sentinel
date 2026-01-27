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
	echo      *echo.Echo
	config    *config.ServerConfig
	storage   storage.Storage
	scheduler *checker.Scheduler
}

type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}

func NewServer(cfg *config.ServerConfig, store storage.Storage, sched *checker.Scheduler) *Server {
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	// Parse templates
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	e.Renderer = &Template{templates: tmpl}

	// Static files
	staticContent, _ := fs.Sub(staticFS, "static")
	e.StaticFS("/static", staticContent)

	server := &Server{
		echo:      e,
		config:    cfg,
		storage:   store,
		scheduler: sched,
	}

	// Register routes
	server.registerRoutes()

	return server
}

func (s *Server) registerRoutes() {
	// Pages
	s.echo.GET("/", s.HandleDashboard)
	s.echo.GET("/checks/:id", s.HandleCheckDetail)
	s.echo.GET("/settings", s.HandleSettings)
	s.echo.POST("/settings/checks", s.HandleCreateCheckForm)
	s.echo.POST("/settings/checks/:id/delete", s.HandleDeleteCheckForm)

	// API
	api := s.echo.Group("/api")
	api.GET("/health", s.HandleHealth)
	api.GET("/checks", s.HandleListChecks)
	api.POST("/checks", s.HandleCreateCheck)
	api.GET("/checks/:id", s.HandleGetCheck)
	api.PUT("/checks/:id", s.HandleUpdateCheck)
	api.DELETE("/checks/:id", s.HandleDeleteCheck)
	api.GET("/checks/:id/results", s.HandleGetCheckResults)
	api.GET("/checks/:id/stats", s.HandleGetCheckStats)
	api.POST("/checks/:id/trigger", s.HandleTriggerCheck)
	api.GET("/incidents", s.HandleListIncidents)
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
