package handlers

import (
	"fmt"
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
)

// TestReactHandler simulates emoji reactions for testing purposes
// This bypasses Slack user mapping and works directly with player names
func TestReactHandler(store club.ClubStore, matchmakingService matchmaking.MatchmakingService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}

		playerName := r.FormValue("player_name")
		emoji := r.FormValue("emoji")

		if playerName == "" || emoji == "" {
			http.Error(w, "Missing required parameters: player_name, emoji", http.StatusBadRequest)
			return
		}

		log.Info("Test reaction received", "player", playerName, "emoji", emoji)

		// Convert emoji to day
		day := emojiToDay(emoji)
		if day == "" {
			http.Error(w, fmt.Sprintf("Invalid emoji '%s'. Valid options: one, two, three, four, five, six, seven", emoji), http.StatusBadRequest)
			return
		}

		// Find player by name in database
		players, err := store.FindPlayersByNameSimilarity(playerName)
		if err != nil {
			log.Error("Failed to find player", "error", err, "name", playerName)
			http.Error(w, "Failed to search for player", http.StatusInternalServerError)
			return
		}

		if len(players) == 0 {
			http.Error(w, fmt.Sprintf("Player not found: %s", playerName), http.StatusNotFound)
			return
		}

		// Use the first match (should be exact or very close)
		foundPlayer := players[0]

		log.Info("Found player for test reaction", "player", foundPlayer.Name, "id", foundPlayer.ID)

		// Get the active match request (we'll assume there's only one for testing)
		activeRequests, err := matchmakingService.GetActiveMatchRequests()
		if err != nil {
			log.Error("Failed to get active match requests", "error", err)
			http.Error(w, "Failed to get active match requests", http.StatusInternalServerError)
			return
		}

		if len(activeRequests) == 0 {
			http.Error(w, "No active match requests found. Create a match request first with /match command.", http.StatusNotFound)
			return
		}

		// Use the most recent active request
		request := activeRequests[0]
		log.Info("Adding availability to active match request", "requestID", request.ID, "player", foundPlayer.Name, "day", day)

		// Add player availability
		err = matchmakingService.AddPlayerAvailability(request.ID, foundPlayer.ID, foundPlayer.Name, day)
		if err != nil {
			log.Error("Failed to add player availability", "error", err)
			http.Error(w, "Failed to add player availability", http.StatusInternalServerError)
			return
		}

		log.Info("Successfully added test availability", "player", foundPlayer.Name, "day", day, "requestID", request.ID)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "success", "message": "Added %s availability for %s to match request %s"}`, day, foundPlayer.Name, request.ID)
	}
}