package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/katieblackabee/sentinel/internal/alerter"
	"github.com/katieblackabee/sentinel/internal/checker"
	"github.com/katieblackabee/sentinel/internal/config"
	"github.com/katieblackabee/sentinel/internal/storage"
	"github.com/katieblackabee/sentinel/internal/web"
)

var Version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:   "sentinel",
		Short: "Self-hosted uptime monitoring",
		Long:  "Sentinel monitors your services and alerts you when they go down.",
		Run: func(cmd *cobra.Command, args []string) {
			serve()
		},
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the monitoring server",
		Run: func(cmd *cobra.Command, args []string) {
			serve()
		},
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("sentinel %s\n", Version)
		},
	}

	rootCmd.AddCommand(serveCmd, versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() {
	fmt.Printf("Sentinel %s starting...\n", Version)

	// Load configuration
	cfg, err := config.LoadWithEnv("sentinel.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize storage
	store, err := storage.NewSQLiteStorage(cfg.Database.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	// Create checks from config if any
	for _, checkCfg := range cfg.Checks {
		// Check if already exists
		existing, _ := store.GetCheckByURL(checkCfg.URL)
		if existing != nil {
			continue
		}

		check := &storage.Check{
			Name:           checkCfg.Name,
			URL:            checkCfg.URL,
			IntervalSecs:   int(checkCfg.GetInterval().Seconds()),
			TimeoutSecs:    int(checkCfg.GetTimeout().Seconds()),
			ExpectedStatus: checkCfg.GetExpectedStatus(),
			Enabled:        checkCfg.IsEnabled(),
			Tags:           checkCfg.Tags,
		}

		if err := store.CreateCheck(check); err != nil {
			fmt.Printf("Failed to create check %s: %v\n", checkCfg.Name, err)
		} else {
			fmt.Printf("Created check: %s\n", checkCfg.Name)
		}
	}

	// Initialize alerter
	alertMgr := alerter.NewManager(&cfg.Alerts, store)

	// Initialize scheduler
	sched := checker.NewScheduler(store, alertMgr, checker.SchedulerConfig{
		ConsecutiveFailures: cfg.Alerts.ConsecutiveFailures,
	})

	// Start scheduler
	if err := sched.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start scheduler: %v\n", err)
		os.Exit(1)
	}

	// Initialize web server
	server := web.NewServer(&cfg.Server, store, sched, cfg.Server.Users)

	// Handle shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil {
			fmt.Printf("Server stopped: %v\n", err)
		}
	}()

	<-quit
	fmt.Println("\nShutting down...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sched.Stop()

	if err := server.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Server shutdown error: %v\n", err)
	}

	fmt.Println("Goodbye!")
}
