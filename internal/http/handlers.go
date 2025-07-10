package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	slackclient "github.com/slack-go/slack"
)

func (s *Server) MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Received metrics request")
		metrics, err := s.Metrics.GetAll()
		if err != nil {
			log.Error("Failed to get metrics", "error", err)
			http.Error(w, "Failed to retrieve metrics", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metrics); err != nil {
			log.Error("Failed to write metrics response", "error", err)
		}
	}
}

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
		s.Metrics.Increment("fetcher_runs")
		isDryRun := isDryRunFromContext(r)

		params := &playtomic.SearchMatchesParams{
			SportID:       s.Cfg.BookingFilter,
			HasPlayers:    true,
			Sort:          "start_date,ASC",
			TenantIDs:     []string{s.Cfg.TenantID},
			FromStartDate: time.Now().Format("2006-01-02") + "T00:00:00",
		}

		matches, err := s.PlaytomicClient.GetMatches(params)
		if err != nil {
			log.Error("Error fetching Playtomic bookings", "error", err)
			http.Error(w, "Failed to fetch matches", http.StatusInternalServerError)
			return
		}

		log.Info("Found matches from API", "count", len(matches))
		var clubMatches int
		var wg sync.WaitGroup
		for _, match := range matches {
			if match.OwnerID == nil || !s.Store.IsKnownPlayer(*match.OwnerID) {
				continue
			}
			wg.Add(1)
			go func(matchID string) {
				defer wg.Done()
				specificMatch, err := s.PlaytomicClient.GetSpecificMatch(matchID)
				if err != nil {
					log.Error("Error fetching specific match", "matchID", matchID, "error", err)
					return
				}

				if !isClubMatch(specificMatch, s.Store) {
					log.Debug("Skipping non-club match", "matchID", matchID)
					return
				}

				for _, team := range specificMatch.Teams {
					for _, player := range team.Players {
						s.Store.AddPlayer(player.UserID, player.Name, player.Level)
					}
				}

				if !isDryRun {
					if err := s.Store.UpsertMatch(&specificMatch); err != nil {
						log.Error("Failed to upsert match", "error", err, "matchID", specificMatch.MatchID)
					} else {
						log.Info("Successfully upserted match", "matchID", specificMatch.MatchID)
						clubMatches++
					}
				} else {
					log.Info("[Dry Run] Would have upserted match", "matchID", specificMatch.MatchID)
				}
			}(match.MatchID)
		}
		wg.Wait()

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Match fetch completed.")
		log.Info("Match fetch finished.", "matches", len(matches), "clubMatches", clubMatches)
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

	if totalPlayers >= 4 && knownPlayers >= 3 {
		return true
	}
	if totalPlayers > 0 && totalPlayers < 4 && knownPlayers == totalPlayers {
		return true
	}
	return false
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

// LeaderboardCommandHandler returns a handler for the /leaderboard Slack command.
func (s *Server) LeaderboardCommandHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := s.Store.GetPlayerStats()
		if err != nil {
			http.Error(w, "Failed to get player stats", http.StatusInternalServerError)
			log.Error("Failed to get player stats from store", "error", err)
			return
		}

		message := s.SlackClient.FormatLeaderboard(stats)
		s.SlackClient.SendMessage(message, s.Metrics, isDryRunFromContext(r))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(message); err != nil {
			log.Error("Failed to encode leaderboard message to JSON", "error", err)
		}
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
		// Example curl command to fetch stats for "Morten Voss":
		// curl -X POST -d "text=Morten Voss" http://localhost:8080/slack/command/player-stats
		log.Info("Received player stats command", "player", playerName)

		stats, err := s.Store.GetPlayerStatsByName(playerName)
		var message slackclient.Message
		if err != nil {
			log.Warn("Could not find player stats", "player", playerName, "error", err)
			message = s.SlackClient.FormatPlayerNotFound(playerName)
		} else {
			message = s.SlackClient.FormatPlayerStats(stats, playerName)
		}
		s.SlackClient.SendMessage(message, s.Metrics, isDryRunFromContext(r))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(message); err != nil {
			log.Error("Failed to encode player stats message to JSON", "error", err)
		}
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

		message := s.SlackClient.FormatLevelLeaderboard(players)
		s.SlackClient.SendMessage(message, s.Metrics, isDryRunFromContext(r))
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(message); err != nil {
			log.Error("Failed to encode level leaderboard message to JSON", "error", err)
		}
	}
}
