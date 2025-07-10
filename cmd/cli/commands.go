package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(membersCmd)
	rootCmd.AddCommand(populateCmd)
	rootCmd.AddCommand(metricsCmd)
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health of the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/health")
	},
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Trigger a check for new matches and notify",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/check")
	},
}

var membersCmd = &cobra.Command{
	Use:   "members",
	Short: "List the members in the club store",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/members")
	},
}

var populateCmd = &cobra.Command{
	Use:   "populate",
	Short: "Populate the club store from the Slack channel",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/populate-club")
	},
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Get application metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/metrics")
	},
}

func performGetRequest(endpoint string) error {
	url := host + endpoint
	fmt.Printf("Making request to %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Println("Response Body:")
	fmt.Println(string(body))

	return nil
}
