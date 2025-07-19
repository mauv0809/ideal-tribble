package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/notifier"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/slack-go/slack"
)

// respondWithSlackMsg is a helper to format and write a Slack message as an HTTP response.
func respondWithSlackMsg(w http.ResponseWriter, msg slack.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(msg); err != nil {
		log.Error("Failed to encode slack message to JSON", "error", err)
	}
}

// parsePlayerStatsText parses the text field from Slack to extract player name and match type.
// Expected formats: "John Doe", "John Doe singles", "John Doe doubles", "John Doe all"
func parsePlayerStatsText(text string) (playerName string, matchType playtomic.MatchTypeEnum) {
	text = strings.TrimSpace(text)
	parts := strings.Fields(text)
	
	if len(parts) == 0 {
		return "", playtomic.MatchTypeEnumAll
	}
	
	// Default match type
	matchType = playtomic.MatchTypeEnumAll
	
	// Check if last part is a match type
	if len(parts) > 1 {
		lastPart := strings.ToLower(parts[len(parts)-1])
		switch lastPart {
		case "singles":
			matchType = playtomic.MatchTypeEnumSingles
			parts = parts[:len(parts)-1] // Remove match type from name parts
		case "doubles":
			matchType = playtomic.MatchTypeEnumDoubles
			parts = parts[:len(parts)-1] // Remove match type from name parts
		case "all":
			matchType = playtomic.MatchTypeEnumAll
			parts = parts[:len(parts)-1] // Remove match type from name parts
		}
	}
	
	// Join remaining parts as player name
	playerName = strings.Join(parts, " ")
	return playerName, matchType
}

func LeaderboardCommandHandler(store club.ClubStore, notifier notifier.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For the main leaderboard, we get combined stats from all match types.
		// A future command could be `/leaderboard singles` to specify.
		stats, err := store.GetPlayerStats(playtomic.MatchTypeEnumAll)
		if err != nil {
			http.Error(w, "Failed to get player stats", http.StatusInternalServerError)
			log.Error("Failed to get player stats from store", "error", err)
			return
		}

		msg, err := notifier.FormatLeaderboardResponse(stats)
		if err != nil {
			http.Error(w, "Failed to format leaderboard", http.StatusInternalServerError)
			log.Error("Failed to format leaderboard", "error", err)
			return
		}

		slackMsg, ok := msg.(slack.Message)
		if !ok {
			http.Error(w, "Invalid message format for Slack", http.StatusInternalServerError)
			log.Error("Failed to cast message to slack.Message")
			return
		}

		respondWithSlackMsg(w, slackMsg)
	}
}

func PlayerStatsCommandHandler(store club.ClubStore, notifier notifier.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}
		
		// Parse player name and match type from the text field
		text := r.FormValue("text")
		if text == "" {
			http.Error(w, "Player name is required.", http.StatusBadRequest)
			return
		}
		
		playerName, matchTypeEnum := parsePlayerStatsText(text)
		if playerName == "" {
			http.Error(w, "Player name is required.", http.StatusBadRequest)
			return
		}

		log.Info("Received player stats command", "player", playerName, "match_type", matchTypeEnum)
		stats, err := store.GetPlayerStatsByName(playerName, matchTypeEnum)
		var msg any
		if err != nil {
			log.Warn("Could not find player stats", "player", playerName, "error", err)
			msg, err = notifier.FormatPlayerNotFoundResponse(playerName)
		} else {
			msg, err = notifier.FormatPlayerStatsResponse(stats, playerName)
		}

		if err != nil {
			http.Error(w, "Failed to format player stats", http.StatusInternalServerError)
			log.Error("Failed to format player stats", "error", err)
			return
		}

		slackMsg, ok := msg.(slack.Message)
		if !ok {
			http.Error(w, "Invalid message format for Slack", http.StatusInternalServerError)
			log.Error("Failed to cast message to slack.Message")
			return
		}
		respondWithSlackMsg(w, slackMsg)
	}
}

func LevelLeaderboardCommandHandler(store club.ClubStore, notifier notifier.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		players, err := store.GetPlayersSortedByLevel()
		if err != nil {
			http.Error(w, "Failed to get players", http.StatusInternalServerError)
			log.Error("Failed to get players sorted by level from store", "error", err)
			return
		}

		msg, err := notifier.FormatLevelLeaderboardResponse(players)
		if err != nil {
			http.Error(w, "Failed to format level leaderboard", http.StatusInternalServerError)
			log.Error("Failed to format level leaderboard", "error", err)
			return
		}

		slackMsg, ok := msg.(slack.Message)
		if !ok {
			http.Error(w, "Invalid message format for Slack", http.StatusInternalServerError)
			log.Error("Failed to cast message to slack.Message")
			return
		}

		respondWithSlackMsg(w, slackMsg)
	}
}

