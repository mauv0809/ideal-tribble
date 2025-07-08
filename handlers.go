package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/mauv0809/ideal-tribble/playtomic"
	"github.com/mauv0809/ideal-tribble/slack"

	"github.com/charmbracelet/log"
	"github.com/rafa-garcia/go-playtomic-api/client"
	"github.com/rafa-garcia/go-playtomic-api/models"
)

func healthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received request to /health. Checking...")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK!")
	}
}

func clearStoreHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID := r.URL.Query().Get("matchID")
		if matchID != "" {
			log.Info("Received request to /clear for a specific match.", "matchID", matchID)
			store.ClearMatch(matchID)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Cleared match %s from store!", matchID)
			log.Info("Successfully cleared match from store.", "matchID", matchID)
		} else {
			log.Info("Received request to /clear. Clearing entire store...")
			store.Clear()
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Store cleared!")
			log.Info("Store cleared successfully.")
		}
	}
}

func fetchPlayer(playerID string) {
	// This function is a placeholder for fetching updated player information from the Playtomic API.
	// We will implement this in the future.
	log.Info("Fetching player data from API...", "playerID", playerID)
}

func listMembersHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refresh := r.URL.Query().Get("refresh") == "true"
		if refresh {
			players, err := store.GetAllPlayers()
			if err != nil {
				http.Error(w, "Failed to get players for refresh", http.StatusInternalServerError)
				return
			}
			for _, player := range players {
				fetchPlayer(player.ID)
			}
		}

		players, err := store.GetAllPlayers()
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

func listMatchesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matches, err := store.GetAllMatches()
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

// checkAndNotifyHandler is the main HTTP handler for our logic.
func checkAndNotifyHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Received request to /check. Fetching bookings...")
		debug := r.URL.Query().Get("debug") == "true"
		ctx := context.Background()
		// 1. Initialize Playtomic Client
		playtomicAPIClient := client.NewClient(
			client.WithTimeout(10*time.Second),
			client.WithRetries(3),
		)
		//2. setup search params
		params := &models.SearchMatchesParams{
			SportID:       cfg.BookingFilter,
			HasPlayers:    true,
			Sort:          "start_date,ASC",
			TenantIDs:     []string{cfg.TenantID},
			FromStartDate: time.Now().Format("2006-01-02") + "T00:00:00",
		}
		log.Infof("Seaching from: %s", params.FromStartDate)
		// 2. Fetch Upcoming Bookings
		matches, err := playtomicAPIClient.GetMatches(ctx, params)
		if err != nil {
			log.Error("Error fetching Playtomic bookings: %v", err)
			http.Error(w, "Error fetching Playtomic bookings", http.StatusInternalServerError)
			return
		}
		log.Infof("Found %d matches from API", len(matches))
		var wg sync.WaitGroup
		matchChan := make(chan playtomic.PadelMatch)
		done := make(chan bool)

		go processMatches(store, matchChan, done, debug)

		playtomicClient := playtomic.NewClient()

		// 3. Filter for specific matches and send Slack notifications
		for _, match := range matches {
			// We only want to notify for matches in the future.
			ownerID := match.OwnerID
			if ownerID != nil {
				if store.IsKnownPlayer(*ownerID) {
					wg.Add(1)
					go func(matchID string) {
						defer wg.Done()
						specificMatch, err := playtomicClient.GetSpecificMatch(matchID)
						if err != nil {
							log.Error("Error fetching specific match: %v", err)
							return
						}

						isClubMatch := false
						if store.IsInitialPlayer(specificMatch.OwnerID) {
							isClubMatch = true
							log.Info("Match booked by initial player, processing.", "matchID", matchID, "owner", specificMatch.OwnerName)
						} else {
							knownPlayers := 0
							totalPlayers := 0
							for _, team := range specificMatch.Teams {
								totalPlayers += len(team.Players)
								for _, player := range team.Players {
									if store.IsKnownPlayer(player.UserID) {
										knownPlayers++
									}
								}
							}

							if totalPlayers >= 4 && knownPlayers >= 3 { // Doubles
								isClubMatch = true
								log.Info("Match meets doubles threshold.", "matchID", matchID, "knownPlayers", knownPlayers, "totalPlayers", totalPlayers)
							} else if totalPlayers < 4 && knownPlayers >= 2 { // Singles
								isClubMatch = true
								log.Info("Match meets singles threshold.", "matchID", matchID, "knownPlayers", knownPlayers, "totalPlayers", totalPlayers)
							}
						}

						if isClubMatch {
							for _, team := range specificMatch.Teams {
								for _, player := range team.Players {
									store.AddPlayer(player.UserID, player.Name)
								}
							}
							matchChan <- specificMatch
						} else {
							log.Info("Skipping match as it does not meet club criteria.", "matchID", matchID)
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
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	}
}

func processMatches(store *ClubStore, matchChan <-chan playtomic.PadelMatch, done chan<- bool, debug bool) {
	for match := range matchChan {
		log.Info("Processing match", "id", match.MatchID)
		existingMatch, existingState, found := store.GetMatchState(match.MatchID)
		if !found {
			log.Info("New match found, processing for booking notification", "id", match.MatchID)
			// This is a new match. The ball bringer will be assigned here.
			state := &MatchNotificationState{BookingNotified: true, ResultNotified: false}
			store.SetMatchState(&match, state)
			go slack.SendBookingNotification(&match, debug)
		} else {
			// We have seen this match before. We need to enrich the current match object
			// with the ball bringer info from the stored match.
			match.BallBringerID = existingMatch.BallBringerID
			match.BallBringerName = existingMatch.BallBringerName

			if match.GameStatus == "ended" && !existingState.ResultNotified {
				log.Info("Match finished, processing for result notification", "id", match.MatchID)
				existingState.ResultNotified = true
				store.SetMatchState(&match, existingState)
				go slack.SendResultNotification(&match, debug)
			} else {
				log.Info("Match already processed or not finished yet", "id", match.MatchID)
			}
		}
	}
	done <- true
}
