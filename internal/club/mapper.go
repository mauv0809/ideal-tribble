package club

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/log"
)

// PlayerSuggestion represents a player mapping suggestion with confidence score
type PlayerSuggestion struct {
	Player     PlayerInfo
	Confidence float64
	Reasons    []string
}

// PlayerMapper handles Slack-to-Playtomic player mapping
type PlayerMapper struct {
	store ClubStore
}

// NewPlayerMapper creates a new player mapper
func NewPlayerMapper(store ClubStore) *PlayerMapper {
	return &PlayerMapper{store: store}
}

// FindOrMapPlayer attempts to find an existing mapping or suggests new mappings
func (pm *PlayerMapper) FindOrMapPlayer(slackUserID, slackUsername, slackDisplayName string) (*PlayerInfo, []PlayerSuggestion, error) {
	// 1. Check if mapping already exists
	if existingPlayer, err := pm.store.GetPlayerBySlackUserID(slackUserID); err != nil {
		return nil, nil, err
	} else if existingPlayer != nil {
		log.Info("Found existing player mapping", "slack_user_id", slackUserID, "player", existingPlayer.Name)
		return existingPlayer, nil, nil
	}

	// 2. Try to find unmapped players with similar names
	unmappedPlayers, err := pm.store.GetUnmappedPlayers()
	if err != nil {
		return nil, nil, err
	}

	suggestions := pm.findSimilarPlayers(slackUsername, slackDisplayName, unmappedPlayers)
	
	// 3. If high confidence match (>0.8), auto-accept
	if len(suggestions) > 0 && suggestions[0].Confidence > 0.8 {
		err := pm.store.UpdatePlayerSlackMapping(
			suggestions[0].Player.ID,
			slackUserID,
			slackUsername,
			slackDisplayName,
			"AUTO_MATCHED",
			suggestions[0].Confidence,
		)
		if err != nil {
			return nil, nil, err
		}
		
		log.Info("Auto-mapped player with high confidence", 
			"slack_user_id", slackUserID, 
			"player", suggestions[0].Player.Name, 
			"confidence", suggestions[0].Confidence)
		
		return &suggestions[0].Player, nil, nil
	}

	// 4. Return suggestions for manual confirmation
	return nil, suggestions, nil
}

// findSimilarPlayers finds players with similar names and ranks them by confidence
func (pm *PlayerMapper) findSimilarPlayers(slackUsername, slackDisplayName string, players []PlayerInfo) []PlayerSuggestion {
	var suggestions []PlayerSuggestion

	for _, player := range players {
		// Skip players that already have Slack mappings
		if player.SlackUserID != nil && *player.SlackUserID != "" {
			continue
		}

		score := pm.calculateSimilarity(slackUsername, slackDisplayName, player.Name)
		if score > 0.3 { // Minimum threshold
			reasons := pm.getMatchReasons(slackUsername, slackDisplayName, player.Name)
			suggestions = append(suggestions, PlayerSuggestion{
				Player:     player,
				Confidence: score,
				Reasons:    reasons,
			})
		}
	}

	// Sort by confidence descending
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Confidence > suggestions[j].Confidence
	})

	// Return top 5 suggestions
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}

	return suggestions
}

// calculateSimilarity calculates similarity between Slack names and player name
func (pm *PlayerMapper) calculateSimilarity(slackUsername, slackDisplayName, playerName string) float64 {
	var scores []float64

	// Normalize names for comparison
	normalizedSlackUsername := pm.normalizeName(slackUsername)
	normalizedSlackDisplayName := pm.normalizeName(slackDisplayName)
	normalizedPlayerName := pm.normalizeName(playerName)

	// Username similarity
	if normalizedSlackUsername != "" {
		scores = append(scores, pm.stringSimilarity(normalizedSlackUsername, normalizedPlayerName))
	}

	// Display name similarity
	if normalizedSlackDisplayName != "" {
		scores = append(scores, pm.stringSimilarity(normalizedSlackDisplayName, normalizedPlayerName))
	}

	// Token-based matching (first/last name components)
	if normalizedSlackDisplayName != "" {
		scores = append(scores, pm.tokenSimilarity(normalizedSlackDisplayName, normalizedPlayerName))
	}

	if len(scores) == 0 {
		return 0.0
	}

	// Return weighted average
	return pm.weightedAverage(scores)
}

// normalizeName normalizes a name for comparison
func (pm *PlayerMapper) normalizeName(name string) string {
	// Remove extra spaces, convert to lowercase
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	
	// Remove special characters except spaces
	var result strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsSpace(r) {
			result.WriteRune(r)
		}
	}
	
	// Normalize multiple spaces to single space
	normalized := strings.Join(strings.Fields(result.String()), " ")
	return normalized
}

// stringSimilarity calculates similarity between two strings using simple edit distance
func (pm *PlayerMapper) stringSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	
	if s1 == "" || s2 == "" {
		return 0.0
	}

	// Simple Levenshtein distance implementation
	distance := pm.levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	
	if maxLen == 0 {
		return 1.0
	}
	
	return 1.0 - float64(distance)/float64(maxLen)
}

// tokenSimilarity calculates similarity based on individual words/tokens
func (pm *PlayerMapper) tokenSimilarity(s1, s2 string) float64 {
	tokens1 := strings.Fields(s1)
	tokens2 := strings.Fields(s2)
	
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0
	}
	
	var matchCount int
	for _, token1 := range tokens1 {
		for _, token2 := range tokens2 {
			if pm.stringSimilarity(token1, token2) > 0.8 {
				matchCount++
				break
			}
		}
	}
	
	maxTokens := len(tokens1)
	if len(tokens2) > maxTokens {
		maxTokens = len(tokens2)
	}
	
	return float64(matchCount) / float64(maxTokens)
}

// weightedAverage calculates weighted average of similarity scores
func (pm *PlayerMapper) weightedAverage(scores []float64) float64 {
	if len(scores) == 0 {
		return 0.0
	}
	
	total := 0.0
	for _, score := range scores {
		total += score
	}
	
	return total / float64(len(scores))
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (pm *PlayerMapper) levenshteinDistance(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	
	if len(s1) == 0 {
		return len(s2)
	}
	
	if len(s2) == 0 {
		return len(s1)
	}
	
	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}
	
	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	
	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len(s1)][len(s2)]
}

// getMatchReasons provides human-readable reasons for the match
func (pm *PlayerMapper) getMatchReasons(slackUsername, slackDisplayName, playerName string) []string {
	var reasons []string
	
	normalizedSlackUsername := pm.normalizeName(slackUsername)
	normalizedSlackDisplayName := pm.normalizeName(slackDisplayName)
	normalizedPlayerName := pm.normalizeName(playerName)
	
	if normalizedSlackUsername == normalizedPlayerName {
		reasons = append(reasons, "Exact username match")
	} else if pm.stringSimilarity(normalizedSlackUsername, normalizedPlayerName) > 0.8 {
		reasons = append(reasons, "Very similar username")
	}
	
	if normalizedSlackDisplayName == normalizedPlayerName {
		reasons = append(reasons, "Exact display name match")
	} else if pm.stringSimilarity(normalizedSlackDisplayName, normalizedPlayerName) > 0.8 {
		reasons = append(reasons, "Very similar display name")
	}
	
	if pm.tokenSimilarity(normalizedSlackDisplayName, normalizedPlayerName) > 0.5 {
		reasons = append(reasons, "Matching name components")
	}
	
	if len(reasons) == 0 {
		reasons = append(reasons, "Partial name similarity")
	}
	
	return reasons
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}