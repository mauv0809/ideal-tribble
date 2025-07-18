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
	"github.com/mauv0809/ideal-tribble/internal/metrics"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
	"github.com/mauv0809/ideal-tribble/internal/processor"
)

func FetchMatchesHandler(store club.ClubStore, metrics metrics.Metrics, cfg config.Config, playtomicClient playtomic.PlaytomicClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Info("Starting match fetch...")
		metrics.IncFetcherRuns()
		isDryRun := IsDryRunFromContext(r)

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

		var clubMatchesToUpsert []*playtomic.PadelMatch
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, match := range matches {
			wg.Add(1)
			go func(matchID string) {
				defer wg.Done()
				if match.OwnerID == nil || !store.IsKnownPlayer(*match.OwnerID) {
					log.Debug("Skipping non-club match", "matchID", matchID)
					return
				}
				specificMatch, err := playtomicClient.GetSpecificMatch(matchID)
				if err != nil {
					log.Error("Error fetching specific match", "matchID", matchID, "error", err)
					return
				}

				if !isClubMatch(specificMatch, store) {
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
				if err := store.UpsertMatches(clubMatchesToUpsert); err != nil {
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
