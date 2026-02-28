package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/mauv0809/ideal-tribble/internal/charts"
	"github.com/mauv0809/ideal-tribble/internal/club"
	"github.com/mauv0809/ideal-tribble/internal/pairings"
	"github.com/mauv0809/ideal-tribble/internal/playtomic"
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
	clubStore     club.ClubStore
	fetchService  FetchService
}

// NewHandlers creates a new web handlers instance.
func NewHandlers(middleware *Middleware, pairingsStore pairings.PairingsStore, clubStore club.ClubStore, fetchService FetchService) *Handlers {
	return &Handlers{
		middleware:    middleware,
		pairingsStore: pairingsStore,
		clubStore:     clubStore,
		fetchService:  fetchService,
	}
}

// Dashboard renders the main dashboard.
func (h *Handlers) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	// Parse period from query param (default 30 days)
	period := 30
	periodLabel := "Last 30 Days"
	if periodStr := r.URL.Query().Get("period"); periodStr != "" {
		if p, err := strconv.Atoi(periodStr); err == nil {
			switch p {
			case 7:
				period = 7
				periodLabel = "Last 7 Days"
			case 30:
				period = 30
				periodLabel = "Last 30 Days"
			case 90:
				period = 90
				periodLabel = "Last 90 Days"
			case 0:
				period = 0
				periodLabel = "All Time"
			}
		}
	}

	// Gather stats
	allPairings, _ := h.pairingsStore.GetTrackedPairings()
	activePairings, _ := h.pairingsStore.GetActivePairings()
	totalMatches, _ := h.pairingsStore.GetTotalMatchCount()
	recentMatches, _ := h.pairingsStore.GetRecentMatchesAllPairings(5)
	lastFetchTimestamp, _ := h.pairingsStore.GetLastFetchTimestamp()

	// Get period-specific stats
	var periodMatchCount int
	var mostActive, bestPerforming *pairings.PairingHighlight
	if period > 0 {
		periodMatchCount, _ = h.pairingsStore.GetRecentMatchCount(period)
		mostActive, _ = h.pairingsStore.GetMostActivePairing(period)
		bestPerforming, _ = h.pairingsStore.GetBestPerformingPairing(period, 3)
	} else {
		// All time
		periodMatchCount = totalMatches
		mostActive, _ = h.pairingsStore.GetMostActivePairing(0)
		bestPerforming, _ = h.pairingsStore.GetBestPerformingPairing(0, 3)
	}

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
		PeriodMatchCount:      periodMatchCount,
		RecentMatchesList:     dashboardMatches,
		LastFetchTimestamp:    lastFetchTimestamp,
		MostActivePairing:     mostActiveHighlight,
		BestPerformingPairing: bestPerformingHighlight,
		SelectedPeriod:        period,
		PeriodLabel:           periodLabel,
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
		// Fetch recent form (last 5 matches)
		if matches, err := h.pairingsStore.GetPairingRecentMatches(p.ID, 5); err == nil && len(matches) > 0 {
			// Reverse to show oldest to newest (left to right)
			form := make([]bool, len(matches))
			for j := 0; j < len(matches); j++ {
				form[j] = matches[len(matches)-1-j].PairingWon
			}
			pairingsWithStats[i].RecentForm = form
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
		// Fetch recent form (last 5 matches)
		if matches, err := h.pairingsStore.GetPairingRecentMatches(p.ID, 5); err == nil && len(matches) > 0 {
			form := make([]bool, len(matches))
			for j := 0; j < len(matches); j++ {
				form[j] = matches[len(matches)-1-j].PairingWon
			}
			pairingsWithStats[i].RecentForm = form
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

	// Get situational stats
	situationalStats, _ := h.pairingsStore.GetSituationalStats(id)

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

	// Build charts
	winRateTrendChart := h.buildWinRateTrendChart(id)
	dayOfWeekChart := h.buildDayOfWeekChart(id)
	hourOfDayChart := h.buildHourOfDayChart(id)

	data := templates.PairingDetailData{
		PageData: templates.PageData{
			Title: pairing.Player1Name + " & " + pairing.Player2Name,
			User:  user,
		},
		Pairing:           *pairing,
		Stats:             stats,
		SituationalStats:  situationalStats,
		RecentMatches:     recentMatches,
		CurrentPage:       page,
		TotalPages:        totalPages,
		TotalMatches:      totalMatches,
		WinRateTrendChart: winRateTrendChart,
		DayOfWeekChart:    dayOfWeekChart,
		HourOfDayChart:    hourOfDayChart,
	}

	component := templates.PairingDetailPage(data)
	_ = component.Render(r.Context(), w)
}

// buildWinRateTrendChart creates a line chart showing rolling win rate over recent matches.
func (h *Handlers) buildWinRateTrendChart(pairingID int64) template.HTML {
	// Get last 20 matches (oldest first for chronological order)
	matches, err := h.pairingsStore.GetPairingRecentMatches(pairingID, 20)
	if err != nil || len(matches) < 8 {
		return "" // Need at least 8 matches for a meaningful trend chart
	}

	// Reverse to get chronological order (oldest first)
	for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
		matches[i], matches[j] = matches[j], matches[i]
	}

	// Calculate rolling win rate - use smaller window for smaller datasets
	windowSize := 5
	if len(matches) < 12 {
		windowSize = 3 // Use 3-match window for smaller datasets
	}

	points := make([]charts.DataPoint, 0)
	var wins int

	for i, m := range matches {
		if m.PairingWon {
			wins++
		}

		// Start tracking after we have enough for a window
		if i >= windowSize-1 {
			// Remove oldest match from window if we're past the initial window
			if i >= windowSize && matches[i-windowSize].PairingWon {
				wins--
			}

			winRate := float64(wins) / float64(windowSize) * 100
			points = append(points, charts.DataPoint{
				Label: fmt.Sprintf("%d", i+1),
				Value: winRate,
				Meta: map[string]string{
					"match": fmt.Sprintf("Match %d", i+1),
				},
			})
		}
	}

	if len(points) < 5 {
		return "" // Need at least 5 data points for a meaningful trend
	}

	config := charts.DefaultConfig()
	config.Width = 400
	config.Height = 200
	config.PaddingTop = 20
	config.PaddingBottom = 30
	config.PaddingLeft = 40
	config.PaddingRight = 20
	config.ShowLegend = false

	chart := charts.NewLineChart().
		SetConfig(config).
		WithArea(true).
		WithSmooth(true).
		AddDataPoints("Win Rate %", points)

	svg, err := chart.Render()
	if err != nil {
		return ""
	}
	return svg
}

