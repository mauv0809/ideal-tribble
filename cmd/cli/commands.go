package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"cloud.google.com/go/pubsub"
	"github.com/spf13/cobra"
)

var days int
var playerName, emoji string

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
	root.AddCommand(setupPubsubCmd)

	// React command for testing
	reactCmd.Flags().StringVar(&playerName, "name", "", "Player name to react as (required)")
	reactCmd.Flags().StringVar(&emoji, "emoji", "", "Emoji reaction: one, two, three, four, five, six, seven (required)")
	reactCmd.MarkFlagRequired("name")
	reactCmd.MarkFlagRequired("emoji")
	root.AddCommand(reactCmd)

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

var setupPubsubCmd = &cobra.Command{
	Use:   "setup-pubsub",
	Short: "Set up Pub/Sub topics and subscriptions for local development",
	Long: `Creates all required Pub/Sub topics and subscriptions for local development.
This command connects to the Pub/Sub emulator at localhost:8085 and creates:
- Topics: assign_ball_boy, update_player_stats, update_weekly_stats, notify_booking, notify_result
- Push subscriptions for each topic pointing to localhost endpoints`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return setupPubSubTopicsAndSubscriptions()
	},
}

var reactCmd = &cobra.Command{
	Use:   "react",
	Short: "Simulate a player emoji reaction for testing match availability",
	Long: `Simulates a player adding an emoji reaction to a match request for testing purposes.
This bypasses Slack and directly adds availability to the active match request.

Emoji options:
  one   = Monday
  two   = Tuesday  
  three = Wednesday
  four  = Thursday
  five  = Friday
  six   = Saturday
  seven = Sunday

Example: ./tribble-cli react --name="Jacob Smith" --emoji="three"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate emoji
		validEmojis := map[string]bool{
			"one": true, "two": true, "three": true, "four": true,
			"five": true, "six": true, "seven": true,
		}
		if !validEmojis[emoji] {
			return fmt.Errorf("invalid emoji '%s'. Valid options: one, two, three, four, five, six, seven", emoji)
		}

		form := url.Values{}
		form.Add("player_name", playerName)
		form.Add("emoji", emoji)
		return performPostRequest("/test/react", strings.NewReader(form.Encode()))
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

func setupPubSubTopicsAndSubscriptions() error {
	ctx := context.Background()

	// Set the emulator host
	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:8085")

	// Topics and their corresponding endpoints (matching terraform/variables.tf)
	topics := map[string]string{
		"assign_ball_boy":     "/assign-ball-boy",
		"update_player_stats": "/update-player-stats",
		"update_weekly_stats": "/update-weekly-stats",
		"notify_booking":      "/notify-booking",
		"notify_result":       "/notify-result",
	}

	// Create client (project ID doesn't matter for emulator)
	client, err := pubsub.NewClient(ctx, "TEST")
	if err != nil {
		return fmt.Errorf("failed to create pubsub client: %w", err)
	}
	defer client.Close()

	fmt.Printf("Setting up Pub/Sub topics and subscriptions on emulator (localhost:8085)...\n")

	for topicName, endpoint := range topics {
		// Create topic
		topic := client.Topic(topicName)
		exists, err := topic.Exists(ctx)
		if err != nil {
			return fmt.Errorf("failed to check if topic %s exists: %w", topicName, err)
		}

		if !exists {
			_, err = client.CreateTopic(ctx, topicName)
			if err != nil {
				return fmt.Errorf("failed to create topic %s: %w", topicName, err)
			}
			fmt.Printf("✓ Created topic: %s\n", topicName)
		} else {
			fmt.Printf("- Topic already exists: %s\n", topicName)
		}

		// Create push subscription
		subName := topicName + "-sub"
		sub := client.Subscription(subName)
		exists, err = sub.Exists(ctx)
		if err != nil {
			return fmt.Errorf("failed to check if subscription %s exists: %w", subName, err)
		}

		if !exists {
			pushConfig := pubsub.PushConfig{
				Endpoint: "http://localhost:8080" + endpoint,
			}

			_, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
				Topic:      topic,
				PushConfig: pushConfig,
			})
			if err != nil {
				return fmt.Errorf("failed to create subscription %s: %w", subName, err)
			}
			fmt.Printf("✓ Created subscription: %s -> http://localhost:8080%s\n", subName, endpoint)
		} else {
			fmt.Printf("- Subscription already exists: %s\n", subName)
		}
	}

	fmt.Printf("\nPub/Sub setup complete! All topics and subscriptions are ready for local development.\n")
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
