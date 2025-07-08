package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/club"
	"github.com/mauv0809/ideal-tribble/config"
	"github.com/mauv0809/ideal-tribble/playtomic"
	"github.com/mauv0809/ideal-tribble/slack"
	"github.com/rafa-garcia/go-playtomic-api/client"
	"github.com/rafa-garcia/go-playtomic-api/models"
)

type Server struct {
	Store *club.Store
	Cfg   config.Config
}

func NewServer(store *club.Store, cfg config.Config) *Server {
	return &Server{
		Store: store,
		Cfg:   cfg,
	}
}

func (s *Server) HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debug("Received request to /health")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK!")
	}
}

func (s *Server) ClearStoreHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.URL.Query().Get("matchID")
		if matchID != "" {
			log.Info("Received request to /clear for a specific match.", "matchID", matchID)
			s.Store.ClearMatch(matchID)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Cleared match %s from store!", matchID)
			log.Info("Successfully cleared match from store.", "matchID", matchID)
		} else {
			log.Info("Received request to /clear. Clearing entire store...")
			s.Store.Clear()
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Store cleared!")
			log.Info("Store cleared successfully.")
		}
	}
}

func fetchPlayer(playerID string) {
	// This function is a placeholder for fetching updated player information from the Playtomic API.
	// We will implement this in the future.
	log.Debug("Fetching player data from API...", "playerID", playerID)
}

func (s *Server) ListMembersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refresh := r.URL.Query().Get("refresh") == "true"
		if refresh {
			players, err := s.Store.GetAllPlayers()
			if err != nil {
				http.Error(w, "Failed to get players for refresh", http.StatusInternalServerError)
				return
			}
			for _, player := range players {
				fetchPlayer(player.ID)
			}
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
			log.Error("Failed to write response", "error", err)
		}
	}
}

// RunCheck encapsulates the core logic of fetching and processing matches.
func (s *Server) RunCheck(debug bool, verbose bool) {
	if verbose {
		originalLevel := log.GetLevel()
		log.SetLevel(log.DebugLevel)
		defer log.SetLevel(originalLevel)
	}

	log.Info("Running check...")
	ctx := context.Background()
	// 1. Initialize Playtomic Client
	playtomicAPIClient := client.NewClient(
		client.WithTimeout(10*time.Second),
		client.WithRetries(3),
	)
	//2. setup search params
	params := &models.SearchMatchesParams{
		SportID:       s.Cfg.BookingFilter,
		HasPlayers:    true,
		Sort:          "start_date,ASC",
		TenantIDs:     []string{s.Cfg.TenantID},
		FromStartDate: time.Now().Format("2006-01-02") + "T00:00:00",
	}
	log.Debugf("Searching from: %s", params.FromStartDate)
	// 2. Fetch Upcoming Bookings
	matches, err := playtomicAPIClient.GetMatches(ctx, params)
	if err != nil {
		log.Error("Error fetching Playtomic bookings: %v", err)
		return
	}
	log.Debugf("Found %d matches from API", len(matches))
	var wg sync.WaitGroup
	matchChan := make(chan playtomic.PadelMatch)
	done := make(chan bool)

	go s.processMatches(matchChan, done, debug)

	playtomicClient := playtomic.NewClient()

	// 3. Filter for specific matches and send Slack notifications
	for _, match := range matches {
		// We only want to notify for matches in the future.
		ownerID := match.OwnerID
		if ownerID != nil {
			if s.Store.IsKnownPlayer(*ownerID) {
				wg.Add(1)
				go func(matchID string) {
					defer wg.Done()
					specificMatch, err := playtomicClient.GetSpecificMatch(matchID)
					if err != nil {
						log.Error("Error fetching specific match: %v", err)
						return
					}

					isClubMatch := false
					if s.Store.IsInitialPlayer(specificMatch.OwnerID) {
						isClubMatch = true
						log.Debug("Match booked by initial player, processing.", "matchID", matchID, "owner", specificMatch.OwnerName)
					} else {
						knownPlayers := 0
						totalPlayers := 0
						for _, team := range specificMatch.Teams {
							totalPlayers += len(team.Players)
							for _, player := range team.Players {
								if s.Store.IsKnownPlayer(player.UserID) {
									knownPlayers++
								}
							}
						}

						if totalPlayers >= 4 && knownPlayers >= 3 { // Doubles
							isClubMatch = true
							log.Debug("Match meets doubles threshold.", "matchID", matchID, "knownPlayers", knownPlayers, "totalPlayers", totalPlayers)
						} else if totalPlayers < 4 && knownPlayers >= 2 { // Singles
							isClubMatch = true
							log.Debug("Match meets singles threshold.", "matchID", matchID, "knownPlayers", knownPlayers, "totalPlayers", totalPlayers)
						}
					}

					if isClubMatch {
						for _, team := range specificMatch.Teams {
							for _, player := range team.Players {
								s.Store.AddPlayer(player.UserID, player.Name)
							}
						}
						matchChan <- specificMatch
					} else {
						log.Debug("Skipping match as it does not meet club criteria.", "matchID", matchID)
					}
				}(match.MatchID)
			}
		}
	}

	go func() {
		wg.Wait()
		close(matchChan)
	}()

	<-done // Wait for processMatches to finish.

	log.Infof("Check complete.")
}

// checkAndNotifyHandler is the main HTTP handler for our logic.
func (s *Server) CheckAndNotifyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		debug := r.URL.Query().Get("debug") == "true"
		verbose := r.URL.Query().Get("verbose") == "true"
		go s.RunCheck(debug, verbose) // Run in a goroutine to not block the HTTP response
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Check initiated.")
	}
}

func (s *Server) processMatches(matchChan <-chan playtomic.PadelMatch, done chan<- bool, debug bool) {
	for match := range matchChan {
		log.Debug("Processing match", "id", match.MatchID)
		existingMatch, existingState, found := s.Store.GetMatchState(match.MatchID)
		if !found {
			// This is a new match. We determine the correct state and which notification to send.
			state := &club.MatchNotificationState{BookingNotified: true}
			var notificationFunc func(*playtomic.PadelMatch, bool)

			if match.GameStatus == playtomic.GameStatusPlayed {
				log.Info("New, already-finished match found. Sending result notification.", "id", match.MatchID)
				state.ResultNotified = true
				notificationFunc = slack.SendResultNotification
			} else {
				log.Info("New match found, processing for booking notification", "id", match.MatchID)
				state.ResultNotified = false // This is the default, but explicit is good.
				notificationFunc = slack.SendBookingNotification
			}

			// Now we perform the actions.
			s.Store.SetMatchState(&match, state)
			go notificationFunc(&match, debug)
		} else {
			// We have seen this match before. We need to enrich the current match object
			// with the ball bringer info from the stored match.
			match.BallBringerID = existingMatch.BallBringerID
			match.BallBringerName = existingMatch.BallBringerName

			if match.GameStatus == playtomic.GameStatusPlayed && !existingState.ResultNotified {
				log.Info("Match finished, processing for result notification", "id", match.MatchID)
				existingState.ResultNotified = true
				s.Store.SetMatchState(&match, existingState)
				go slack.SendResultNotification(&match, debug)
			} else {
				if match.GameStatus == playtomic.GameStatusPlayed {
					log.Debug("Match already processed", "id", match.MatchID)
				} else {
					log.Debug("Match not finished yet", "id", match.MatchID)
				}
			}
		}
	}
	done <- true
}
