package web

import (
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/config"
	"github.com/mauv0809/ideal-tribble/internal/matchmaking"
	"github.com/mauv0809/ideal-tribble/internal/pairings"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
)

// DefaultFetchService implements FetchService using existing infrastructure.
type DefaultFetchService struct {
	store             club.ClubStore
	cfg               config.Config
	playtomicClient   playtomic.PlaytomicClient
	matchmakingService matchmaking.MatchmakingService
	pairingsStore     pairings.PairingsStore
}

// NewFetchService creates a new fetch service.
func NewFetchService(
	store club.ClubStore,
	cfg config.Config,
	playtomicClient playtomic.PlaytomicClient,
	matchmakingService matchmaking.MatchmakingService,
	pairingsStore pairings.PairingsStore,
) *DefaultFetchService {
	return &DefaultFetchService{
		store:             store,
		cfg:               cfg,
		playtomicClient:   playtomicClient,
		matchmakingService: matchmakingService,
		pairingsStore:     pairingsStore,
	}
}

// FetchMatches fetches matches from Playtomic for the specified number of days.
func (s *DefaultFetchService) FetchMatches(days int) (clubMatches int, pairingMatches int, err error) {
	log.Info("Starting manual match fetch from web UI", "days", days)

	startDate := time.Now().AddDate(0, 0, -days)

	params := &playtomic.SearchMatchesParams{
		SportID:       "PADEL",
		HasPlayers:    true,
		Sort:          "start_date,ASC",
		TenantIDs:     []string{s.cfg.TenantID},
		FromStartDate: startDate.Format("2006-01-02") + "T00:00:00",
	}

	matches, err := s.playtomicClient.GetMatches(params)
	if err != nil {
		log.Error("Error fetching Playtomic bookings", "error", err)
		return 0, 0, err
	}

	log.Info("Found matches from API", "count", len(matches))

	// Pre-fetch all known player IDs
	allPlayers, err := s.store.GetAllPlayers()
	if err != nil {
		log.Error("Failed to get all players for filtering", "error", err)
		return 0, 0, err
	}

	knownPlayerIDs := make(map[string]struct{}, len(allPlayers))
	for _, p := range allPlayers {
		knownPlayerIDs[p.ID] = struct{}{}
	}

	// Build tracked player IDs from active pairings for broader match fetching
	trackedPlayerIDs := make(map[string]struct{})
	if s.pairingsStore != nil {
		trackedPairings, err := s.pairingsStore.GetActivePairings()
		if err == nil {
			for _, p := range trackedPairings {
				trackedPlayerIDs[p.Player1ID] = struct{}{}
				trackedPlayerIDs[p.Player2ID] = struct{}{}
			}
			log.Info("Built tracked player IDs for fetch", "count", len(trackedPlayerIDs))
		}
	}

	clubMatchesToUpsert, allFetchedMatches := s.fetchAndFilterClubMatches(matches, knownPlayerIDs, trackedPlayerIDs)

	if len(clubMatchesToUpsert) > 0 {
		log.Info("Upserting club matches", "count", len(clubMatchesToUpsert))
		if err := s.store.UpsertMatches(clubMatchesToUpsert); err != nil {
			log.Error("Failed to bulk upsert matches", "error", err)
			return 0, 0, err
		}
		clubMatches = len(clubMatchesToUpsert)
	}

	// Detect and complete any match requests
	if s.matchmakingService != nil && len(clubMatchesToUpsert) > 0 {
		completedRequestIDs, err := s.matchmakingService.DetectMatchedRequests(clubMatchesToUpsert)
		if err != nil {
			log.Error("Failed to detect matched requests", "error", err)
		} else if len(completedRequestIDs) > 0 {
			for _, requestID := range completedRequestIDs {
				if err := s.matchmakingService.UpdateMatchRequestStatus(requestID, matchmaking.StatusCompleted); err != nil {
					log.Error("Failed to mark match request as completed", "requestID", requestID, "error", err)
				}
			}
		}
	}

	// Detect and store pairing matches
	if s.pairingsStore != nil {
		trackedPairings, err := s.pairingsStore.GetActivePairings()
		if err != nil {
			log.Error("Failed to get tracked pairings", "error", err)
		} else if len(trackedPairings) > 0 {
			log.Info("Checking for pairing matches",
				"total_fetched_matches", len(allFetchedMatches),
				"tracked_pairings", len(trackedPairings))

			// Log match details for debugging
			for _, m := range allFetchedMatches {
				log.Debug("Fetched match details",
					"matchID", m.MatchID,
					"matchType", m.MatchTypeEnum,
					"gameStatus", m.GameStatus,
					"teams", len(m.Teams))
			}

			pairingMatchList := pairings.DetectPairingMatches(allFetchedMatches, trackedPairings)
			if len(pairingMatchList) > 0 {
				log.Info("Upserting pairing matches", "count", len(pairingMatchList))
				if err := s.pairingsStore.UpsertPairingMatches(pairingMatchList); err != nil {
					log.Error("Failed to upsert pairing matches", "error", err)
				} else {
					pairingMatches = len(pairingMatchList)
				}
			} else {
				log.Info("No pairing matches detected from fetched matches")
			}
		}
	}

	log.Info("Manual match fetch completed", "club_matches", clubMatches, "pairing_matches", pairingMatches)
	return clubMatches, pairingMatches, nil
}

