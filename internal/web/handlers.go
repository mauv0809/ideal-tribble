package web

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mauv0809/ideal-tribble/internal/pairings"
	"github.com/mauv0809/ideal-tribble/internal/web/templates"
)

// FetchService defines the interface for fetching matches.
type FetchService interface {
	FetchMatches(days int) (clubMatches int, pairingMatches int, err error)
}

// Handlers contains the HTTP handlers for web pages.
type Handlers struct {
	middleware    *Middleware
	pairingsStore pairings.PairingsStore
	fetchService  FetchService
}

// NewHandlers creates a new web handlers instance.
func NewHandlers(middleware *Middleware, pairingsStore pairings.PairingsStore, fetchService FetchService) *Handlers {
	return &Handlers{
		middleware:    middleware,
		pairingsStore: pairingsStore,
		fetchService:  fetchService,
	}
}

// Dashboard renders the main dashboard.
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	// Gather stats
	allPairings, _ := h.pairingsStore.GetTrackedPairings()
	activePairings, _ := h.pairingsStore.GetActivePairings()
	totalMatches, _ := h.pairingsStore.GetTotalMatchCount()
	recentMatchCount, _ := h.pairingsStore.GetRecentMatchCount(30)
	recentMatches, _ := h.pairingsStore.GetRecentMatchesAllPairings(5)
	lastFetchTimestamp, _ := h.pairingsStore.GetLastFetchTimestamp()

	// Get pairing highlights (last 30 days)
	mostActive, _ := h.pairingsStore.GetMostActivePairing(30)
	bestPerforming, _ := h.pairingsStore.GetBestPerformingPairing(30, 3) // Min 3 matches

	// Build a map of pairing ID to pairing name for display
	pairingNames := make(map[int64]string)
	for _, p := range allPairings {
		pairingNames[p.ID] = p.Player1Name + " & " + p.Player2Name
	}

	// Convert to dashboard matches
	var dashboardMatches []templates.DashboardMatch
	for _, m := range recentMatches {
		dashboardMatches = append(dashboardMatches, templates.DashboardMatch{
			MatchID:       m.MatchID,
			PairingID:     m.PairingID,
			PairingName:   pairingNames[m.PairingID],
			MatchDateUnix: m.MatchDate,
			Won:           m.PairingWon,
			SetsWon:       m.SetsWon,
			SetsLost:      m.SetsLost,
			OpponentName:  m.Opponent1Name + " & " + m.Opponent2Name,
		})
	}

	// Convert pairing highlights to template format
	var mostActiveHighlight *templates.PairingHighlight
	if mostActive != nil {
		mostActiveHighlight = &templates.PairingHighlight{
			PairingID:     mostActive.PairingID,
			PairingName:   mostActive.PairingName,
			MatchesPlayed: mostActive.MatchesPlayed,
			MatchesWon:    mostActive.MatchesWon,
			WinPercentage: mostActive.WinPercentage,
		}
	}

	var bestPerformingHighlight *templates.PairingHighlight
	if bestPerforming != nil {
		bestPerformingHighlight = &templates.PairingHighlight{
			PairingID:     bestPerforming.PairingID,
			PairingName:   bestPerforming.PairingName,
			MatchesPlayed: bestPerforming.MatchesPlayed,
			MatchesWon:    bestPerforming.MatchesWon,
			WinPercentage: bestPerforming.WinPercentage,
		}
	}

	data := templates.DashboardData{
		PageData: templates.PageData{
			Title: "Dashboard",
			User:  user,
		},
		TotalPairings:         len(allPairings),
		ActivePairings:        len(activePairings),
		TotalMatches:          totalMatches,
		RecentMatchCount:      recentMatchCount,
		RecentMatchesList:     dashboardMatches,
		LastFetchTimestamp:    lastFetchTimestamp,
		MostActivePairing:     mostActiveHighlight,
		BestPerformingPairing: bestPerformingHighlight,
	}

	if flashes := h.middleware.GetFlash(w, r, "success"); len(flashes) > 0 {
		data.PageData.FlashSuccess = flashes[0]
	}

	component := templates.DashboardPage(data)
	_ = component.Render(r.Context(), w)
}