func MatchCommandHandler(store club.ClubStore, notifier notifier.Notifier, matchmakingService matchmaking.MatchmakingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}

		// Get user info from Slack form data
		userName := r.FormValue("user_name")
		channelID := r.FormValue("channel_id")

		if userName == "" || channelID == "" {
			http.Error(w, "Missing required Slack form data", http.StatusBadRequest)
			return
		}

		log.Info("Received match command", "user", userName, "channel", channelID)

		// Use the new mapping system to find/map the player
		mapper := club.NewPlayerMapper(store)
		foundPlayer, suggestions, err := mapper.FindOrMapPlayer(r.FormValue("user_id"), userName, userName)
		if err != nil {
			log.Error("Failed to find/map player", "error", err)
			http.Error(w, "Failed to process match request", http.StatusInternalServerError)
			return
		}

		// If no player found and we have suggestions, send mapping confirmation
		if foundPlayer == nil && len(suggestions) > 0 {
			// For now, we'll just take the best suggestion if confidence is reasonable
			if suggestions[0].Confidence > 0.5 {
				// Auto-accept medium confidence matches
				err := store.UpdatePlayerSlackMapping(
					suggestions[0].Player.ID,
					r.FormValue("user_id"),
					userName,
					userName,
					"AUTO_MATCHED",
					suggestions[0].Confidence,
				)
				if err != nil {
					log.Error("Failed to update player mapping", "error", err)
					http.Error(w, "Failed to process match request", http.StatusInternalServerError)
					return
				}
				foundPlayer = &suggestions[0].Player
				log.Info("Auto-mapped player with medium confidence", "player", foundPlayer.Name, "confidence", suggestions[0].Confidence)
			} else {
				// Low confidence - ask user to confirm
				// For now, just return an error message
				msg, err := notifier.FormatPlayerNotFoundResponse(userName)
				if err != nil {
					log.Error("Failed to format player not found response", "error", err)
					http.Error(w, "Failed to process match request", http.StatusInternalServerError)
					return
				}

				slackMsg, ok := msg.(slack.Message)
				if !ok {
					log.Error("Failed to cast message to slack.Message")
					http.Error(w, "Invalid message format", http.StatusInternalServerError)
					return
				}

				respondWithSlackMsg(w, slackMsg)
				return
			}
		}

		if foundPlayer == nil {
			// Still no player found - this shouldn't happen if everyone in channel is a member
			msg, err := notifier.FormatPlayerNotFoundResponse(userName)
			if err != nil {
				log.Error("Failed to format player not found response", "error", err)
				http.Error(w, "Failed to process match request", http.StatusInternalServerError)
				return
			}

			slackMsg, ok := msg.(slack.Message)
			if !ok {
				log.Error("Failed to cast message to slack.Message")
				http.Error(w, "Invalid message format", http.StatusInternalServerError)
				return
			}

			respondWithSlackMsg(w, slackMsg)
			return
		}

		// Create match request using Playtomic player ID
		request, err := matchmakingService.CreateMatchRequest(foundPlayer.ID, foundPlayer.Name, channelID)
		if err != nil {
			log.Error("Failed to create match request", "error", err)
			http.Error(w, "Failed to create match request", http.StatusInternalServerError)
			return
		}

		// Send availability request message
		_, timestamp, err := notifier.SendMatchAvailabilityRequest(request, false)
		if err != nil {
			log.Error("Failed to send availability request", "error", err)
			http.Error(w, "Failed to send availability request", http.StatusInternalServerError)
			return
		}

		// Update match request with thread information
		request.ThreadTS = &timestamp
		request.AvailabilityMessageTS = &timestamp
		// Note: In a real implementation, you'd want to update these in the database

		// Format response for the user
		msg, err := notifier.FormatMatchRequestResponse(request)
		if err != nil {
			log.Error("Failed to format match request response", "error", err)
			http.Error(w, "Failed to format response", http.StatusInternalServerError)
			return
		}

		slackMsg, ok := msg.(slack.Message)
		if !ok {
			log.Error("Failed to cast message to slack.Message")
			http.Error(w, "Invalid message format", http.StatusInternalServerError)
			return
		}

		respondWithSlackMsg(w, slackMsg)
	}
}
