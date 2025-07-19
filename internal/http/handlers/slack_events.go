package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
)

func SlackEventsHandler(store club.ClubStore, notifier notifier.Notifier, matchmakingService matchmaking.MatchmakingService, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Error("Failed to parse form", "error", err)
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		// Parse the event payload
		var eventPayload struct {
			Type      string `json:"type"`
			Challenge string `json:"challenge,omitempty"`
			Event     struct {
				Type     string `json:"type"`
				Channel  string `json:"channel,omitempty"`
				User     string `json:"user,omitempty"`
				// For reaction events
				Reaction string `json:"reaction,omitempty"`
				ItemUser string `json:"item_user,omitempty"`
				EventTs  string `json:"event_ts,omitempty"`
				Item     struct {
					Type      string `json:"type"`
					Channel   string `json:"channel"`
					Timestamp string `json:"ts"`
				} `json:"item,omitempty"`
			} `json:"event,omitempty"`
		}

		if err := json.Unmarshal(bodyBytes, &eventPayload); err != nil {
			log.Error("Failed to unmarshal event payload", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Handle challenge verification (for initial webhook setup)
		if eventPayload.Type == "url_verification" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(eventPayload.Challenge))
			return
		}
		log.Info("event type", "type", eventPayload.Type)
		// Handle actual events
		if eventPayload.Type == "event_callback" {
			log.Info("Received event", "type", eventPayload.Event.Type)

			// Filter for our specific channel only
			// For reaction events, channel is in Item.Channel, for other events it's in Event.Channel
			eventChannel := eventPayload.Event.Channel
			if eventChannel == "" && eventPayload.Event.Item.Channel != "" {
				eventChannel = eventPayload.Event.Item.Channel
			}
			
			if eventChannel != cfg.Slack.ChannelID {
				log.Info("Ignoring event from different channel", "channel", eventChannel)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
				return
			}

			// Handle member_joined_channel events
			if eventPayload.Event.Type == "member_joined_channel" {
				log.Info("New member joined channel", "user", eventPayload.Event.User, "channel", eventPayload.Event.Channel)

				if err := handleNewMember(eventPayload.Event.User, notifier); err != nil {
					log.Error("Failed to handle new member", "error", err, "user", eventPayload.Event.User)
					// Don't return error to Slack to avoid retries
				}
			}

			// Handle reaction_added events for availability collection
			if eventPayload.Event.Type == "reaction_added" {
				log.Info("Reaction added", "user", eventPayload.Event.User, "reaction", eventPayload.Event.Reaction, "timestamp", eventPayload.Event.Item.Timestamp)

				if err := handleReactionAdded(eventPayload.Event.User, eventPayload.Event.Reaction, eventPayload.Event.Item.Timestamp, store, matchmakingService); err != nil {
					log.Error("Failed to handle reaction added", "error", err, "user", eventPayload.Event.User)
					// Don't return error to Slack to avoid retries
				}
			}

			// Handle reaction_removed events for availability collection
			if eventPayload.Event.Type == "reaction_removed" {
				log.Info("Reaction removed", "user", eventPayload.Event.User, "reaction", eventPayload.Event.Reaction, "timestamp", eventPayload.Event.Item.Timestamp)

				if err := handleReactionRemoved(eventPayload.Event.User, eventPayload.Event.Reaction, eventPayload.Event.Item.Timestamp, store, matchmakingService); err != nil {
					log.Error("Failed to handle reaction removed", "error", err, "user", eventPayload.Event.User)
					// Don't return error to Slack to avoid retries
				}
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

// handleNewMember sends welcome message to new member asking for Playtomic profile
func handleNewMember(slackUserID string, notifier notifier.Notifier) error {
	log.Info("Sending welcome message to new member", "user", slackUserID)

	// Create welcome message asking for Playtomic profile URL
	welcomeText := "Welcome to the Padel club! 🎾\n\nTo get started with match notifications and our leaderboard, please share your Playtomic profile URL.\n\nYou can find it by:\n1. Opening the Playtomic app\n2. Going to your profile\n3. Tapping the share button\n4. Copying the link (it looks like: https://app.playtomic.io/profile/user/XXXXXXX)\n\nJust paste the URL here and I'll link your Slack account to your Playtomic profile!"

	// Send DM to the new member
	_, _, err := notifier.SendDirectMessage(slackUserID, welcomeText)
	if err != nil {
		return err
	}

	log.Info("Welcome message sent successfully", "user", slackUserID)
	return nil
}

// handleReactionAdded processes emoji reactions for availability collection
func handleReactionAdded(slackUserID, reaction, messageTimestamp string, store club.ClubStore, matchmakingService matchmaking.MatchmakingService) error {
	log.Info("Processing reaction added", "user", slackUserID, "reaction", reaction, "timestamp", messageTimestamp)
	
	// Check if this is a reaction to an active match request message
	requestID, isActiveRequest, err := matchmakingService.IsActiveMatchRequestMessage(messageTimestamp)
	if err != nil {
		log.Error("Failed to check if message is active match request", "error", err, "timestamp", messageTimestamp)
		return fmt.Errorf("failed to check if message is active match request: %w", err)
	}

	log.Info("Checked active match request", "requestID", requestID, "isActive", isActiveRequest, "timestamp", messageTimestamp)

	if !isActiveRequest {
		log.Debug("Reaction not on active match request, ignoring", "user", slackUserID, "reaction", reaction, "timestamp", messageTimestamp)
		return nil
	}

	// Convert emoji reaction to day of week
	day := emojiToDay(reaction)
	log.Info("Converted reaction to day", "reaction", reaction, "day", day)
	if day == "" {
		log.Debug("Reaction not a day emoji, ignoring", "user", slackUserID, "reaction", reaction)
		return nil
	}

	// Find player by Slack user ID using the player mapper
	mapper := club.NewPlayerMapper(store)
	foundPlayer, _, err := mapper.FindOrMapPlayer(slackUserID, "", "")
	if err != nil {
		log.Error("Failed to find player", "error", err, "user", slackUserID)
		return fmt.Errorf("failed to find player: %w", err)
	}

	log.Info("Found player for reaction", "player", foundPlayer, "user", slackUserID)

	if foundPlayer == nil {
		log.Warn("Player not found for reaction, ignoring", "user", slackUserID)
		return nil
	}

	// Add player availability
	log.Info("About to add player availability", "requestID", requestID, "playerID", foundPlayer.ID, "playerName", foundPlayer.Name, "day", day)
	err = matchmakingService.AddPlayerAvailability(requestID, foundPlayer.ID, foundPlayer.Name, day)
	if err != nil {
		log.Error("Failed to add player availability", "error", err, "requestID", requestID, "playerID", foundPlayer.ID, "day", day)
		return fmt.Errorf("failed to add player availability: %w", err)
	}

	log.Info("Successfully added player availability from reaction", "player", foundPlayer.Name, "day", day, "requestID", requestID)
	return nil
}

// handleReactionRemoved processes emoji reaction removals for availability collection
func handleReactionRemoved(slackUserID, reaction, messageTimestamp string, store club.ClubStore, matchmakingService matchmaking.MatchmakingService) error {
	// Check if this is a reaction to an active match request message
	requestID, isActiveRequest, err := matchmakingService.IsActiveMatchRequestMessage(messageTimestamp)
	if err != nil {
		return fmt.Errorf("failed to check if message is active match request: %w", err)
	}

	if !isActiveRequest {
		log.Debug("Reaction removal not on active match request, ignoring", "user", slackUserID, "reaction", reaction, "timestamp", messageTimestamp)
		return nil
	}

	// Convert emoji reaction to day of week
	day := emojiToDay(reaction)
	if day == "" {
		log.Debug("Reaction removal not a day emoji, ignoring", "user", slackUserID, "reaction", reaction)
		return nil
	}

	// Find player by Slack user ID using the player mapper
	mapper := club.NewPlayerMapper(store)
	foundPlayer, _, err := mapper.FindOrMapPlayer(slackUserID, "", "")
	if err != nil {
		return fmt.Errorf("failed to find player: %w", err)
	}

	if foundPlayer == nil {
		log.Warn("Player not found for reaction removal, ignoring", "user", slackUserID)
		return nil
	}

	// Remove player availability
	err = matchmakingService.RemovePlayerAvailability(requestID, foundPlayer.ID, day)
	if err != nil {
		return fmt.Errorf("failed to remove player availability: %w", err)
	}

	log.Info("Removed player availability from reaction", "player", foundPlayer.Name, "day", day, "requestID", requestID)
	return nil
}

// emojiToDay converts emoji reactions to day strings
func emojiToDay(reaction string) string {
	emojiDayMap := map[string]string{
		"one":   "Monday",
		"two":   "Tuesday",
		"three": "Wednesday",
		"four":  "Thursday",
		"five":  "Friday",
		"six":   "Saturday",
		"seven": "Sunday",
	}

	return emojiDayMap[reaction]
}
