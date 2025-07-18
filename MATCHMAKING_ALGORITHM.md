# Padel Matchmaking Algorithm

## Overview

This document describes the hybrid matchmaking algorithm used to create fair and competitive padel matches. The algorithm combines multiple strategies to ensure balanced teams, fair rotation, and engaging gameplay.

## Core Principles

1. **Competitive Balance**: Matches should be as evenly matched as possible
2. **Fairness**: All players should have equal opportunities and responsibilities
3. **Variety**: Promote diverse partnerships and opponent matchups
4. **Skill Development**: Encourage improvement through appropriate challenges

## Data Sources

### Player Data Structure
```go
type PlayerInfo struct {
    ID                  string
    Name                string
    Level               float64    // Playtomic skill level (0.0-10.0)
    BallBringerCount    int       // Times player has brought balls
    SlackUserID         *string   // Slack integration data
    // ... mapping fields
}

type PlayerStats struct {
    PlayerID      string
    MatchesPlayed int
    MatchesWon    int
    MatchesLost   int
    SetsWon       int
    SetsLost      int
    GamesWon      int
    GamesLost     int
    WinPercentage float64
}
```

### Match History Data
- Complete match records with teams and results
- Weekly performance snapshots for trend analysis
- Partnership history and success rates
- Head-to-head performance records

## Algorithm Components

### 1. Dynamic ELO Rating System

#### Initial ELO Calculation
```
Initial ELO = (Playtomic Level * 200) + 1000
```

**Rationale**: 
- Maps Playtomic levels (0-10) to ELO range (1000-3000)
- Provides reasonable starting point for skill assessment
- Allows for significant adjustment based on actual performance

#### ELO Updates After Matches
```
New ELO = Current ELO + K * (Actual Score - Expected Score)
```

**K-Factor Determination**:
- New players (< 10 matches): K = 40
- Developing players (10-30 matches): K = 30  
- Established players (> 30 matches): K = 20

**Expected Score Calculation**:
```
Expected Score = 1 / (1 + 10^((Opponent ELO - Player ELO) / 400))
```

**Team ELO Calculation**:
```
Team ELO = (Player1 ELO + Player2 ELO) / 2
```

#### Partnership Adjustment
```
Partnership Bonus = Previous Partnership Success Rate * 50
Adjusted Team ELO = Team ELO + Partnership Bonus
```

#### ELO Decay (Optional)
For inactive players (no matches in 30+ days):
```
Decayed ELO = Current ELO * 0.95 per month of inactivity
```

### 2. Multi-Factor Skill Assessment

#### Composite Skill Score
```
Skill Score = (0.4 * Normalized ELO) + 
              (0.3 * Win Rate Score) + 
              (0.2 * Recent Form Score) + 
              (0.1 * Head-to-Head Score)
```

**Component Calculations**:

**Normalized ELO**:
```
Normalized ELO = (ELO - 1000) / 2000  // Scale to 0-1 range
```

**Win Rate Score**:
```
Win Rate Score = (Matches Won / Total Matches) * Recency Weight
Recency Weight = 0.7 + (0.3 * Recent Match Ratio)
Recent Match Ratio = Recent Matches / Total Matches
```

**Recent Form Score**:
```
Recent Form Score = (Recent Wins / Recent Matches) 
Recent Matches = Last 10 matches or all matches if < 10
```

**Head-to-Head Score**:
```
H2H Score = (Wins vs Opponent / Total vs Opponent) if >= 3 matches
          = 0.5 if < 3 matches (neutral)
```

### 3. Team Chemistry & Balance

#### Partnership Compatibility
```go
type Partnership struct {
    Player1ID     string
    Player2ID     string
    MatchesPlayed int
    MatchesWon    int
    SetsWon       int
    SetsLost      int
    WinRate       float64
    LastPlayed    time.Time
}
```

**Partnership Score Calculation**:
```
Base Partnership Score = Partnership Win Rate
Frequency Penalty = min(0.2, (Recent Partnerships / 10) * 0.1)
Time Bonus = max(0, (Days Since Last Partnership - 7) * 0.01)
Final Partnership Score = Base Score - Frequency Penalty + Time Bonus
```

#### Team Balance Metrics

**Skill Differential**:
```
Skill Differential = |Team1 Average Skill - Team2 Average Skill|
Balance Score = 1 - (Skill Differential / Max Possible Differential)
```

