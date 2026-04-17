package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/katieblackabee/sentinel/internal/probe"
)

var Version = "dev"

func main() {
	serverURL := flag.String("server", "", "Sentinel server URL (required)")
	probeKey := flag.String("key", "", "Probe API key (required)")
	locationName := flag.String("name", "", "Probe location name (required)")
	region := flag.String("region", "", "Probe region (required)")
	city := flag.String("city", "", "Probe city")
	country := flag.String("country", "", "Probe country")
	lat := flag.Float64("lat", 0, "Latitude")
	lon := flag.Float64("lon", 0, "Longitude")
	version := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *version {
		fmt.Printf("sentinel-probe %s\n", Version)
		os.Exit(0)
	}

	// Validate required flags
	if *serverURL == "" {
		fmt.Fprintln(os.Stderr, "Error: -server is required")
		flag.Usage()
		os.Exit(1)
	}
	if *probeKey == "" {
		fmt.Fprintln(os.Stderr, "Error: -key is required")
		flag.Usage()
		os.Exit(1)
	}
	if *locationName == "" {
		fmt.Fprintln(os.Stderr, "Error: -name is required")
		flag.Usage()
		os.Exit(1)
	}
	if *region == "" {
		fmt.Fprintln(os.Stderr, "Error: -region is required")
		flag.Usage()
		os.Exit(1)
	}

	// Create agent
	agent := probe.NewProbeAgent(
		*serverURL,
		*probeKey,
		*locationName,
		*region,
		*city,
		*country,
		*lat,
		*lon,
	)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down probe agent...")
		cancel()
		agent.Stop()
	}()

	log.Printf("Starting Sentinel probe agent %s", Version)
	log.Printf("Location: %s (%s, %s)", *locationName, *city, *region)
	log.Printf("Connecting to server: %s", *serverURL)

	// Start agent and block until context cancelled
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Agent failed: %v", err)
	}

	log.Println("Probe agent stopped")
}