// buildDayOfWeekChart creates a bar chart showing wins/losses by day of week.
func (h *Handlers) buildDayOfWeekChart(pairingID int64) template.HTML {
	timeStats, err := h.pairingsStore.GetPairingTimeStats(pairingID)
	if err != nil || timeStats == nil {
		return ""
	}

	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	wins := make([]int, 7)
	losses := make([]int, 7)
	hasData := false

	for day := 0; day < 7; day++ {
		if stats, ok := timeStats.ByDayOfWeek[day]; ok && stats.MatchesPlayed > 0 {
			wins[day] = stats.MatchesWon
			losses[day] = stats.MatchesLost
			hasData = true
		}
	}

	if !hasData {
		return ""
	}

	config := charts.DefaultConfig()
	config.Width = 400
	config.Height = 220
	config.PaddingTop = 20
	config.PaddingBottom = 30
	config.PaddingLeft = 40
	config.PaddingRight = 20
	config.ShowLegend = true

	chart := charts.NewWinLossBarChart()
	chart.SetConfig(config)
	chart.SetWinLossData(dayNames, wins, losses)

	svg, err := chart.Render()
	if err != nil {
		return ""
	}
	return svg
}

// buildHourOfDayChart creates a bar chart showing wins/losses by time of day.
func (h *Handlers) buildHourOfDayChart(pairingID int64) template.HTML {
	timeStats, err := h.pairingsStore.GetPairingTimeStats(pairingID)
	if err != nil || timeStats == nil {
		return ""
	}

	// Morning (6-12), Afternoon (12-18), Evening (18-24)
	timeRanges := []string{"Morning", "Afternoon", "Evening"}
	wins := make([]int, 3)
	losses := make([]int, 3)
	hasData := false

	for hourRange, stats := range timeStats.ByHourRange {
		if stats.MatchesPlayed > 0 {
			hasData = true
			idx := -1
			switch hourRange {
			case "morning":
				idx = 0
			case "afternoon":
				idx = 1
			case "evening":
				idx = 2
			}
			if idx >= 0 {
				wins[idx] = stats.MatchesWon
				losses[idx] = stats.MatchesLost
			}
		}
	}

	if !hasData {
		return ""
	}

	config := charts.DefaultConfig()
	config.Width = 400
	config.Height = 220
	config.PaddingTop = 20
	config.PaddingBottom = 30
	config.PaddingLeft = 40
	config.PaddingRight = 20
	config.ShowLegend = true

	chart := charts.NewWinLossBarChart()
	chart.SetConfig(config)
	chart.SetWinLossData(timeRanges, wins, losses)

	svg, err := chart.Render()
	if err != nil {
		return ""
	}
	return svg
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

	// Build H2H trend chart
	h2hTrendChart := h.buildH2HTrendChart(matches)

	data := templates.OpponentDetailData{
		PageData: templates.PageData{
			Title: "vs " + oppStats.Opponent1Name + " & " + oppStats.Opponent2Name,
			User:  user,
		},
		Pairing:       *pairing,
		OpponentStats: *oppStats,
		Matches:       oppMatches,
		RecentForm:    recentForm,
		H2HTrendChart: h2hTrendChart,
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

	// Build charts
	h2hTrendChart := h.buildH2HTrendChart(matches)
	partnerEffectChart := h.buildPartnerEffectChart(partners)

	data := templates.IndividualPlayerDetailData{
		PageData: templates.PageData{
			Title: "vs " + playerStats.PlayerName,
			User:  user,
		},
		Pairing:            *pairing,
		PlayerStats:        *playerStats,
		Partners:           partners,
		Matches:            playerMatches,
		RecentForm:         recentForm,
		H2HTrendChart:      h2hTrendChart,
		PartnerEffectChart: partnerEffectChart,
	}

	component := templates.IndividualPlayerDetailPage(data)
	_ = component.Render(r.Context(), w)
}

// buildPartnerEffectChart creates a bar chart showing win rate against a player by their partner.
func (h *Handlers) buildPartnerEffectChart(partners []pairings.PlayerPartnerStats) template.HTML {
	if len(partners) < 2 {
		return "" // Need at least 2 partners for comparison
	}

	// Limit to top 6 partners by matches played
	maxPartners := 6
	if len(partners) < maxPartners {
		maxPartners = len(partners)
	}

	labels := make([]string, maxPartners)
	wins := make([]int, maxPartners)
	losses := make([]int, maxPartners)

	for i := 0; i < maxPartners; i++ {
		// Truncate long names
		name := partners[i].PartnerName
		if len(name) > 10 {
			name = name[:8] + ".."
		}
		labels[i] = name
		wins[i] = partners[i].MatchesWon
		losses[i] = partners[i].MatchesLost
	}

	config := charts.DefaultConfig()
	config.Width = 400
	config.Height = 220
	config.PaddingTop = 20
	config.PaddingBottom = 30
	config.PaddingLeft = 40
	config.PaddingRight = 20
	config.ShowLegend = true

	chart := charts.NewWinLossBarChart()
	chart.SetConfig(config)
	chart.SetWinLossData(labels, wins, losses)

	svg, err := chart.Render()
	if err != nil {
		return ""
	}
	return svg
}

// buildH2HTrendChart creates a line chart showing rolling win rate over time.
func (h *Handlers) buildH2HTrendChart(matches []pairings.PairingMatch) template.HTML {
	if len(matches) < 3 {
		return "" // Not enough data for a meaningful chart
	}

	// Matches are returned newest first, reverse for chronological order
	reversed := make([]pairings.PairingMatch, len(matches))
	for i, m := range matches {
		reversed[len(matches)-1-i] = m
	}

	// Use 3-match rolling window for win rate
	windowSize := 3
	points := make([]charts.DataPoint, 0, len(reversed)-windowSize+1)

	for i := windowSize - 1; i < len(reversed); i++ {
		// Calculate win rate for the window ending at position i
		wins := 0
		for j := i - windowSize + 1; j <= i; j++ {
			if reversed[j].PairingWon {
				wins++
			}
		}
		winRate := float64(wins) / float64(windowSize) * 100

		points = append(points, charts.DataPoint{
			Label: fmt.Sprintf("%d", i+1),
			Value: winRate,
			Meta: map[string]string{
				"match": fmt.Sprintf("Match %d", i+1),
			},
		})
	}

	if len(points) < 2 {
		return "" // Need at least 2 points for a line
	}

	config := charts.DefaultConfig()
	config.Width = 400
	config.Height = 200
	config.PaddingTop = 20
	config.PaddingBottom = 30
	config.PaddingLeft = 40
	config.PaddingRight = 20
	config.ShowLegend = false

	chart := charts.NewLineChart().
		SetConfig(config).
		WithArea(true).
		WithSmooth(true).
		AddDataPoints("Win Rate", points)

	svg, err := chart.Render()
	if err != nil {
		return ""
	}
	return svg
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

// NewMatchForm renders the manual match entry form.
func (h *Handlers) NewMatchForm(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	// Get all known players for suggestions
	allPlayers, _ := h.clubStore.GetAllPlayers()

	data := templates.MatchEntryData{
		PageData: templates.PageData{
			Title: "Enter Match",
			User:  user,
		},
		AllPlayers: allPlayers,
	}

	// Check for pairing pre-fill
	if pairingIDStr := r.URL.Query().Get("pairing"); pairingIDStr != "" {
		if pairingID, err := strconv.ParseInt(pairingIDStr, 10, 64); err == nil {
			if pairing, err := h.pairingsStore.GetPairingByID(pairingID); err == nil && pairing != nil {
				data.PairingID = pairingID
				data.Team1Player1 = &templates.PrefilledPlayer{
					ID:   pairing.Player1ID,
					Name: pairing.Player1Name,
				}
				data.Team1Player2 = &templates.PrefilledPlayer{
					ID:   pairing.Player2ID,
					Name: pairing.Player2Name,
				}
			}
		}
	}

	component := templates.MatchEntryPage(data)
	_ = component.Render(r.Context(), w)
}

// CreateManualMatch handles the form submission for manual match entry.
func (h *Handlers) CreateManualMatch(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	if err := r.ParseForm(); err != nil {
		h.renderMatchFormWithError(w, r, "Invalid form data")
		return
	}

	// Parse date and time
	dateStr := r.FormValue("match_date")
	timeStr := r.FormValue("match_time")
	if dateStr == "" {
		h.renderMatchFormWithError(w, r, "Match date is required")
		return
	}

	dateTimeStr := dateStr
	if timeStr != "" {
		dateTimeStr = dateStr + " " + timeStr
	} else {
		dateTimeStr = dateStr + " 12:00"
	}

	matchDate, err := time.Parse("2006-01-02 15:04", dateTimeStr)
	if err != nil {
		h.renderMatchFormWithError(w, r, "Invalid date format")
		return
	}

	// Parse match type
	matchType := r.FormValue("match_type")
	var matchTypeEnum playtomic.MatchTypeEnum
	switch matchType {
	case "singles":
		matchTypeEnum = playtomic.MatchTypeEnumSingles
	case "doubles":
		matchTypeEnum = playtomic.MatchTypeEnumDoubles
	default:
		matchTypeEnum = playtomic.MatchTypeEnumDoubles
	}

	competitionMode := r.FormValue("competition_mode")
	if competitionMode != "COMPETITIVE" && competitionMode != "FRIENDLY" {
		competitionMode = "FRIENDLY"
	}

	venueName := r.FormValue("venue_name")

	// Parse players
	team1Players := []club.ManualPlayerInput{
		{ID: r.FormValue("team1_player1_id"), Name: r.FormValue("team1_player1_name")},
	}
	team2Players := []club.ManualPlayerInput{
		{ID: r.FormValue("team2_player1_id"), Name: r.FormValue("team2_player1_name")},
	}

	if matchTypeEnum == playtomic.MatchTypeEnumDoubles {
		team1Players = append(team1Players, club.ManualPlayerInput{
			ID: r.FormValue("team1_player2_id"), Name: r.FormValue("team1_player2_name"),
		})
		team2Players = append(team2Players, club.ManualPlayerInput{
			ID: r.FormValue("team2_player2_id"), Name: r.FormValue("team2_player2_name"),
		})
	}

	// Validate player names
	for i, p := range team1Players {
		if p.Name == "" {
			h.renderMatchFormWithError(w, r, "Team 1 player "+(string(rune('1'+i)))+" name is required")
			return
		}
	}
	for i, p := range team2Players {
		if p.Name == "" {
			h.renderMatchFormWithError(w, r, "Team 2 player "+(string(rune('1'+i)))+" name is required")
			return
		}
	}

	// Parse sets
	var sets []club.SetScoreInput
	for i := 1; i <= 3; i++ {
		t1Str := r.FormValue("set" + strconv.Itoa(i) + "_team1")
		t2Str := r.FormValue("set" + strconv.Itoa(i) + "_team2")

		if t1Str == "" && t2Str == "" {
			continue // Skip empty sets
		}

		t1, err1 := strconv.Atoi(t1Str)
		t2, err2 := strconv.Atoi(t2Str)
		if err1 != nil || err2 != nil {
			continue // Skip invalid sets
		}

		if t1 < 0 || t1 > 7 || t2 < 0 || t2 > 7 {
			h.renderMatchFormWithError(w, r, "Set scores must be between 0 and 7")
			return
		}

		sets = append(sets, club.SetScoreInput{
			Team1Games: t1,
			Team2Games: t2,
		})
	}

	if len(sets) == 0 {
		h.renderMatchFormWithError(w, r, "At least one set score is required")
		return
	}

	// Create the manual match input
	input := &club.ManualMatchInput{
		MatchDate:       matchDate,
		VenueName:       venueName,
		MatchTypeEnum:   matchTypeEnum,
		CompetitionMode: competitionMode,
		Team1Players:    team1Players,
		Team2Players:    team2Players,
		Sets:            sets,
	}

	// Create the match
	match, err := h.clubStore.CreateManualMatch(input, user.Email)
	if err != nil {
		h.renderMatchFormWithError(w, r, "Failed to create match: "+err.Error())
		return
	}

	// Update player stats for this match
	h.clubStore.UpdatePlayerStats(match)
	h.clubStore.UpdateWeeklyStats(match)

	// Set flash message and redirect
	h.middleware.SetFlash(w, r, "success", "Match created successfully")
	http.Redirect(w, r, "/matches/manual", http.StatusSeeOther)
}

func (h *Handlers) renderMatchFormWithError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	user := GetUser(r)
	allPlayers, _ := h.clubStore.GetAllPlayers()

	data := templates.MatchEntryData{
		PageData: templates.PageData{
			Title: "Enter Match",
			User:  user,
		},
		AllPlayers: allPlayers,
		Error:      errorMsg,
	}

	component := templates.MatchEntryPage(data)
	_ = component.Render(r.Context(), w)
}

// ManualMatchesList shows all manually entered matches.
func (h *Handlers) ManualMatchesList(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	matches, _ := h.clubStore.GetManualMatches()

	data := templates.ManualMatchesData{
		PageData: templates.PageData{
			Title: "Manual Matches",
			User:  user,
		},
		Matches: matches,
	}

	component := templates.ManualMatchesPage(data)
	_ = component.Render(r.Context(), w)
}

// SuggestPlayers returns player suggestions for autocomplete.
func (h *Handlers) SuggestPlayers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" || len(query) < 2 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return
	}

	suggestions, err := h.clubStore.SuggestPlayersForName(query, 5)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return
	}

	component := templates.PlayerSuggestions(suggestions)
	w.Header().Set("Content-Type", "text/html")
	_ = component.Render(r.Context(), w)
}