**Playstyle Compatibility** (Future Enhancement):
```go
type PlaystyleProfile struct {
    Aggressiveness float64 // 0-1 scale
    NetPlay        float64 // 0-1 scale  
    Consistency    float64 // 0-1 scale
    PowerPlay      float64 // 0-1 scale
}
```

### 4. Fairness & Rotation System

#### Booking Responsibility Rotation
```go
type BookingHistory struct {
    PlayerID       string
    TimesBooked    int
    LastBooked     time.Time
    SkillLevel     float64
    BookingScore   float64
}
```

**Booking Score Calculation**:
```
Base Booking Score = 1 / (Times Booked + 1)
Skill Adjustment = (Player Skill - Average Skill) * 0.1
Time Bonus = min(0.3, (Days Since Last Booking) * 0.02)
Final Booking Score = Base Score + Skill Adjustment + Time Bonus
```

#### Match Frequency Balancing
```
Frequency Score = 1 / (Recent Matches + 1)
Recent Matches = Matches played in last 14 days
```

#### Opponent Diversity
```go
type OpponentHistory struct {
    PlayerID      string
    OpponentID    string
    MatchesPlayed int
    LastPlayed    time.Time
}
```

**Diversity Score**:
```
Diversity Score = 1 - (Matches Against Opponent / Total Matches)
Time Bonus = max(0, (Days Since Last Match - 14) * 0.01)
Final Diversity Score = Base Score + Time Bonus
```

### 5. Match Generation Algorithm

#### Phase 1: Player Pool Analysis
```go
func analyzePlayerPool(availablePlayers []PlayerInfo) PoolAnalysis {
    analysis := PoolAnalysis{}
    
    for _, player := range availablePlayers {
        // Calculate current ELO
        elo := calculateCurrentELO(player.ID)
        
        // Assess recent form
        recentForm := calculateRecentForm(player.ID)
        
        // Get partnership preferences
        partnerships := getPartnershipHistory(player.ID)
        
        analysis.Players = append(analysis.Players, PlayerAnalysis{
            Player:       player,
            ELO:          elo,
            RecentForm:   recentForm,
            Partnerships: partnerships,
        })
    }
    
    return analysis
}
```

#### Phase 2: Team Generation
```go
func generateTeamCombinations(players []PlayerAnalysis) []TeamCombination {
    var combinations []TeamCombination
    
    // Generate all possible team pairings
    for i := 0; i < len(players); i++ {
        for j := i + 1; j < len(players); j++ {
            for k := 0; k < len(players); k++ {
                if k == i || k == j { continue }
                for l := k + 1; l < len(players); l++ {
                    if l == i || l == j { continue }
                    
                    team1 := Team{players[i], players[j]}
                    team2 := Team{players[k], players[l]}
                    
                    combination := TeamCombination{
                        Team1: team1,
                        Team2: team2,
                        Score: calculateTeamBalanceScore(team1, team2),
                    }
                    
                    combinations = append(combinations, combination)
                }
            }
        }
    }
    
    return combinations
}
```

#### Phase 3: Match Optimization
```go
func optimizeMatches(combinations []TeamCombination) []RankedMatch {
    var rankedMatches []RankedMatch
    
    for _, combo := range combinations {
        match := RankedMatch{
            Combination: combo,
            Scores: MatchScores{
                Balance:    calculateBalanceScore(combo),
                Variety:    calculateVarietyScore(combo),
                Fairness:   calculateFairnessScore(combo),
                Chemistry:  calculateChemistryScore(combo),
            },
        }
        
        // Calculate composite score
        match.CompositeScore = 
            (0.4 * match.Scores.Balance) +
            (0.25 * match.Scores.Variety) +
            (0.25 * match.Scores.Fairness) +
            (0.1 * match.Scores.Chemistry)
        
        rankedMatches = append(rankedMatches, match)
    }
    
    // Sort by composite score
    sort.Slice(rankedMatches, func(i, j int) bool {
        return rankedMatches[i].CompositeScore > rankedMatches[j].CompositeScore
    })
    
    return rankedMatches
}
```

### 6. Scoring Components Detail

