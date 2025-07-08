package playtomic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/log"
)

// Client is a custom Playtomic API client.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new custom Playtomic client.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetSpecificMatch fetches a specific match by its ID.
func (c *Client) GetSpecificMatch(matchID string) (PadelMatch, error) {
	url := fmt.Sprintf("https://api.playtomic.io/v1/matches/%s", matchID)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return PadelMatch{}, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-AU,en;q=0.9")
	req.Header.Set("User-Agent", "PlaytomicGoClient/1.0")
	log.Debug("Requesting match with curl:", "cmd",
		fmt.Sprintf(
			`curl -X GET '%s' -H 'Accept: */*' -H 'Content-Type: application/json' -H 'Accept-Language: en-AU,en;q=0.9' -H 'User-Agent: PlaytomicGoClient/1.0'`,
			url,
		),
	)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PadelMatch{}, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return PadelMatch{}, fmt.Errorf("received non-OK HTTP status: %d. body: %s", resp.StatusCode, string(body))
	}

	var matchResponse playtomicMatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&matchResponse); err != nil {
		return PadelMatch{}, fmt.Errorf("failed to decode response: %w", err)
	}
	log.Debug("matchResponse:", "matchResponse", matchResponse)

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
				Level:  responsePlayer.LevelValue,
				Paid:   paymentStatus[responsePlayer.UserID],
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
	default:
		gameStatus = GameStatusUnknown
		log.Warn("Unknown game status received from Playtomic API", "status", matchResponse.GameStatus)
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
	default:
		log.Warn("Unknown results status received from Playtomic API", "status", matchResponse.ResultsStatus)
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
	}

	if matchResponse.MerchantAccessCode != nil {
		padelMatch.AccessCode = matchResponse.MerchantAccessCode.Code
	}

	return padelMatch, nil
}

// playtomicMatchResponse defines the structure for the JSON response from the Playtomic API for a single match.
type playtomicMatchResponse struct {
	OwnerID            string                       `json:"owner_id"`
	StartDate          string                       `json:"start_date"`
	EndDate            string                       `json:"end_date"`
	CreatedAt          string                       `json:"created_at"`
	Status             string                       `json:"status"`
	GameStatus         string                       `json:"game_status"`
	Teams              []playtomicTeamResponse      `json:"teams"`
	Results            []playtomicResult            `json:"results"`
	ResultsStatus      string                       `json:"results_status"`
	RegistrationInfo   playtomicRegistrationInfo    `json:"registration_info"`
	ResourceName       string                       `json:"resource_name"`
	MerchantAccessCode *playtomicMerchantAccessCode `json:"merchant_access_code"`
	Price              string                       `json:"price"`
	Tenant             playtomicTenant              `json:"tenant"`
}

// playtomicResult defines a set result.
type playtomicResult struct {
	Name   string               `json:"name"`
	Scores []playtomicTeamScore `json:"scores"`
}

// playtomicTeamScore defines the score for a team in a set.
type playtomicTeamScore struct {
	TeamID string `json:"team_id"`
	Score  int    `json:"score"`
}

// playtomicRegistrationInfo defines the registration details.
type playtomicRegistrationInfo struct {
	Registrations []playtomicRegistration `json:"registrations"`
}

// playtomicRegistration defines a single player registration.
type playtomicRegistration struct {
	UserID  string `json:"user_id"`
	Payable bool   `json:"payable"`
}

// playtomicTenant defines the structure for the tenant information in the response.
type playtomicTenant struct {
	ID   string `json:"tenant_id"`
	Name string `json:"tenant_name"`
}

// playtomicMerchantAccessCode defines the structure for the merchant access code.
type playtomicMerchantAccessCode struct {
	Code string `json:"code"`
}

// playtomicTeamResponse defines the structure for a team within the match response.
type playtomicTeamResponse struct {
	TeamID     string                    `json:"team_id"`
	Players    []playtomicPlayerResponse `json:"players"`
	TeamResult *string                   `json:"team_result"`
}

// playtomicPlayerResponse defines the structure for a player within a team.
type playtomicPlayerResponse struct {
	UserID     string   `json:"user_id"`
	Name       string   `json:"name"`
	LevelValue *float64 `json:"level_value"`
}
