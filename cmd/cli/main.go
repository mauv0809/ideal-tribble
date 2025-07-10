package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	host string
)

var rootCmd = &cobra.Command{
	Use:   "tribble-cli",
	Short: "A CLI to interact with the ideal-tribble server",
	Long: `A command-line interface for making requests to the various endpoints
of the ideal-tribble application.`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&host, "host", "http://localhost:8080", "The host address of the server")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your command '%s'", err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}
