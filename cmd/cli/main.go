package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	host    string
	dryRun  bool
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "tribble-cli",
	Short: "A CLI to interact with the ideal-tribble server",
	Long: `A command-line interface for making requests to the various endpoints
of the ideal-tribble application.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&host, "host", "http://localhost:8080", "The host address of the server")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Print the request without sending it")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Print the response body")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your command '%s'", err)
		os.Exit(1)
	}
}

func main() {
	addCommands(rootCmd)
	Execute()
}