// SuggestVenues returns venue name suggestions for autocomplete.
func (h *Handlers) SuggestVenues(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return
	}

	venues, err := h.clubStore.GetDistinctVenues(query, 8)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(""))
		return
	}

	component := templates.VenueSuggestions(venues)
	w.Header().Set("Content-Type", "text/html")
	_ = component.Render(r.Context(), w)
}

// PlayersList shows all players with their alias status.
func (h *Handlers) PlayersList(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)

	// Get Playtomic players
	playtomicPlayers, _ := h.clubStore.GetAllPlayers()

	// Get manual player aliases
	aliases, _ := h.clubStore.GetAllPlayerAliases()

	data := templates.PlayersListData{
		PageData: templates.PageData{
			Title: "Players",
			User:  user,
		},
		PlaytomicPlayers: playtomicPlayers,
		ManualAliases:    aliases,
	}

	component := templates.PlayersListPage(data)
	_ = component.Render(r.Context(), w)
}

// LinkPlayer links a manual player to a Playtomic player.
func (h *Handlers) LinkPlayer(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	manualID := r.PathValue("id")
	playtomicID := r.FormValue("playtomic_id")
	playtomicName := r.FormValue("playtomic_name")

	if manualID == "" || playtomicID == "" {
		http.Error(w, "Missing player IDs", http.StatusBadRequest)
		return
	}

	err := h.clubStore.LinkPlayerAlias(manualID, playtomicID, playtomicName, true, 1.0)
	if err != nil {
		http.Error(w, "Failed to link player: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success for HTMX
	w.Header().Set("HX-Redirect", "/players")
	w.WriteHeader(http.StatusOK)
}

// UnlinkPlayer removes the link between a manual and Playtomic player.
func (h *Handlers) UnlinkPlayer(w http.ResponseWriter, r *http.Request) {
	manualID := r.PathValue("id")
	if manualID == "" {
		http.Error(w, "Missing player ID", http.StatusBadRequest)
		return
	}

	err := h.clubStore.UnlinkPlayerAlias(manualID)
	if err != nil {
		http.Error(w, "Failed to unlink player: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success for HTMX
	w.Header().Set("HX-Redirect", "/players")
	w.WriteHeader(http.StatusOK)
}
