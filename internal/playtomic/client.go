package playtomic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/rafa-garcia/go-playtomic-api/client"
	"github.com/rafa-garcia/go-playtomic-api/models"
)

// APIClient is a custom Playtomic API client that implements the PlaytomicClient interface.
type APIClient struct {
	httpClient *http.Client
	apiClient  *client.Client
	BaseURL    string
}

// NewClient creates a new custom Playtomic client.
func NewClient() PlaytomicClient {
	return &APIClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiClient: client.NewClient(
			client.WithTimeout(10*time.Second),
			client.WithRetries(3),
		),
		BaseURL: "https://api.playtomic.io",
	}
}

// Ensure APIClient implements the PlaytomicClient interface.
var _ PlaytomicClient = (*APIClient)(nil)

// GetMatches fetches a list of matches based on the provided search parameters.
func (c *APIClient) GetMatches(params *SearchMatchesParams) ([]MatchSummary, error) {
	const pageSize = 300
	var (
		allMatches []MatchSummary
		page       = 0
	)

	for {
		externalParams := &models.SearchMatchesParams{
			SportID:       params.SportID,
			HasPlayers:    params.HasPlayers,
			Sort:          params.Sort,
			TenantIDs:     params.TenantIDs,
			FromStartDate: params.FromStartDate,
			Size:          pageSize,
			Page:          page,
		}

		log.Debug("Fetching matches from Playtomic API", "params", externalParams)
		matches, err := c.apiClient.GetMatches(context.Background(), externalParams)
		if err != nil {
			return nil, fmt.Errorf("error fetching matches from playtomic api: %w", err)
		}

		log.Info("Successfully fetched matches", "count", len(matches), "page", page)
		for _, m := range matches {
			allMatches = append(allMatches, MatchSummary{
				MatchID: m.MatchID,
				OwnerID: m.OwnerID,
			})
		}

		// If we got less than pageSize, we've reached the last page
		if len(matches) < pageSize {
			log.Info("Reached last page", "page", page)
			break
		}
		page++
	}
	log.Info("Fetched all matches", "count", len(allMatches))
	return allMatches, nil
}