func (s *DefaultFetchService) fetchAndFilterClubMatches(summaries []playtomic.MatchSummary, knownPlayerIDs map[string]struct{}, trackedPlayerIDs map[string]struct{}) ([]*playtomic.PadelMatch, []*playtomic.PadelMatch) {
	var clubMatchesToUpsert []*playtomic.PadelMatch
	var allFetchedMatches []*playtomic.PadelMatch
	var mu sync.Mutex
	var wg sync.WaitGroup

	concurrencyLimit := 50
	sem := make(chan struct{}, concurrencyLimit)

	for _, summary := range summaries {
		// Pre-filter using summary data: check if ANY player is known or tracked
		hasRelevantPlayer := false
		for _, playerID := range summary.PlayerIDs {
			if _, ok := knownPlayerIDs[playerID]; ok {
				hasRelevantPlayer = true
				break
			}
			if _, ok := trackedPlayerIDs[playerID]; ok {
				hasRelevantPlayer = true
				break
			}
		}

		if !hasRelevantPlayer {
			continue // Skip matches with no relevant players
		}

		wg.Add(1)
		go func(m playtomic.MatchSummary) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			specificMatch, err := s.playtomicClient.GetSpecificMatch(m.MatchID)
			if err != nil {
				log.Error("Error fetching specific match", "matchID", m.MatchID, "error", err)
				return
			}

			mu.Lock()
			allFetchedMatches = append(allFetchedMatches, &specificMatch)

			if s.isClubMatch(specificMatch, knownPlayerIDs) {
				clubMatchesToUpsert = append(clubMatchesToUpsert, &specificMatch)
			}
			mu.Unlock()
		}(summary)
	}
	wg.Wait()

	return clubMatchesToUpsert, allFetchedMatches
}

func (s *DefaultFetchService) isClubMatch(match playtomic.PadelMatch, knownPlayerIDs map[string]struct{}) bool {
	// Must have exactly 2 teams
	if len(match.Teams) != 2 {
		return false
	}

	// Must have a valid match type (SINGLES or DOUBLES)
	if match.MatchTypeEnum != playtomic.MatchTypeEnumSingles && match.MatchTypeEnum != playtomic.MatchTypeEnumDoubles {
		return false
	}

	// Each team must have players and all players must be club members
	for _, team := range match.Teams {
		if len(team.Players) == 0 {
			return false
		}
		for _, player := range team.Players {
			if _, ok := knownPlayerIDs[player.UserID]; !ok {
				return false
			}
		}
	}
	return true
}
