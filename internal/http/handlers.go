package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"io"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/slack-go/slack"
)

func (s *Server) HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Received health check request")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK!")
	}
}

func (s *Server) ClearStoreHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.URL.Query().Get("matchID")
		if matchID != "" {
			log.Info("Received request to clear a specific match", "matchID", matchID)
			s.Store.ClearMatch(matchID)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Cleared match %s from store!", matchID)
			log.Info("Successfully cleared match from store", "matchID", matchID)
		} else {
			log.Info("Received request to clear entire store")
			s.Store.Clear()
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Store cleared!")
			log.Info("Store cleared successfully")
		}
	}
}

func (s *Server) FetchMatchesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Starting match fetch...")
		s.Metrics.IncFetcherRuns()
		isDryRun := isDryRunFromContext(r)

		daysStr := r.URL.Query().Get("days")
		daysToSubtract := 0
		if daysStr != "" {
			parsedDays, err := strconv.Atoi(daysStr)
			if err == nil && parsedDays > 0 {
				daysToSubtract = parsedDays
				log.Info("Fetching historical matches", "days", daysToSubtract)
			} else {
				log.Warn("Invalid 'days' parameter provided. Defaulting to 0.", "days_param", daysStr)
			}
		}

		startDate := time.Now().AddDate(0, 0, -daysToSubtract)

		params := &playtomic.SearchMatchesParams{
			SportID:       "PADEL",
			HasPlayers:    true,
			Sort:          "start_date,ASC",
			TenantIDs:     []string{s.Cfg.TenantID},
			FromStartDate: startDate.Format("2006-01-02") + "T00:00:00",
		}
		log.Info("Fetching matches from", "startDate", startDate)
		matches, err := s.PlaytomicClient.GetMatches(params)
		if err != nil {
			log.Error("Error fetching Playtomic bookings", "error", err)
			http.Error(w, "Failed to fetch matches", http.StatusInternalServerError)
			return
		}

		log.Info("Found matches from API", "count", len(matches))

		var clubMatchesToUpsert []*playtomic.PadelMatch
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, match := range matches {

			wg.Add(1)
			go func(matchID string) {
				defer wg.Done()
				if match.OwnerID == nil || !s.Store.IsKnownPlayer(*match.OwnerID) {
					log.Debug("Skipping non-club match", "matchID", matchID)
					return
				}
				specificMatch, err := s.PlaytomicClient.GetSpecificMatch(matchID)
				if err != nil {
					log.Error("Error fetching specific match", "matchID", matchID, "error", err)
					return
				}

				if !isClubMatch(specificMatch, s.Store) {
					log.Debug("Skipping non-club match", "matchID", matchID)
					return
				}

				mu.Lock()
				clubMatchesToUpsert = append(clubMatchesToUpsert, &specificMatch)
				mu.Unlock()
			}(match.MatchID)
		}
		wg.Wait()

		if len(clubMatchesToUpsert) > 0 {
			if !isDryRun {
				log.Info("Upserting club matches", "count", len(clubMatchesToUpsert))
				if err := s.Store.UpsertMatches(clubMatchesToUpsert); err != nil {
					log.Error("Failed to bulk upsert matches", "error", err)
					http.Error(w, "Failed to save matches", http.StatusInternalServerError)
					return
				}
			} else {
				log.Info("[Dry Run] Would have upserted club matches", "count", len(clubMatchesToUpsert))
			}
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Match fetch completed.")
		log.Info("Match fetch finished.", "total_api_matches", len(matches), "club_matches_found", len(clubMatchesToUpsert))
	}
}