#### Balance Score (Weight: 0.4)
```
ELO Difference = |Team1 ELO - Team2 ELO|
Max Possible Difference = 2000 // Theoretical max ELO range
Balance Score = 1 - (ELO Difference / Max Possible Difference)
```

#### Variety Score (Weight: 0.25)
```
Partnership Novelty = Average of both partnership novelty scores
Opponent Diversity = Average of all opponent diversity scores
Variety Score = (0.6 * Partnership Novelty) + (0.4 * Opponent Diversity)
```

#### Fairness Score (Weight: 0.25)
```
Booking Fairness = Selected booking player's booking score
Frequency Fairness = Average of all players' frequency scores
Fairness Score = (0.6 * Booking Fairness) + (0.4 * Frequency Fairness)
```

#### Chemistry Score (Weight: 0.1)
```
Team1 Chemistry = Partnership compatibility score for team1
Team2 Chemistry = Partnership compatibility score for team2
Chemistry Score = (Team1 Chemistry + Team2 Chemistry) / 2
```

### 7. Implementation Considerations

#### Performance Optimizations
- **Caching**: Cache ELO calculations and partnership data
- **Pruning**: Early elimination of clearly unbalanced combinations
- **Parallel Processing**: Calculate scores for combinations in parallel
- **Database Indexing**: Optimize queries for player stats and match history

#### Error Handling
- **Insufficient Players**: Fallback to simple skill-based matching
- **Missing Data**: Use default values and confidence scoring
- **Database Failures**: Graceful degradation to basic matching

#### Testing Strategy
- **Unit Tests**: Individual scoring components
- **Integration Tests**: Full algorithm with mock data
- **Performance Tests**: Large player pools and combination generation
- **Regression Tests**: Ensure algorithm improvements don't break existing functionality

### 8. Configuration Parameters

```go
type AlgorithmConfig struct {
    // ELO System
    InitialELOBase      float64 // 1000
    PlaytomicMultiplier float64 // 200
    KFactorNew          float64 // 40
    KFactorDeveloping   float64 // 30
    KFactorEstablished  float64 // 20
    
    // Skill Score Weights
    ELOWeight          float64 // 0.4
    WinRateWeight      float64 // 0.3
    RecentFormWeight   float64 // 0.2
    HeadToHeadWeight   float64 // 0.1
    
    // Match Score Weights
    BalanceWeight      float64 // 0.4
    VarietyWeight      float64 // 0.25
    FairnessWeight     float64 // 0.25
    ChemistryWeight    float64 // 0.1
    
    // Fairness Parameters
    RecentMatchWindow  int     // 14 days
    PartnershipWindow  int     // 30 days
    BookingRotationMin int     // 7 days
    
    // Performance Limits
    MaxCombinations    int     // 1000
    TimeoutSeconds     int     // 30
}
```

### 9. Monitoring & Analytics

#### Key Metrics
- **Match Balance**: Track actual vs predicted match outcomes
- **Player Satisfaction**: Survey feedback on match quality
- **Algorithm Performance**: Execution time and success rates
- **Fairness Metrics**: Distribution of booking responsibilities and match frequency

#### Logging
- **Decision Audit Trail**: Log why specific matches were selected
- **Performance Metrics**: Track algorithm execution times
- **Error Tracking**: Monitor and alert on algorithm failures

### 10. Future Enhancements

#### Advanced Features
- **Weather Considerations**: Factor in weather preferences for outdoor courts
- **Time Preferences**: Player availability and preferred playing times
- **Skill Improvement Tracking**: Dynamic adjustment based on performance trends
- **Social Preferences**: Factor in player friendship networks

#### Machine Learning Integration
- **Outcome Prediction**: Use historical data to improve match predictions
- **Player Clustering**: Identify playing styles and preferences
- **Dynamic Weight Adjustment**: Optimize algorithm weights based on outcomes

## Implementation Timeline

1. **Phase 1**: Basic ELO system and team generation
2. **Phase 2**: Partnership tracking and fairness rotation
3. **Phase 3**: Advanced scoring and optimization
4. **Phase 4**: Performance optimization and testing
5. **Phase 5**: Monitoring and analytics integration

## References

- [ELO Rating System](https://en.wikipedia.org/wiki/Elo_rating_system)
- [Padel Rules and Scoring](https://www.padel.sport/rules/)
- [Team Formation Algorithms](https://www.researchgate.net/publication/team-formation-algorithms)