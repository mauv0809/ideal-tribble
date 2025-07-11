package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

var days int

func addCommands(root *cobra.Command) {
	root.AddCommand(healthCmd)

	fetchCmd.Flags().IntVar(&days, "days", 0, "Number of past days to fetch matches from")
	root.AddCommand(fetchCmd)

	root.AddCommand(processCmd)
	root.AddCommand(membersCmd)
	root.AddCommand(matchesCmd)
	root.AddCommand(leaderboardCmd)
	root.AddCommand(metricsCmd)
	root.AddCommand(clearCmd)

	// Slack commands
	commandCmd.AddCommand(commandLeaderboardCmd)
	commandCmd.AddCommand(commandLevelLeaderboardCmd)
	commandCmd.AddCommand(commandPlayerStatsCmd)
	root.AddCommand(commandCmd)
}

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health of the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/health")
	},
}

var fetchCmd = &cobra.Command{
	Use:   "fetch",
	Short: "Trigger a fetch for new matches from Playtomic",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/fetch"
		if days > 0 {
			path = fmt.Sprintf("/fetch?days=%d", days)
		}
		return performPostRequest(path, nil)
	},
}

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Trigger the processing of fetched matches",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performPostRequest("/process", nil)
	},
}

var membersCmd = &cobra.Command{
	Use:   "members",
	Short: "List the members in the club store",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/members")
	},
}

var matchesCmd = &cobra.Command{
	Use:   "matches",
	Short: "List all processed matches",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/matches")
	},
}

var leaderboardCmd = &cobra.Command{
	Use:   "leaderboard",
	Short: "Get the player statistics leaderboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/leaderboard")
	},
}

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Get application metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performGetRequest("/metrics")
	},
}

var clearCmd = &cobra.Command{
	Use:   "clear [matchID]",
	Short: "Clear the internal store, or a specific match if matchID is provided",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/clear"
		if len(args) > 0 {
			path = fmt.Sprintf("/clear?matchID=%s", args[0])
		}
		return performPostRequest(path, nil)
	},
}

var commandCmd = &cobra.Command{
	Use:   "command",
	Short: "Execute Slack commands",
}

var commandLeaderboardCmd = &cobra.Command{
	Use:   "leaderboard",
	Short: "Get the leaderboard formatted for Slack",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performPostRequest("/slack/command/leaderboard", nil)
	},
}

var commandLevelLeaderboardCmd = &cobra.Command{
	Use:   "level-leaderboard",
	Short: "Get the level leaderboard formatted for Slack",
	RunE: func(cmd *cobra.Command, args []string) error {
		return performPostRequest("/slack/command/level-leaderboard", nil)
	},
}

var commandPlayerStatsCmd = &cobra.Command{
	Use:   "player-stats [name]",
	Short: "Get stats for a specific player formatted for Slack",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		form := url.Values{}
		form.Add("text", args[0])
		return performPostRequest("/slack/command/player-stats", strings.NewReader(form.Encode()))
	},
}

func performGetRequest(endpoint string) error {
	fullURL := host + endpoint
	if dryRun {
		fmt.Printf("Dry run: Would make GET request to %s\n", fullURL)
		return nil
	}

	if verbose {
		fmt.Printf("Making GET request to %s\n", fullURL)
	}

	resp, err := http.Get(fullURL)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if verbose {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
		fmt.Println("Response Body:")
		fmt.Println(string(body))
	} else {
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
	}

	return nil
}

func performPostRequest(endpoint string, reqBody io.Reader) error {
	fullURL := host + endpoint
	if dryRun {
		fmt.Printf("Dry run: Would make POST request to %s\n", fullURL)
		return nil
	}

	if verbose {
		fmt.Printf("Making POST request to %s\n", fullURL)
	}

	var req *http.Request
	var err error

	if reqBody != nil {
		// We need to buffer the body if we want to log it and send it.
		buf, err := io.ReadAll(reqBody)
		if err != nil {
			return fmt.Errorf("failed to read request body: %w", err)
		}
		if verbose {
			fmt.Printf("Request Body: %s\n", string(buf))
		}
		req, err = http.NewRequest("POST", fullURL, bytes.NewBuffer(buf))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest("POST", fullURL, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if verbose {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
		fmt.Println("Response Body:")
		fmt.Println(string(body))
	} else {
		fmt.Printf("Status Code: %d\n", resp.StatusCode)
	}

	return nil
}