// PairingsList renders the list of tracked pairings.
func (h *Handlers) PairingsList(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	allPairings, _ := h.pairingsStore.GetTrackedPairings()

	// Build pairings with stats
	pairingsWithStats := make([]templates.PairingWithStats, len(allPairings))
	for i, p := range allPairings {
		pairingsWithStats[i] = templates.PairingWithStats{
			TrackedPairing: p,
		}
		// Fetch stats for this pairing
		if stats, err := h.pairingsStore.GetPairingOverallStats(p.ID); err == nil && stats != nil {
			pairingsWithStats[i].MatchesPlayed = stats.MatchesPlayed
			pairingsWithStats[i].MatchesWon = stats.MatchesWon
			pairingsWithStats[i].WinPercentage = stats.WinPercentage
		}
	}

	data := templates.PairingsListData{
		PageData: templates.PageData{
			Title: "Pairings",
			User:  user,
		},
		Pairings: pairingsWithStats,
	}

	if flashes := h.middleware.GetFlash(w, r, "success"); len(flashes) > 0 {
		data.PageData.FlashSuccess = flashes[0]
	}
	if flashes := h.middleware.GetFlash(w, r, "error"); len(flashes) > 0 {
		data.PageData.FlashError = flashes[0]
	}

	component := templates.PairingsListPage(data)
	_ = component.Render(r.Context(), w)
}

// PairingsTablePartial renders just the pairings table for htmx updates.
func (h *Handlers) PairingsTablePartial(w http.ResponseWriter, r *http.Request) {
	allPairings, _ := h.pairingsStore.GetTrackedPairings()

	// Build pairings with stats
	pairingsWithStats := make([]templates.PairingWithStats, len(allPairings))
	for i, p := range allPairings {
		pairingsWithStats[i] = templates.PairingWithStats{
			TrackedPairing: p,
		}
		if stats, err := h.pairingsStore.GetPairingOverallStats(p.ID); err == nil && stats != nil {
			pairingsWithStats[i].MatchesPlayed = stats.MatchesPlayed
			pairingsWithStats[i].MatchesWon = stats.MatchesWon
			pairingsWithStats[i].WinPercentage = stats.WinPercentage
		}
	}

	component := templates.PairingsTable(pairingsWithStats)
	_ = component.Render(r.Context(), w)
}

// NewPairingPage renders the add new pairing form.
func (h *Handlers) NewPairingPage(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	// Get all known players for autocomplete
	players, _ := h.pairingsStore.GetAllKnownPlayers()

	data := templates.NewPairingData{
		PageData: templates.PageData{
			Title: "Add Pairing",
			User:  user,
		},
		Players: players,
	}

	component := templates.NewPairingPage(data)
	_ = component.Render(r.Context(), w)
}

// CreatePairing handles adding a new pairing.
func (h *Handlers) CreatePairing(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	player1ID := r.FormValue("player1_id")
	player1Name := r.FormValue("player1_name")
	player2ID := r.FormValue("player2_id")
	player2Name := r.FormValue("player2_name")

	if player1ID == "" || player1Name == "" || player2ID == "" || player2Name == "" {
		user := GetUser(r)
		players, _ := h.pairingsStore.GetAllKnownPlayers()
		data := templates.NewPairingData{
			PageData: templates.PageData{
				Title: "Add Pairing",
				User:  user,
			},
			Error:   "All fields are required",
			Players: players,
		}
		component := templates.NewPairingPage(data)
		_ = component.Render(r.Context(), w)
		return
	}

	_, err := h.pairingsStore.AddTrackedPairing(player1ID, player1Name, player2ID, player2Name)
	if err != nil {
		user := GetUser(r)
		players, _ := h.pairingsStore.GetAllKnownPlayers()
		data := templates.NewPairingData{
			PageData: templates.PageData{
				Title: "Add Pairing",
				User:  user,
			},
			Error:   "Failed to add pairing: " + err.Error(),
			Players: players,
		}
		component := templates.NewPairingPage(data)
		_ = component.Render(r.Context(), w)
		return
	}

	_ = h.middleware.SetFlash(w, r, "success", "Pairing added successfully")
	http.Redirect(w, r, "/pairings", http.StatusSeeOther)
}