func isClubMatch(match playtomic.PadelMatch, store club.ClubStore) bool {
	knownPlayers := 0
	totalPlayers := 0
	for _, team := range match.Teams {
		totalPlayers += len(team.Players)
		for _, player := range team.Players {
			if store.IsKnownPlayer(player.UserID) {
				knownPlayers++
			}
		}
	}

	if totalPlayers >= 4 && knownPlayers >= 4 {
		return true
	}
	if totalPlayers > 0 && totalPlayers < 4 && knownPlayers == totalPlayers {
		return true
	}
	return false
}
func (s *Server) BallBoyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Received ball boy message", "body", string(bodyBytes))
		// Define a small struct to decode the incoming JSON's `data` field
		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"` // base64-encoded message payload
				// You can add other fields if needed
			} `json:"message"`
		}

		// Parse the outer JSON to get `data`
		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		// Decode base64 to raw MessagePack bytes
		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := isDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		s.pubsub.ProcessMessage(rawData, &match)
		s.Processor.AssignBallBringer(&match, isDryRun)
		w.Write([]byte("OK"))
	}
}
func (s *Server) UpdatePlayerStatsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Received update player stats message", "body", string(bodyBytes))
		// Define a small struct to decode the incoming JSON's `data` field
		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"` // base64-encoded message payload
				// You can add other fields if needed
			} `json:"message"`
		}

		// Parse the outer JSON to get `data`
		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		// Decode base64 to raw MessagePack bytes
		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := isDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		s.pubsub.ProcessMessage(rawData, &match)
		s.Processor.UpdatePlayerStats(&match, isDryRun)
		w.Write([]byte("OK"))
	}
}
func (s *Server) NotifyBookingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Recieved notify booking message", "body", string(bodyBytes))
		// Define a small struct to decode the incoming JSON's `data` field
		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"` // base64-encoded message payload
				// You can add other fields if needed
			} `json:"message"`
		}

		// Parse the outer JSON to get `data`
		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		// Decode base64 to raw MessagePack bytes
		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := isDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		s.pubsub.ProcessMessage(rawData, &match)
		err = s.Processor.NotifyBooking(&match, isDryRun)
		if err != nil {
			log.Error("Failed to notify booking", "error", err)
			http.Error(w, "Failed to notify booking", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("OK"))
	}
}
func (s *Server) NotifyResultHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Error("Failed to read request body", "error", err)
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		log.Debug("Recieved notify booking message", "body", string(bodyBytes))
		// Define a small struct to decode the incoming JSON's `data` field
		var pubsubMsg struct {
			Subscription string `json:"subscription"`
			Message      struct {
				Data string `json:"data"` // base64-encoded message payload
				// You can add other fields if needed
			} `json:"message"`
		}

		// Parse the outer JSON to get `data`
		if err := json.Unmarshal(bodyBytes, &pubsubMsg); err != nil {
			log.Error("Failed to unmarshal wrapper JSON", "error", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		// Decode base64 to raw MessagePack bytes
		rawData, err := base64.StdEncoding.DecodeString(pubsubMsg.Message.Data)
		if err != nil {
			log.Error("Failed to decode base64 data", "error", err)
			http.Error(w, "Invalid base64 data", http.StatusBadRequest)
			return
		}
		isDryRun := isDryRunFromContext(r)
		match := playtomic.PadelMatch{}
		s.pubsub.ProcessMessage(rawData, &match)
		err = s.Processor.NotifyResult(&match, isDryRun)
		if err != nil {
			log.Error("Failed to notify result", "error", err)
			http.Error(w, "Failed to notify result", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("OK"))
	}
}
func (s *Server) ProcessMatchesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Starting match processing...")
		isDryRun := isDryRunFromContext(r)

		s.Processor.ProcessMatches(isDryRun)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Match processing completed.")
		log.Info("Match processing finished.")
	}
}

func (s *Server) ListMembersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refresh := r.URL.Query().Get("refresh") == "true"
		if refresh {
			log.Warn("User triggered manual player refresh. This is deprecated and will be removed.")
		}

		players, err := s.Store.GetAllPlayers()
		if err != nil {
			http.Error(w, "Failed to get players", http.StatusInternalServerError)
			log.Error("Failed to get players from store", "error", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(players); err != nil {
			log.Error("Failed to write response", "error", err)
		}
	}
}

func (s *Server) ListMatchesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matches, err := s.Store.GetAllMatches()
		if err != nil {
			http.Error(w, "Failed to get matches", http.StatusInternalServerError)
			log.Error("Failed to get matches from store", "error", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(matches); err != nil {
			log.Error("Failed to encode matches to JSON", "error", err)
		}
	}
}

// LeaderboardHandler returns a handler that serves the player statistics leaderboard.
func (s *Server) LeaderboardHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.Store.GetPlayerStats()
		if err != nil {
			http.Error(w, "Failed to get player stats", http.StatusInternalServerError)
			log.Error("Failed to get player stats from store", "error", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(stats); err != nil {
			log.Error("Failed to encode player stats to JSON", "error", err)
		}
	}
}

// respondWithSlackMsg is a helper to format and write a Slack message as an HTTP response.
func respondWithSlackMsg(w http.ResponseWriter, msg slack.Message) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(msg); err != nil {
		log.Error("Failed to encode slack message to JSON", "error", err)
	}
}

// LeaderboardCommandHandler returns a handler for the /leaderboard Slack command.
func (s *Server) LeaderboardCommandHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.Store.GetPlayerStats()
		if err != nil {
			http.Error(w, "Failed to get player stats", http.StatusInternalServerError)
			log.Error("Failed to get player stats from store", "error", err)
			return
		}

		msg, err := s.Notifier.FormatLeaderboardResponse(stats)
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

// PlayerStatsCommandHandler returns a handler for the /player-stats Slack command.
func (s *Server) PlayerStatsCommandHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}
		playerName := r.FormValue("text")
		if playerName == "" {
			http.Error(w, "Player name is required.", http.StatusBadRequest)
			return
		}

		log.Info("Received player stats command", "player", playerName)

		stats, err := s.Store.GetPlayerStatsByName(playerName)
		var msg any
		if err != nil {
			log.Warn("Could not find player stats", "player", playerName, "error", err)
			msg, err = s.Notifier.FormatPlayerNotFoundResponse(playerName)
		} else {
			msg, err = s.Notifier.FormatPlayerStatsResponse(stats, playerName)
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

// LevelLeaderboardCommandHandler returns a handler for the /level-leaderboard Slack command.
func (s *Server) LevelLeaderboardCommandHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		players, err := s.Store.GetPlayersSortedByLevel()
		if err != nil {
			http.Error(w, "Failed to get players", http.StatusInternalServerError)
			log.Error("Failed to get players sorted by level from store", "error", err)
			return
		}

		msg, err := s.Notifier.FormatLevelLeaderboardResponse(players)
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

/*func (s *Server) SendInngestEventHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{"matchId": "1234-556435", "test": "test"}
		s.InngestClient.SendEvent("test", data)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}*/
