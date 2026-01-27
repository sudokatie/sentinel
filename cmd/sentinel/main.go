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

	// Check commands
	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Manage checks",
	}

	checkAddCmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a new check",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkAdd(cmd, args[0])
		},
	}
	checkAddCmd.Flags().StringP("name", "n", "", "Check name (defaults to URL hostname)")
	checkAddCmd.Flags().IntP("interval", "i", 60, "Check interval in seconds")
	checkAddCmd.Flags().IntP("timeout", "t", 10, "Request timeout in seconds")
	checkAddCmd.Flags().IntP("status", "s", 200, "Expected HTTP status code")

	checkListCmd := &cobra.Command{
		Use:   "list",
		Short: "List all checks",
		Run: func(cmd *cobra.Command, args []string) {
			checkList()
		},
	}

	checkTestCmd := &cobra.Command{
		Use:   "test <url>",
		Short: "Test a URL without saving",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			checkTest(args[0])
		},
	}

	checkCmd.AddCommand(checkAddCmd, checkListCmd, checkTestCmd)
	rootCmd.AddCommand(serveCmd, versionCmd, checkCmd)

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
		RetentionDays:       cfg.Retention.ResultsDays,
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

func checkAdd(cmd *cobra.Command, url string) {
	cfg, err := config.LoadWithEnv("sentinel.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	store, err := storage.NewSQLiteStorage(cfg.Database.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		// Extract hostname from URL as default name
		name = url
		if len(url) > 50 {
			name = url[:50] + "..."
		}
	}

	interval, _ := cmd.Flags().GetInt("interval")
	timeout, _ := cmd.Flags().GetInt("timeout")
	status, _ := cmd.Flags().GetInt("status")

	check := &storage.Check{
		Name:           name,
		URL:            url,
		IntervalSecs:   interval,
		TimeoutSecs:    timeout,
		ExpectedStatus: status,
		Enabled:        true,
	}

	if err := store.CreateCheck(check); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create check: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created check: %s (ID: %d)\n", check.Name, check.ID)
}

func checkList() {
	cfg, err := config.LoadWithEnv("sentinel.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	store, err := storage.NewSQLiteStorage(cfg.Database.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	checks, err := store.ListChecks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list checks: %v\n", err)
		os.Exit(1)
	}

	if len(checks) == 0 {
		fmt.Println("No checks configured")
		return
	}

	fmt.Printf("%-4s %-30s %-40s %-10s %-8s\n", "ID", "NAME", "URL", "INTERVAL", "ENABLED")
	fmt.Println(string(make([]byte, 96)))
	for _, c := range checks {
		enabled := "yes"
		if !c.Enabled {
			enabled = "no"
		}
		name := c.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		url := c.URL
		if len(url) > 40 {
			url = url[:37] + "..."
		}
		fmt.Printf("%-4d %-30s %-40s %-10s %-8s\n", c.ID, name, url, fmt.Sprintf("%ds", c.IntervalSecs), enabled)
	}
}

func checkTest(url string) {
	fmt.Printf("Testing %s...\n", url)

	httpChecker := checker.NewHTTPChecker()
	resp := httpChecker.Execute(&checker.CheckRequest{
		URL:            url,
		Timeout:        10 * time.Second,
		ExpectedStatus: 200,
	})

	if resp.Error != nil {
		fmt.Printf("FAILED: %v\n", resp.Error)
		os.Exit(1)
	}

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response time: %dms\n", resp.ResponseTimeMs)

	if resp.StatusCode == 200 {
		fmt.Println("Result: OK")
	} else {
		fmt.Printf("Result: Unexpected status (expected 200)\n")
		os.Exit(1)
	}
}
