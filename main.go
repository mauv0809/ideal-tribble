package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/rafa-garcia/go-playtomic-api/client"
	"github.com/rafa-garcia/go-playtomic-api/models"
)

// Config stores the application configuration.
// It's populated from environment variables.
type Config struct {
	PlaytomicUser  string
	PlaytomicPass  string
	SlackBotToken  string
	SlackChannelID string
	BookingFilter  string // A string to identify the specific booking
	Port           string
}

func main() {
	// Load configuration from environment variables
	cfg := loadConfig()

	// Set up the HTTP handler
	http.HandleFunc("/check", checkAndNotifyHandler(cfg))

	// Start the server
	log.Printf("Starting server on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, nil); err != nil {
		log.Fatalf("failed to start server: %s\n", err)
	}
}

// loadConfig loads configuration from environment variables.
func loadConfig() Config {
	// A helper function to get an env var or return a default
	getEnv := func(key, fallback string) string {
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		if fallback == "" {
			log.Fatalf("Error: Environment variable %s is not set.", key)
		}
		return fallback
	}

	return Config{
		//SlackBotToken:  getEnv("SLACK_BOT_TOKEN", ""),
		//SlackChannelID: getEnv("SLACK_CHANNEL_ID", ""),
		//BookingFilter:  getEnv("BOOKING_FILTER", "Padel"), // Default to "Padel"
		Port: getEnv("PORT", "8080"),
	}
}

type Player struct {
	name  string
	level float64
}
type Team struct {
	id      string
	players []Player
}
type PadelMatch struct {
	start int64
	end   int64
	teams []Team
}

// checkAndNotifyHandler is the main HTTP handler for our logic.
func checkAndNotifyHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received request to /check. Fetching bookings...")
		ctx := context.Background()
		// 1. Initialize Playtomic Client
		playtomicClient := client.NewClient(
			client.WithTimeout(10*time.Second),
			client.WithRetries(3),
		)
		//2. setup search params
		params := &models.SearchMatchesParams{
			SportID:       "PADEL",
			HasPlayers:    true,
			Sort:          "start_date,ASC",
			TenantIDs:     []string{"b8fe7430-f819-4413-b402-a008f94fc2b5"},
			FromStartDate: time.Now().Format("2006-01-02") + "T00:00:00",
		}
		log.Infof("Seaching from: %s", params.FromStartDate)
		// 2. Fetch Upcoming Bookings
		matches, err := playtomicClient.GetMatches(ctx, params)
		if err != nil {
			log.Error("Error fetching Playtomic bookings: %v", err)
			http.Error(w, "Error fetching Playtomic bookings", http.StatusInternalServerError)
			return
		}

		notifiedCount := 0

		// 3. Filter for specific matches and send Slack notifications
		for _, match := range matches {
			// We only want to notify for matches in the future.
			ownerID := match.OwnerID
			if ownerID != nil && *ownerID == "9759891" {
				const layout = "2006-01-02T15:04:05"

				startTime, err := time.Parse(layout, match.StartDate)
				if err != nil {
					log.Errorf("Failed to parse start time: %s", err.Error())
					continue
				}
				endTime, err := time.Parse(layout, match.EndDate)
				if err != nil {
					log.Errorf("Failed to parse end time: %s", err.Error())
					continue
				}
				teams := []Team{}
				for _, team := range match.Teams {
					t := Team{
						id: team.TeamID,
					}
					for _, player := range team.Players {
						t.players = append(t.players, Player{
							name:  player.Name,
							level: player.LevelValue,
						})

					}
					teams = append(teams, t)
				}

				padelMatch := PadelMatch{
					start: startTime.Local().Unix(),
					end:   endTime.Local().Unix(),
					teams: teams,
				}
				log.Info("Match found:", "match", padelMatch)
				notifiedCount++
			}
		}
		log.Infof("Check complete. Notified for %d matches", notifiedCount)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Check complete. Notified for %d matches", notifiedCount)
	}
}

// isTargetBooking checks if a booking matches our filter criteria.
// This function is an example. You should customize it based on the
// actual data in the Playtomic booking object and your filtering needs.
/*func isTargetBooking(match models.Match, filter string) bool {
	// Example: Check if the resource type or court name contains the filter string.
	// This is a guess; you'll need to see what fields the `model.match` struct has.
	if strings.Contains(strings.ToLower(match), strings.ToLower(filter)) {
		return true
	}
	if strings.Contains(strings.ToLower(match.ResourceType), strings.ToLower(filter)) {
		return true
	}
	// If no filter is matched, you might want a default behavior.
	// Here we return false if no match is found.
	return false
}

// sendSlackNotification formats and sends a message to Slack.
func sendSlackNotification(cfg Config, match models.Match) error {
	// Create a new Slack client
	slackClient := slack.New(cfg.SlackBotToken)

	// Format the message using Slack's Block Kit for better formatting
	headerText := slack.NewTextBlockObject("mrkdwn", "🎾 *Upcoming Playtomic match!* 🎾", false, false)
	headerSection := slack.NewHeaderBlock(headerText)

	fields := []*slack.TextBlockObject{
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Date:*\n%s", match.StartDate.Format("Monday, 02 Jan 2006")), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Time:*\n%s", match.StartDate.Format("15:04")), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Court:*\n%s", match.Court.Name), false, false),
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*match ID:*\n%s", match.ID), false, false),
	}
	fieldsSection := slack.NewSectionBlock(nil, fields, nil)

	// Post the message to the channel
	channelID, timestamp, err := slackClient.PostMessage(
		cfg.SlackChannelID,
		slack.MsgOptionBlocks(headerSection, fieldsSection),
		slack.MsgOptionAsUser(true), // Post as the bot user
	)

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	log.Printf("Message successfully sent to channel %s at %s", channelID, timestamp)
	return nil
}*/
