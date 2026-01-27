package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	fmt.Println("Sentinel starting...")
	fmt.Printf("Version: %s\n", Version)
	// Will be implemented in later tasks
	select {} // Block forever for now
}