// GetSpecificMatch fetches a specific match by its ID.
func (c *APIClient) GetSpecificMatch(matchID string) (PadelMatch, error) {
	url := fmt.Sprintf("%s/v1/matches/%s", c.BaseURL, matchID)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return PadelMatch{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.9")
	req.Header.Set("User-Agent", "PlaytomicGoClient/1.0")
	log.Debug("Requesting specific match from Playtomic API", "url", url)
	log.Debug(fmt.Sprintf(
		`curl -X GET '%s' -H 'Accept: */*' -H 'Content-Type: application/json' -H 'Accept-Language: en-AU,en;q=0.9' -H 'User-Agent: PlaytomicGoClient/1.0'`,
		url,
	))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PadelMatch{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error("Received non-OK HTTP status from Playtomic API", "status", resp.StatusCode, "body", string(body))
		return PadelMatch{}, fmt.Errorf("received non-OK HTTP status: %d", resp.StatusCode)
	}

	var matchResponse playtomicMatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&matchResponse); err != nil {
		return PadelMatch{}, fmt.Errorf("failed to decode response: %w", err)
	}

	paymentStatus := make(map[string]bool)
	for _, reg := range matchResponse.RegistrationInfo.Registrations {
		paymentStatus[reg.UserID] = !reg.Payable
	}

	const layout = "2006-01-02T15:04:05"

	startTime, err := time.Parse(layout, matchResponse.StartDate)
	if err != nil {
		return PadelMatch{}, fmt.Errorf("failed to parse start time: %w", err)
	}
	endTime, err := time.Parse(layout, matchResponse.EndDate)
	if err != nil {
		return PadelMatch{}, fmt.Errorf("failed to parse end time: %w", err)
	}
	createdAtTime, err := time.Parse(layout, matchResponse.CreatedAt)
	if err != nil {
		return PadelMatch{}, fmt.Errorf("failed to parse created at time: %w", err)
	}

	var teams []Team
	for _, responseTeam := range matchResponse.Teams {
		t := Team{
			ID: responseTeam.TeamID,
		}
		if responseTeam.TeamResult != nil {
			t.TeamResult = *responseTeam.TeamResult
		}
		for _, responsePlayer := range responseTeam.Players {
			t.Players = append(t.Players, Player{
				UserID: responsePlayer.UserID,
				Name:   responsePlayer.Name,
				Level: func() float64 {
					if responsePlayer.LevelValue == nil {
						zero := float64(0)
						return zero
					}
					return *responsePlayer.LevelValue
				}(),
				Paid: paymentStatus[responsePlayer.UserID],
			})
		}
		teams = append(teams, t)
	}
	ownerName := ""
	for _, team := range teams {
		for _, player := range team.Players {
			if player.UserID == matchResponse.OwnerID {
				ownerName = player.Name
				break
			}
		}
		if ownerName != "" {
			break
		}
	}

	var results []SetResult
	for _, responseResult := range matchResponse.Results {
		set := SetResult{
			Name:   responseResult.Name,
			Scores: make(map[string]int),
		}
		for _, score := range responseResult.Scores {
			set.Scores[score.TeamID] = score.Score
		}
		results = append(results, set)
	}

	var gameStatus GameStatus
	switch matchResponse.GameStatus {
	case string(GameStatusPending):
		gameStatus = GameStatusPending
	case string(GameStatusPlayed):
		gameStatus = GameStatusPlayed
	case string(GameStatusCanceled):
		gameStatus = GameStatusCanceled
	case string(GameStatusWaitingFor):
		gameStatus = GameStatusWaitingFor
	case string(GameStatusExpired):
		gameStatus = GameStatusExpired
	default:
		gameStatus = GameStatusUnknown
		log.Warn("Unknown game status received from Playtomic API", "status", matchResponse.GameStatus, "matchID", matchID)
	}
	var resultsStatus ResultsStatus
	switch matchResponse.ResultsStatus {
	case string(ResultsStatusPending):
		resultsStatus = ResultsStatusPending
	case string(ResultsStatusConfirmed):
		resultsStatus = ResultsStatusConfirmed
	case string(ResultsStatusInvalid):
		resultsStatus = ResultsStatusInvalid
	case string(ResultsStatusNotAllowed):
		resultsStatus = ResultsStatusNotAllowed
	case string(ResultsStatusExpired):
		resultsStatus = ResultsStatusExpired
	case string(ResultsStatusCanceled):
		resultsStatus = ResultsStatusCanceled
	case string(ResultsStatusWaitingFor):
		resultsStatus = ResultsStatusWaitingFor
	case string(ResultsStatusValidating):
		resultsStatus = ResultsStatusValidating
	default:
		log.Warn("Unknown results status received from Playtomic API", "status", matchResponse.ResultsStatus, "matchID", matchID)
	}

	var matchType MatchType
	switch matchResponse.MatchType {
	case string(MatchTypeCompetition):
		matchType = MatchTypeCompetition
	case string(MatchTypePractice):
		matchType = MatchTypePractice
	default:
		log.Warn("Unknown match type received from Playtomic API", "type", matchResponse.MatchType, "matchID", matchID)
	}
	padelMatch := PadelMatch{
		MatchID:       matchID,
		OwnerID:       matchResponse.OwnerID,
		OwnerName:     ownerName,
		Start:         startTime.Local().Unix(),
		End:           endTime.Local().Unix(),
		CreatedAt:     createdAtTime.Local().Unix(),
		Teams:         teams,
		GameStatus:    gameStatus,
		Status:        matchResponse.Status,
		Results:       results,
		ResultsStatus: resultsStatus,
		ResourceName:  matchResponse.ResourceName,
		Price:         matchResponse.Price,
		Tenant: Tenant{
			ID:   matchResponse.Tenant.ID,
			Name: matchResponse.Tenant.Name,
		},
		MatchType: matchType,
	}

	if matchResponse.MerchantAccessCode != nil {
		padelMatch.AccessCode = matchResponse.MerchantAccessCode.Code
	}
	log.Debug("Match", "match", padelMatch)
	return padelMatch, nil
}