// DeletePairing handles removing a pairing.
func (h *Handlers) DeletePairing(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	_ = h.pairingsStore.RemoveTrackedPairing(id)

	// Return updated table for htmx
	h.PairingsTablePartial(w, r)
}

// ActivatePairing activates a pairing.
func (h *Handlers) ActivatePairing(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	_ = h.pairingsStore.ActivatePairing(id)
	h.PairingsTablePartial(w, r)
}

// DeactivatePairing deactivates a pairing.
func (h *Handlers) DeactivatePairing(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	_ = h.pairingsStore.DeactivatePairing(id)
	h.PairingsTablePartial(w, r)
}

// PairingDetail shows detailed stats for a pairing.
func (h *Handlers) PairingDetail(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	pairing, err := h.pairingsStore.GetPairingByID(id)
	if err != nil {
		http.Error(w, "Pairing not found", http.StatusNotFound)
		return
	}

	statsPtr, _ := h.pairingsStore.GetPairingOverallStats(id)
	var stats pairings.PairingStats
	if statsPtr != nil {
		stats = *statsPtr
	}

	// Pagination
	const pageSize = 10
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	totalMatches, _ := h.pairingsStore.GetPairingMatchCount(id)
	totalPages := (totalMatches + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize
	recentMatches := h.getMatchesPaginated(id, pageSize, offset)

	data := templates.PairingDetailData{
		PageData: templates.PageData{
			Title: pairing.Player1Name + " & " + pairing.Player2Name,
			User:  user,
		},
		Pairing:       *pairing,
		Stats:         stats,
		RecentMatches: recentMatches,
		CurrentPage:   page,
		TotalPages:    totalPages,
		TotalMatches:  totalMatches,
	}

	component := templates.PairingDetailPage(data)
	_ = component.Render(r.Context(), w)
}

// MatchHistoryPartial returns the match history partial for HTMX requests.
func (h *Handlers) MatchHistoryPartial(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	filter := r.URL.Query().Get("filter")
	if filter != "" && filter != "wins" && filter != "losses" {
		filter = ""
	}

	const pageSize = 10
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	totalMatches, _ := h.pairingsStore.GetPairingMatchCountFiltered(id, filter)
	totalPages := (totalMatches + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	offset := (page - 1) * pageSize
	matches, _ := h.pairingsStore.GetPairingMatchesFiltered(id, filter, pageSize, offset)

	data := templates.MatchHistoryData{
		PairingID:    id,
		Matches:      h.convertMatches(matches),
		CurrentPage:  page,
		TotalPages:   totalPages,
		TotalMatches: totalMatches,
		Filter:       filter,
	}

	component := templates.MatchHistoryPartial(data)
	_ = component.Render(r.Context(), w)
}

func (h *Handlers) getRecentMatches(pairingID int64, limit int) []templates.RecentMatch {
	matches, err := h.pairingsStore.GetPairingRecentMatches(pairingID, limit)
	if err != nil {
		return nil
	}
	return h.convertMatches(matches)
}

func (h *Handlers) getMatchesPaginated(pairingID int64, limit, offset int) []templates.RecentMatch {
	matches, err := h.pairingsStore.GetPairingMatchesPaginated(pairingID, limit, offset)
	if err != nil {
		return nil
	}
	return h.convertMatches(matches)
}

func (h *Handlers) convertMatches(matches []pairings.PairingMatch) []templates.RecentMatch {
	result := make([]templates.RecentMatch, len(matches))
	for i, m := range matches {
		result[i] = templates.RecentMatch{
			MatchID:       m.MatchID,
			MatchDateUnix: m.MatchDate,
			Won:           m.PairingWon,
			SetsWon:       m.SetsWon,
			SetsLost:      m.SetsLost,
			Opponent1Name: m.Opponent1Name,
			Opponent2Name: m.Opponent2Name,
			TenantName:    m.TenantName,
		}
	}
	return result
}

// MatchDetail shows detailed information about a specific match.
func (h *Handlers) MatchDetail(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	matchID := r.PathValue("matchID")
	if matchID == "" {
		http.Error(w, "Match ID required", http.StatusBadRequest)
		return
	}

	// Get pairing ID from query param (for context/back link)
	var pairingID int64
	var pairingName string
	if pStr := r.URL.Query().Get("pairing"); pStr != "" {
		if p, err := strconv.ParseInt(pStr, 10, 64); err == nil {
			pairingID = p
			if pairing, err := h.pairingsStore.GetPairingByID(p); err == nil {
				pairingName = pairing.Player1Name + " & " + pairing.Player2Name
			}
		}
	}

	// If no pairing specified, try to find the match in any pairing
	if pairingID == 0 {
		// Try all pairings to find the match
		allPairings, _ := h.pairingsStore.GetTrackedPairings()
		for _, p := range allPairings {
			if match, err := h.pairingsStore.GetPairingMatchByID(p.ID, matchID); err == nil && match != nil {
				pairingID = p.ID
				pairingName = p.Player1Name + " & " + p.Player2Name
				break
			}
		}
	}

	if pairingID == 0 {
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}

	match, err := h.pairingsStore.GetPairingMatchByID(pairingID, matchID)
	if err != nil || match == nil {
		http.Error(w, "Match not found", http.StatusNotFound)
		return
	}

	// Get head-to-head history against same opponents
	h2hMatches, _ := h.pairingsStore.GetHeadToHead(pairingID, match.Opponent1ID, match.Opponent2ID)
	h2h := h.convertMatches(h2hMatches)

	// Calculate H2H record
	var h2hWins, h2hLosses int
	for _, m := range h2hMatches {
		if m.PairingWon {
			h2hWins++
		} else {
			h2hLosses++
		}
	}

	data := templates.MatchDetailData{
		PageData: templates.PageData{
			Title: "Match Details",
			User:  user,
		},
		Match:       *match,
		PairingID:   pairingID,
		PairingName: pairingName,
		HeadToHead:  h2h,
		H2HWins:     h2hWins,
		H2HLosses:   h2hLosses,
	}

	component := templates.MatchDetailPage(data)
	_ = component.Render(r.Context(), w)
}

// OpponentStats shows opponent breakdown for a pairing.
func (h *Handlers) OpponentStats(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	pairing, err := h.pairingsStore.GetPairingByID(id)
	if err != nil {
		http.Error(w, "Pairing not found", http.StatusNotFound)
		return
	}

	opponents, _ := h.pairingsStore.GetPairingVsOpponentStats(id)

	data := templates.OpponentStatsData{
		PageData: templates.PageData{
			Title: "Opponent Breakdown",
			User:  user,
		},
		Pairing:   *pairing,
		Opponents: opponents,
	}

	component := templates.OpponentStatsPage(data)
	_ = component.Render(r.Context(), w)
}

// OpponentDetail shows detailed stats against a specific opponent pair.
func (h *Handlers) OpponentDetail(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	opp1ID := r.PathValue("opp1")
	opp2ID := r.PathValue("opp2")
	if opp1ID == "" || opp2ID == "" {
		http.Error(w, "Missing opponent IDs", http.StatusBadRequest)
		return
	}

	pairing, err := h.pairingsStore.GetPairingByID(id)
	if err != nil {
		http.Error(w, "Pairing not found", http.StatusNotFound)
		return
	}

	// Get stats for this specific opponent
	oppStats, err := h.pairingsStore.GetPairingVsSpecificOpponent(id, opp1ID, opp2ID)
	if err != nil {
		http.Error(w, "Opponent stats not found", http.StatusNotFound)
		return
	}

	// Get match history against this opponent
	matches, _ := h.pairingsStore.GetHeadToHead(id, opp1ID, opp2ID)

	// Convert to template format
	var oppMatches []templates.OpponentMatch
	for _, m := range matches {
		oppMatches = append(oppMatches, templates.OpponentMatch{
			MatchID:       m.MatchID,
			MatchDateUnix: m.MatchDate,
			Won:           m.PairingWon,
			SetsWon:       m.SetsWon,
			SetsLost:      m.SetsLost,
		})
	}

	// Build recent form (last 5 results, oldest first so most recent is on right)
	var recentForm []bool
	start := 0
	if len(matches) > 5 {
		start = len(matches) - 5
	}
	for i := start; i < len(matches); i++ {
		recentForm = append(recentForm, matches[i].PairingWon)
	}

	data := templates.OpponentDetailData{
		PageData: templates.PageData{
			Title: "vs " + oppStats.Opponent1Name + " & " + oppStats.Opponent2Name,
			User:  user,
		},
		Pairing:       *pairing,
		OpponentStats: *oppStats,
		Matches:       oppMatches,
		RecentForm:    recentForm,
	}

	component := templates.OpponentDetailPage(data)
	_ = component.Render(r.Context(), w)
}

// IndividualPlayers shows individual player breakdown for a pairing.
func (h *Handlers) IndividualPlayers(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	pairing, err := h.pairingsStore.GetPairingByID(id)
	if err != nil {
		http.Error(w, "Pairing not found", http.StatusNotFound)
		return
	}

	players, _ := h.pairingsStore.GetIndividualOpponentStats(id)

	data := templates.IndividualPlayersData{
		PageData: templates.PageData{
			Title: "Individual Players",
			User:  user,
		},
		Pairing: *pairing,
		Players: players,
	}

	component := templates.IndividualPlayersPage(data)
	_ = component.Render(r.Context(), w)
}

// IndividualPlayerDetail shows detailed stats against a specific individual player.
func (h *Handlers) IndividualPlayerDetail(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	playerID := r.PathValue("playerID")
	if playerID == "" {
		http.Error(w, "Missing player ID", http.StatusBadRequest)
		return
	}

	pairing, err := h.pairingsStore.GetPairingByID(id)
	if err != nil {
		http.Error(w, "Pairing not found", http.StatusNotFound)
		return
	}

	// Get stats for this specific player
	playerStats, partners, err := h.pairingsStore.GetIndividualOpponentDetail(id, playerID)
	if err != nil || playerStats == nil {
		http.Error(w, "Player stats not found", http.StatusNotFound)
		return
	}

	// Get match history against this player
	matches, _ := h.pairingsStore.GetMatchesVsIndividualOpponent(id, playerID)

	// Convert to template format, determining partner for each match
	var playerMatches []templates.IndividualPlayerMatch
	for _, m := range matches {
		// Determine the partner (the opponent who is NOT the player we're looking at)
		partnerName := m.Opponent2Name
		if m.Opponent1ID != playerID {
			partnerName = m.Opponent1Name
		}
		playerMatches = append(playerMatches, templates.IndividualPlayerMatch{
			MatchID:       m.MatchID,
			MatchDateUnix: m.MatchDate,
			Won:           m.PairingWon,
			SetsWon:       m.SetsWon,
			SetsLost:      m.SetsLost,
			PartnerName:   partnerName,
		})
	}

	// Build recent form (last 5 results, oldest first so most recent is on right)
	var recentForm []bool
	start := 0
	if len(matches) > 5 {
		start = len(matches) - 5
	}
	for i := start; i < len(matches); i++ {
		recentForm = append(recentForm, matches[i].PairingWon)
	}

	data := templates.IndividualPlayerDetailData{
		PageData: templates.PageData{
			Title: "vs " + playerStats.PlayerName,
			User:  user,
		},
		Pairing:     *pairing,
		PlayerStats: *playerStats,
		Partners:    partners,
		Matches:     playerMatches,
		RecentForm:  recentForm,
	}

	component := templates.IndividualPlayerDetailPage(data)
	_ = component.Render(r.Context(), w)
}

// FetchMatches handles manual match fetching from Playtomic.
func (h *Handlers) FetchMatches(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid form data"})
		return
	}

	daysStr := r.FormValue("days")
	days, err := strconv.Atoi(daysStr)
	if err != nil || days < 1 || days > 365 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Days must be between 1 and 365"})
		return
	}

	if h.fetchService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Fetch service not available"})
		return
	}

	clubMatches, pairingMatches, err := h.fetchService.FetchMatches(days)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "Fetch completed successfully",
		"club_matches":   clubMatches,
		"pairing_matches": pairingMatches,
		"days":           days,
	})
}
