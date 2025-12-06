package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/pairings"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
)

func FetchMatchesHandler(store club.ClubStore, metrics metrics.Metrics, cfg config.Config, playtomicClient playtomic.PlaytomicClient, matchmakingService matchmaking.MatchmakingService, pairingsStore pairings.PairingsStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Starting match fetch...")
		metrics.IncFetcherRuns()
		isDryRun := IsDryRunFromContext(r)

		daysStr := r.URL.Query().Get("days")
		daysToSubtract := 1
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
			TenantIDs:     []string{cfg.TenantID},
			FromStartDate: startDate.Format("2006-01-02") + "T00:00:00",
		}
		log.Info("Fetching matches from", "startDate", startDate)
		matches, err := playtomicClient.GetMatches(params)
		if err != nil {
			log.Error("Error fetching Playtomic bookings", "error", err)
			http.Error(w, "Failed to fetch matches", http.StatusInternalServerError)
			return
		}

		log.Info("Found matches from API", "count", len(matches))

		// Pre-fetch all known player IDs to avoid querying the DB in a loop.
		// This is a major performance optimization.
		log.Info("Fetching all players for filtering")
		allPlayers, err := store.GetAllPlayers()
		if err != nil {
			log.Error("Failed to get all players for filtering", "error", err)
			http.Error(w, "Failed to get players", http.StatusInternalServerError)
			return
		}
		knownPlayerIDs := make(map[string]struct{}, len(allPlayers))
		for _, p := range allPlayers {
			knownPlayerIDs[p.ID] = struct{}{}
		}
		log.Info("Fetched all players for filtering", "count", len(knownPlayerIDs))

		clubMatchesToUpsert, allFetchedMatches := fetchAndFilterClubMatches(matches, knownPlayerIDs, playtomicClient)

		if len(clubMatchesToUpsert) > 0 {
			if !isDryRun {
				log.Info("Upserting club matches", "count", len(clubMatchesToUpsert))
				if err := store.UpsertMatches(clubMatchesToUpsert); err != nil {
					log.Error("Failed to bulk upsert matches", "error", err)
					http.Error(w, "Failed to save matches", http.StatusInternalServerError)
					return
				}
			} else {
				log.Info("[Dry Run] Would have upserted club matches", "count", len(clubMatchesToUpsert))
			}
		}

		// Detect and complete any match requests that match the fetched bookings
		if matchmakingService != nil && len(clubMatchesToUpsert) > 0 {
			log.Info("Checking for matches that fulfill match requests")
			completedRequestIDs, err := matchmakingService.DetectMatchedRequests(clubMatchesToUpsert)
			if err != nil {
				log.Error("Failed to detect matched requests", "error", err)
				// Don't return error - this is non-critical, continue with fetch completion
			} else if len(completedRequestIDs) > 0 {
				log.Info("Found match requests fulfilled by bookings", "count", len(completedRequestIDs))

				// Mark each matched request as completed
				for _, requestID := range completedRequestIDs {
					if !isDryRun {
						if err := matchmakingService.UpdateMatchRequestStatus(requestID, matchmaking.StatusCompleted); err != nil {
							log.Error("Failed to mark match request as completed", "requestID", requestID, "error", err)
							// Continue with other requests even if one fails
						} else {
							log.Info("Marked match request as completed", "requestID", requestID)
						}
					} else {
						log.Info("[Dry Run] Would have marked match request as completed", "requestID", requestID)
					}
				}
			} else {
				log.Debug("No match requests matched the fetched bookings")
			}
		}

		// Detect and store pairing matches for analytics
		pairingMatchCount := 0
		if pairingsStore != nil {
			trackedPairings, err := pairingsStore.GetActivePairings()
			if err != nil {
				log.Error("Failed to get tracked pairings", "error", err)
			} else if len(trackedPairings) > 0 {
				pairingMatches := pairings.DetectPairingMatches(allFetchedMatches, trackedPairings)
				if len(pairingMatches) > 0 {
					if !isDryRun {
						log.Info("Upserting pairing matches", "count", len(pairingMatches))
						if err := pairingsStore.UpsertPairingMatches(pairingMatches); err != nil {
							log.Error("Failed to upsert pairing matches", "error", err)
							// Non-critical, continue
						} else {
							pairingMatchCount = len(pairingMatches)
						}
					} else {
						log.Info("[Dry Run] Would have upserted pairing matches", "count", len(pairingMatches))
						pairingMatchCount = len(pairingMatches)
					}
				}
			}
		}

		// Update last fetch timestamp
		if pairingsStore != nil && !isDryRun {
			if err := pairingsStore.SetLastFetchTimestamp(time.Now().Unix()); err != nil {
				log.Error("Failed to update last fetch timestamp", "error", err)
			}
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Match fetch completed.")
		log.Info("Match fetch finished.", "total_api_matches", len(matches), "club_matches_found", len(clubMatchesToUpsert), "pairing_matches_found", pairingMatchCount)
	}
}

// fetchAndFilterClubMatches takes a list of match summaries and concurrently fetches
// full details, returning club matches and all fetched matches (for pairing detection).
func fetchAndFilterClubMatches(summaries []playtomic.MatchSummary, knownPlayerIDs map[string]struct{}, playtomicClient playtomic.PlaytomicClient) ([]*playtomic.PadelMatch, []*playtomic.PadelMatch) {
	var clubMatchesToUpsert []*playtomic.PadelMatch
	var allFetchedMatches []*playtomic.PadelMatch
	var mu sync.Mutex
	var wg sync.WaitGroup

	// A semaphore to limit concurrency to a reasonable number (e.g., 50).
	// This prevents overwhelming the local system or the remote API with too many parallel requests.
	concurrencyLimit := 50
	sem := make(chan struct{}, concurrencyLimit)

	for _, summary := range summaries {
		wg.Add(1)
		// Pass the match summary by value to the goroutine to avoid capturing the loop variable.
		go func(m playtomic.MatchSummary) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire a token from the semaphore
			defer func() { <-sem }() // Release the token when the goroutine finishes

			// Pre-filter: skip matches where the owner is not a known player.
			if m.OwnerID == nil {
				return
			}
			if _, ok := knownPlayerIDs[*m.OwnerID]; !ok {
				return
			}

			// Fetch full match details.
			specificMatch, err := playtomicClient.GetSpecificMatch(m.MatchID)
			if err != nil {
				log.Error("Error fetching specific match", "matchID", m.MatchID, "error", err)
				return
			}

			mu.Lock()
			// Store all fetched matches for pairing detection
			allFetchedMatches = append(allFetchedMatches, &specificMatch)

			// Filter: check if all participants are known club members.
			if isClubMatch(specificMatch, knownPlayerIDs) {
				clubMatchesToUpsert = append(clubMatchesToUpsert, &specificMatch)
			}
			mu.Unlock()
		}(summary)
	}
	wg.Wait()

	return clubMatchesToUpsert, allFetchedMatches
}

func ProcessMatchesHandler(processor *processor.Processor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Starting match processing...")
		isDryRun := IsDryRunFromContext(r)

		processor.ProcessMatches(isDryRun)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Match processing completed.")
		log.Info("Match processing finished.")
	}
}
